// Room invite codes — the same string is shown as QR code and copied with "Copy Code".
//
// Format (kept short so the QR code stays coarse and easy to scan):
//   CB1-<base64url>    plain:  bytes = room id (16 byte UUID) | room key (32 byte) | room name (UTF-8)
//   CB1P-<base64url>   PIN:    the plain code encrypted with AesAdapter.encryptByPIN
//
// The code is plain text without a URL scheme, so the native iOS/Android camera only offers
// to copy it; it can then be pasted into the join dialog.
// Older invite codes (base64 JSON {v, enc, data} and the raw JSON {i, n, k} of old QR codes)
// are still accepted by decode().

const PREFIX_PLAIN = "CB1-";
const PREFIX_PIN = "CB1P-";

const UUID_BYTES = 16;
const KEY_BYTES = 32;

export class InviteCode {
    constructor(aes) {
        this._aes = aes;
    }

    // invite: {i: roomId, n: roomName, k: base64 room key}. Returns the invite code string.
    async encode(invite, pin) {
        const name = new TextEncoder().encode(invite.n || "");
        const bytes = new Uint8Array(UUID_BYTES + KEY_BYTES + name.length);
        bytes.set(uuidToBytes(invite.i), 0);
        bytes.set(base64ToBytes(invite.k), UUID_BYTES);
        bytes.set(name, UUID_BYTES + KEY_BYTES);

        const plain = PREFIX_PLAIN + bytesToBase64Url(bytes);
        if (!pin) return plain;

        const encrypted = await this._aes.encryptByPIN(plain, pin);
        return PREFIX_PIN + base64ToBase64Url(encrypted);
    }

    // True if decode() needs a PIN for this code.
    needsPin(code) {
        code = code.trim();
        if (code.startsWith(PREFIX_PIN)) return true;
        const legacy = parseLegacy(code);
        return !!(legacy && legacy.enc);
    }

    // Returns {i, n, k}. Throws InviteError("pin") if a PIN is required or wrong,
    // InviteError("invalid") if the code cannot be read.
    async decode(code, pin) {
        code = code.trim();

        if (code.startsWith(PREFIX_PIN)) {
            if (!pin) throw new InviteError("pin");
            let plain;
            try {
                plain = await this._aes.decryptByPIN(base64UrlToBase64(code.slice(PREFIX_PIN.length)), pin);
            } catch {
                throw new InviteError("pin");
            }
            return this.decode(plain);
        }

        if (code.startsWith(PREFIX_PLAIN)) {
            let bytes;
            try {
                bytes = base64UrlToBytes(code.slice(PREFIX_PLAIN.length));
            } catch {
                throw new InviteError("invalid");
            }
            if (bytes.length < UUID_BYTES + KEY_BYTES) throw new InviteError("invalid");
            return {
                i: bytesToUuid(bytes.subarray(0, UUID_BYTES)),
                k: bytesToBase64(bytes.subarray(UUID_BYTES, UUID_BYTES + KEY_BYTES)),
                n: new TextDecoder().decode(bytes.subarray(UUID_BYTES + KEY_BYTES)),
            };
        }

        // Old QR codes: raw JSON {i, n, k}
        if (code.startsWith("{")) {
            try {
                return checkInvite(JSON.parse(code));
            } catch {
                throw new InviteError("invalid");
            }
        }

        // Old copied codes: base64 of {v: 1, enc, data}
        const legacy = parseLegacy(code);
        if (!legacy) throw new InviteError("invalid");

        let data = legacy.data;
        if (legacy.enc) {
            if (!pin) throw new InviteError("pin");
            try {
                data = await this._aes.decryptByPIN(legacy.data, pin);
            } catch {
                throw new InviteError("pin");
            }
        }
        try {
            return checkInvite(JSON.parse(data));
        } catch {
            throw new InviteError("invalid");
        }
    }
}

// reason: "pin" (PIN missing or wrong) or "invalid"
export class InviteError extends Error {
    constructor(reason) {
        super(reason === "pin" ? "invite code needs a valid PIN" : "invalid invite code");
        this.reason = reason;
    }
}

function parseLegacy(code) {
    try {
        const bytes = Uint8Array.from(atob(code), c => c.charCodeAt(0));
        const obj = JSON.parse(new TextDecoder().decode(bytes));
        return obj && obj.v === 1 ? obj : null;
    } catch {
        return null;
    }
}

function checkInvite(obj) {
    if (!obj || typeof obj.i !== "string" || typeof obj.k !== "string") throw new Error("invalid invite");
    return { i: obj.i, n: obj.n || "", k: obj.k };
}

// -------------------------------------------------------------------------
// Encoding helpers
// -------------------------------------------------------------------------

function uuidToBytes(uuid) {
    const hex = uuid.replace(/-/g, "");
    if (!/^[0-9a-f]{32}$/i.test(hex)) throw new Error("room id is not a UUID: " + uuid);
    return Uint8Array.from(hex.match(/../g), h => parseInt(h, 16));
}

function bytesToUuid(bytes) {
    const hex = Array.from(bytes, b => b.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function bytesToBase64(bytes) {
    return btoa(Array.from(bytes, b => String.fromCharCode(b)).join(""));
}

function base64ToBytes(b64) {
    return Uint8Array.from(atob(b64), c => c.charCodeAt(0));
}

function base64ToBase64Url(b64) {
    return b64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64UrlToBase64(b64url) {
    const b64 = b64url.replace(/-/g, "+").replace(/_/g, "/");
    return b64 + "=".repeat((4 - b64.length % 4) % 4);
}

function bytesToBase64Url(bytes) {
    return base64ToBase64Url(bytesToBase64(bytes));
}

function base64UrlToBytes(b64url) {
    return base64ToBytes(base64UrlToBase64(b64url));
}
