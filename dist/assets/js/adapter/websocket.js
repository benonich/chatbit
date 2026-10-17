import {Events} from "./event.js";
import {Message} from "../domain/message.js";
import {File} from "../domain/file.js";

const CHUNK_SIZE = 64 * 1024; // 64KB

// Pure transport layer — no decryption, no DB, no business logic.
// Emits raw events; a higher layer (App / chat-handler) processes them.
//
// Events emitted:
//   ws:connected                — socket opened (subscribe to send join_rooms)
//   ws:message       (item)     — raw encrypted message payload from server
//   ws:file_done     ({id, roomId, mimeType, timestamp, buffer}) — assembled binary transfer
export class WebSocketAdapter {
    constructor(events, db) {
        this.conn = null;
        this._events = events;
        this._db = db;
        this._pendingAck = null;
        this._bufferedAcks = new Map();
        this._incomingTransfers = new Map();
    }

    _getEndpoint() {
        const protocol = location.protocol === "https:" ? "wss://" : "ws://";
        return `${protocol}${document.location.host}/ws`;
    }

    handler = async (profile) => {
        console.log("WS Connection starting...");

        await this.initWS(profile);
        this._events.off(Events.STATE.UNLOCK, this.handler);


        const rooms = await this._db.getRooms();
        const allRooms = rooms.map(r => ({id: r.id, notification: r.push_notifications}));
        const lastTimeStamp = await this._db.getLatestTimestamp();

        this.joinRooms({
            rooms: allRooms,
            lastConnectionTime: lastTimeStamp,
        });

        this._events.emit(Events.WS.READY);
    };

    init() {
        this._events.on(Events.STATE.UNLOCK, this.handler);

        this._events.on(Events.WS.SEND_FILE, async (msg) => {
            this.transferStart(msg)
            await this.sendFile(msg);
        });

        this._events.on(Events.WS.SEND_MESSAGE, async (msg) => {
            this.sendMessage(msg);
        });

        this._events.on(Events.WS.JOIN_ROOM, async (roomId) => {
            this.joinRooms({
                rooms: [roomId],
                last_connection_time: 0,
            });
        });

        this._events.on(Events.WS.LEAVE_ROOM, async (roomId) => {
            this.leaveRooms({rooms: [roomId]});
        });

        this._events.on(Events.WS.TRANSFER_REQUEST, async (msg) => {
            this.transferRequest(msg);
        });


        this._events.on(Events.WS.SUBSCRIBE_PUSH, async (subPush) => {
            this.subscribePush(subPush);
        });
    }

    initWS(profile) {
        if (this.conn !== null && this.conn.readyState === WebSocket.OPEN) return;

        return new Promise(async (resolve, reject) => {
            if (!window["WebSocket"]) {
                reject(new Error("WebSockets not supported"));
                return;
            }

            this.conn = new WebSocket(this._getEndpoint() + "?client_uid=" + profile.clientId);
            this.conn.binaryType = "arraybuffer";

            this.conn.onclose = async (evt) => {
                console.log("WS Connection closed", evt);
                await new Promise(r => setTimeout(r, 1000));
                console.log("Reconnecting WS");
                await this.initWS();
            };

            this.conn.onmessage = (e) => {
                if (e.data instanceof ArrayBuffer) {
                    this._handleIncomingChunk(e.data);
                    return;
                }
                for (const text of e.data.split('\n')) {
                    if (!text.trim()) continue;
                    try {
                        this._handleIncomingMessage(JSON.parse(text));
                    } catch (err) {
                        console.error("Failed to parse WS message:", text, err);
                    }
                }
            };

            this.conn.onopen = () => {
                console.log("WS Connection connected");
                this._events.emit('ws:connected');
                resolve();
            };

            this.conn.onerror = (err) => {
                console.log("WS Connection error:", err.message);
                reject(err);
            };
        });
    }

    // -------------------------------------------------------------------------
    // Outgoing messages
    // -------------------------------------------------------------------------

    send(type, msg) {
        this.conn.send(JSON.stringify({type, data: msg}));
    }

    sendMessage(msg) {
        this.conn.send(JSON.stringify({type: "message", data: msg}));
    }

    joinRooms(msg) {
        this.conn.send(JSON.stringify({type: "join_rooms", data: msg}));
    }

    leaveRooms(msg) {
        this.conn.send(JSON.stringify({type: "leave_rooms", data: msg}));
    }

    subscribePush(msg) {
        this.conn.send(JSON.stringify({type: "sub_push", data: msg}));
    }

    // request image or data from server
    transferRequest(msg) {
        this.conn.send(JSON.stringify({type: "transfer_request", data: msg}));
    }

    // start to send image or data to server
    transferStart(msg) {
        this.conn.send(JSON.stringify({type: "transfer_start", data: msg}));
    }

    // -------------------------------------------------------------------------
    // Incoming message dispatch
    // -------------------------------------------------------------------------

    _handleIncomingMessage(msg) {
        switch (msg.type) {
            case "message":
                msg.data.synced = true;

                this._events.emit(Events.WS.RECEIVED_MESSAGE, Message.fromJSON(msg.data));
                break;
            case "transfer_ack":
                this.resolveAck(msg.data.id, msg.data.chunk);
                break;
            case "transfer_nack":
                // ToDo
                console.log("received nack");
                break;
            case "transfer_start":
                this._handleTransferStart(
                    msg.data.id, msg.data.roomId, msg.data.mimeType,
                    msg.data.timestamp, msg.data.chunksTotal, msg.data.chunkSize
                );
                break;
            case "transfer_done":
                this._assembleTransfer(msg.data.id);
                break;
            default:
                console.warn("Unknown WS message type:", msg.type);
        }
    }

    // -------------------------------------------------------------------------
    // Incoming binary transfer
    // -------------------------------------------------------------------------

    _handleTransferStart(id, roomId, mimeType, timestamp, chunksTotal, chunkSize) {
        console.log("transfer start", id, chunksTotal, "chunks");
        this._incomingTransfers.set(id, {
            roomId, mimeType, timestamp,
            chunks: new Array(chunksTotal).fill(null),
            chunksSize: chunkSize,
            received: 0,
            chunksTotal,
        });
    }

    _handleIncomingChunk(data) {
        const view = new DataView(data);
        const uuidBytes = new Uint8Array(data, 0, 16);
        const id = [
            uuidBytes.slice(0, 4), uuidBytes.slice(4, 6),
            uuidBytes.slice(6, 8), uuidBytes.slice(8, 10),
            uuidBytes.slice(10, 16),
        ].map(b => Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')).join('-');

        const chunkId = view.getUint32(16, false);
        const encrypted = data.slice(20);

        const t = this._incomingTransfers.get(id);
        if (!t) return;
        t.chunks[chunkId] = encrypted;
        t.received++;
    }

    // Assembles chunks into a single buffer and emits ws:file_done.
    // No DB access — the consumer saves the file.
    _assembleTransfer(fileId) {
        const t = this._incomingTransfers.get(fileId);
        if (!t) {
            console.error("unknown transfer:", fileId);
            return;
        }

        const totalLength = t.chunks.reduce((sum, c) => sum + c.byteLength, 0);
        const combined = new Uint8Array(totalLength);
        let offset = 0;
        for (const chunk of t.chunks) {
            combined.set(new Uint8Array(chunk), offset);
            offset += chunk.byteLength;
        }
        this._incomingTransfers.delete(fileId);

        const fileM = new File({
            id: fileId,
            roomId: t.roomId,
            mimeType: t.mimeType,
            timestamp: t.timestamp,
            blob: combined.buffer,
            chunkSize: t.chunkSize,
            ttl: t.ttl,
            store: t.store,
            totalSize: t.totalSize,
        });

        this._events.emit(Events.WS.RECEIVED_FILE, fileM);
    }

    // -------------------------------------------------------------------------
    // Outgoing binary transfer
    // -------------------------------------------------------------------------

    async sendFile(msg) {
        for (let i = 0; i < msg.chunksTotal; i++) {
            await this.waitForAck(msg.id, i);
            const chunk = msg.blob.slice(i * CHUNK_SIZE, (i + 1) * CHUNK_SIZE);
            const header = this._encodeChunkHeader(msg.id, i);
            this.conn.send(this._concat(header, chunk));
        }
        this.conn.send(JSON.stringify({type: 'transfer_done', data: {id: msg.id}}));
    }

    _encodeChunkHeader(id, index) {
        const buf = new ArrayBuffer(20);
        const view = new DataView(buf);
        const uuidBytes = id.replace(/-/g, '').match(/.{2}/g).map(h => parseInt(h, 16));
        uuidBytes.forEach((b, i) => view.setUint8(i, b));
        view.setUint32(16, index, false); // big-endian
        return buf;
    }

    _concat(a, b) {
        const tmp = new Uint8Array(a.byteLength + b.byteLength);
        tmp.set(new Uint8Array(a), 0);
        tmp.set(new Uint8Array(b), a.byteLength);
        return tmp.buffer;
    }

    // Keep old public names for callers that use them directly
    encodeChunkHeader(id, index) {
        return this._encodeChunkHeader(id, index);
    }

    concat(a, b) {
        return this._concat(a, b);
    }

    // -------------------------------------------------------------------------
    // Flow control — ACK/NACK for outgoing chunks
    // -------------------------------------------------------------------------

    resolveAck(id, chunkIndex) {
        if (this._pendingAck &&
            id === this._pendingAck.id &&
            chunkIndex === this._pendingAck.chunkIndex) {
            this._pendingAck.resolve();
            this._pendingAck = null;
        } else {
            this._bufferedAcks.set(`${id}:${chunkIndex}`, true);
        }
    }

    waitForAck(id, chunkIndex) {
        const key = `${id}:${chunkIndex}`;
        if (this._bufferedAcks.has(key)) {
            this._bufferedAcks.delete(key);
            return Promise.resolve();
        }
        return new Promise(resolve => {
            this._pendingAck = {id, chunkIndex, resolve};
        });
    }
}

// -------------------------------------------------------------------------
// Global helpers — thin wrappers so existing call sites keep working
// -------------------------------------------------------------------------

function joinRooms(room_id, notification) {
    ws.joinRooms({rooms: [{id: room_id, notification: notification}]});
}

function leaveRoom(room_id) {
    ws.leaveRooms({rooms: [room_id]});
}
