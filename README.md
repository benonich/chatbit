# ChatBit

End-to-end encrypted group chat as a Progressive Web App.

[![CI](https://github.com/benonich/chatbit/actions/workflows/ci.yml/badge.svg)](https://github.com/benonich/chatbit/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](go.mod)
[![PWA](https://img.shields.io/badge/PWA-installable-5A0FC8)](dist/manifest.json)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

ChatBit encrypts every message and image in the browser with AES-256-GCM before it leaves the
device. The Go server only relays and temporarily stores ciphertext — it never sees room keys,
message content, names or images. There are no accounts: a room is joined with an invite code
or QR code that carries the room key.

## Table of Contents

- [Features](#features)
- [How It Works](#how-it-works)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Deployment](#deployment)
- [Project Structure](#project-structure)
- [WebSocket Protocol](#websocket-protocol)
- [Security Notes](#security-notes)
- [Roadmap](#roadmap)
- [License](#license)

## Features

- **End-to-end encryption** — AES-256-GCM per room, keys never leave the client unencrypted
- **No accounts** — local profile protected by a password (PBKDF2, 600 000 iterations)
- **Invite by QR code or code** — compact invite codes (`CB1-…`), optionally protected with a PIN;
  the QR code can also be scanned with the native camera app and pasted
- **Fast QR scanner** — native `BarcodeDetector` where available, zxing-cpp (WebAssembly) on iOS and Firefox
- **Images** — encrypted, chunked upload over the WebSocket with retry
- **Expiring messages** — per-room TTL; the server keeps messages for at most 30 days and files for 7 days
- **Offline-capable PWA** — installable, cached app shell, automatic update notice
- **Web Push** — notifications for new messages (VAPID)
- **Reliable connection** — automatic reconnect with exponential backoff, outbox for messages sent while offline

## How It Works

```mermaid
flowchart LR
    subgraph Client["Browser (PWA)"]
        UI[Views] --> Bus((EventBus))
        Bus --> AES[AES-GCM / PBKDF2<br/>Web Crypto]
        Bus --> DB[(IndexedDB<br/>Dexie)]
        Bus --> WS[WebSocket adapter]
        SW[Service Worker<br/>cache + push]
    end

    subgraph Server["ChatBit server (Go)"]
        Static[Static files<br/>/]
        Hub[WebSocket hub<br/>/ws]
        Store[(Badger KV<br/>TTL)]
        Files[(Encrypted files)]
        Push[Web Push<br/>VAPID]
    end

    WS <-->|ciphertext| Hub
    Hub --> Store
    Hub --> Files
    Hub --> Push -.->|notification| SW
    Client -->|HTTPS| Static
```

1. **Profile** — on first start the user picks an alias and a password. A master key is derived
   with PBKDF2 and only kept in memory; room keys are stored in IndexedDB encrypted with it.
2. **Room** — creating a room generates a random AES-256 key in the browser.
3. **Invite** — the room id, key and name are packed into an invite code / QR code
   (optionally encrypted with a PIN) and shared out of band.
4. **Messages** — text and images are encrypted with the room key and sent over the WebSocket.
   The server forwards them to everyone in the room, stores them with a TTL and sends a push
   notification to subscribed members.
5. **Reconnect** — after a reconnect the client rejoins its rooms and receives the messages it missed.

```mermaid
sequenceDiagram
    participant A as Alice (PWA)
    participant S as ChatBit server
    participant B as Bob (PWA)

    A->>A: create room, generate AES key
    A-->>B: invite code / QR code (out of band)
    B->>S: join_rooms [room id, last seen]
    S-->>B: missed messages (ciphertext)
    A->>A: encrypt message with room key
    A->>S: message (ciphertext, ttl)
    S->>S: store with TTL
    S->>B: message (ciphertext)
    S-)B: web push (if subscribed)
    B->>B: decrypt with room key
```

## Getting Started

### Requirements

- [Go](https://go.dev/dl/) 1.27+
- [Task](https://taskfile.dev/) (optional, for builds with version info)
- Docker (optional)

### Run locally

```bash
git clone https://github.com/benonich/chatbit.git
cd chatbit
go run .
```

Open <http://localhost:8081>. On `localhost` the service worker does not cache anything, so
changes in `dist/` are visible after a reload.

> Camera access (QR scanner), service workers and push notifications require a secure context.
> They work on `localhost`; on any other host ChatBit has to be served over **HTTPS**.

### Build

```bash
task build:app        # binary in artifacts/chatbit with version, commit and build date
```

### Tests

```bash
go test -race ./...
golangci-lint run      # config in .golangci.yml
```

## Configuration

| Variable            | Description                                                           | Default                  |
|---------------------|-----------------------------------------------------------------------|--------------------------|
| `VAPID_PUBLIC_KEY`  | Web Push public key. Must be set together with `VAPID_PRIVATE_KEY`.   | read from `vapid.json`   |
| `VAPID_PRIVATE_KEY` | Web Push private key. Must be set together with `VAPID_PUBLIC_KEY`.   | read from `vapid.json`   |
| `IMAGE_TMP_DIR`     | Directory for uploaded (encrypted) files.                             | `./files`                |

VAPID keys are resolved in this order: environment variables → `vapid.json` in the working
directory → a newly generated pair, which is then saved to `vapid.json`. Keep the keys stable:
changing them invalidates all existing push subscriptions.

Generate a key pair once, e.g. with:

```bash
npx web-push generate-vapid-keys
```

Data written to the working directory:

| Path             | Content                                 |
|------------------|-----------------------------------------|
| `chatbit_db.kv/` | Badger database (messages, rooms, push subscriptions) |
| `vapid.json`     | Generated VAPID keys (if not set via env) |
| `files/`         | Uploaded encrypted files, one folder per room (if `IMAGE_TMP_DIR` is not set) |

The server listens on port **8081** (HTTP and WebSocket on `/ws`).

## Deployment

### Docker

```bash
docker buildx build \
  --build-arg BUILD_BASE_IMAGE=golang:1.27-alpine \
  --build-arg TARGET_BASE_IMAGE=alpine:3.24 \
  -t chatbit .

docker run -d -p 8081:8081 \
  -e VAPID_PUBLIC_KEY=... \
  -e VAPID_PRIVATE_KEY=... \
  -v chatbit-db:/app/chatbit_db.kv \
  -v chatbit-files:/app/files \
  chatbit
```

Mount only the data paths — a volume on `/app` would hide the binary and `dist/` of newer images.

### Reverse proxy

Run ChatBit behind a reverse proxy that terminates TLS and forwards WebSocket upgrades on `/ws`.
Example for nginx:

```nginx
location / {
    proxy_pass http://127.0.0.1:8081;
}

location /ws {
    proxy_pass http://127.0.0.1:8081;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 1h;
}
```

### Updates

The service worker cache is named after the build commit. After deploying a new build, clients
install the new version in the background and show an update notice.

## Project Structure

```
.
├── main.go                     # entry point: message TTL, VAPID keys, server start
├── internal/
│   ├── core/                   # application wiring (DB, pub/sub)
│   ├── domain/                 # Message, Room, Peer, File, WebSocket message types
│   ├── port/                   # HTTP file server, WebSocket hub, file transfer
│   ├── adapter/                # badger database, pub/sub, VAPID / web push
│   └── controller/store/       # generic KV store with TTL
├── dist/                       # the PWA (served as static files)
│   ├── index.html
│   ├── service-worker.js
│   ├── manifest.json
│   └── assets/
│       ├── js/
│       │   ├── app.js          # composition root
│       │   ├── adapter/        # aes, database, websocket, views, scanner, invite, push, …
│       │   ├── domain/         # Message, Room, File, Profile
│       │   └── lib/            # Bootstrap, Dexie, qrcode.js, zxing-wasm
│       └── view/               # page templates (header/content)
├── api/openapi.yaml
├── Dockerfile
└── Taskfile.yml
```

## WebSocket Protocol

All frames on `/ws` are JSON with a `type` field; file chunks are sent as binary frames.

| Type               | Direction        | Purpose                                         |
|--------------------|------------------|-------------------------------------------------|
| `join_rooms`       | client → server  | subscribe to rooms, request missed messages     |
| `leave_rooms`      | client → server  | unsubscribe from rooms                          |
| `create_room`      | client → server  | register a new room                             |
| `message`          | both             | encrypted message                               |
| `sub_push`         | client → server  | store a web push subscription                   |
| `transfer_start`   | client → server  | start a chunked file upload                     |
| `transfer_ack` / `transfer_nack` | server → client | chunk / upload accepted or rejected   |
| `transfer_done`    | both             | upload finished                                 |
| `transfer_request` | client → server  | download an encrypted file                      |

## Security Notes

- The server only handles ciphertext, but it does see metadata: room ids, timestamps,
  message sizes and which client is in which room.
- **Whoever has an invite code has the room key.** Share codes over a trusted channel and use a
  PIN when the channel is not trusted. Invite codes do not expire, and a member who has joined
  keeps the key — there is no key rotation yet.
- The password protects the local profile. It cannot be recovered; forgetting it means losing
  access to the stored rooms on that device.

## Roadmap

- [ ] Online / offline status of users
- [ ] Expiring and one-time invite codes (server-side invite token with TTL)
- [ ] Room key rotation
- [ ] Server-side message timestamps
- [ ] Configurable listen port

## License

[MIT](LICENSE) © benonich
