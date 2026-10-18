// adapter/aes.js
// Responsible for: all encryption/decryption logic + in-memory key cache

export class AesAdapter {

    // -------------------------------------------------------------------------
    // Key Generation & Derivation
    // -------------------------------------------------------------------------

    // Generate a new key — extractable so it can be exported to base64 for DB storage
    async generateKey() {
        return await crypto.subtle.generateKey(
            {name: 'AES-GCM', length: 256},
            true,
            ['encrypt', 'decrypt']
        );
    }

    // Derive DB key from password + salt — non-extractable, stays in crypto engine
    async deriveKey(password, salt) {
        const keyMaterial = await crypto.subtle.importKey(
            'raw',
            new TextEncoder().encode(password),
            'PBKDF2',
            false,
            ['deriveKey']
        );
        return crypto.subtle.deriveKey(
            {name: 'PBKDF2', salt, iterations: 600000, hash: 'SHA-256'},
            keyMaterial,
            {name: 'AES-GCM', length: 256},
            false,  // non-extractable — key can never be read as raw bytes
            ['encrypt', 'decrypt']
        );
    }

    // -------------------------------------------------------------------------
    // Encrypt / Decrypt
    // -------------------------------------------------------------------------

    async encrypt(data, key) {
        const iv = crypto.getRandomValues(new Uint8Array(12));
        const encrypted = await crypto.subtle.encrypt(
            {name: 'AES-GCM', iv},
            key,
            new TextEncoder().encode(data)
        );
        const buf = new Uint8Array(iv.byteLength + encrypted.byteLength);
        buf.set(iv, 0);
        buf.set(new Uint8Array(encrypted), iv.byteLength);
        return this._bufToBase64(buf);
    }

    async decrypt(encryptedData, key) {
        const buf = this._base64ToBuf(encryptedData);
        const iv = buf.slice(0, 12);
        const data = buf.slice(12);
        const decrypted = await crypto.subtle.decrypt(
            {name: 'AES-GCM', iv},
            key,
            data
        );
        return new TextDecoder().decode(decrypted);
    }

    async encryptFile(fileOrBlob, key) {
        const iv = crypto.getRandomValues(new Uint8Array(12));
        const buf = await fileOrBlob.arrayBuffer();

        const encrypted = await crypto.subtle.encrypt(
            { name: "AES-GCM", iv },
            await key,
            buf
        );

        const result = new Uint8Array(12 + encrypted.byteLength);
        result.set(iv, 0);
        result.set(new Uint8Array(encrypted), 12);
        return result.buffer; // ArrayBuffer — no Base64 overhead
    }

    async decryptFile(encryptedBuffer, mimeType, key) {
        const data = new Uint8Array(encryptedBuffer);
        const iv = data.slice(0, 12);
        const payload = data.slice(12);

        const decrypted = await crypto.subtle.decrypt(
            { name: "AES-GCM", iv },
            await key,
            payload
        );

        return URL.createObjectURL(new Blob([decrypted], { type: mimeType }));
    }

    // -------------------------------------------------------------------------
    // Encrypt / Decrypt by PIN
    // -------------------------------------------------------------------------

    async encryptByPIN(data, pin) {
        const salt = await crypto.getRandomValues(new Uint8Array(16));
        const key = await this.deriveKey(pin, salt)

        const iv = crypto.getRandomValues(new Uint8Array(12));
        const encrypted = await crypto.subtle.encrypt(
            {name: 'AES-GCM', iv},
            key,
            new TextEncoder().encode(data)
        );
        const buf = new Uint8Array(salt.byteLength + iv.byteLength + encrypted.byteLength);
        buf.set(salt, 0);
        buf.set(iv, salt.byteLength);
        buf.set(new Uint8Array(encrypted), salt.byteLength + iv.byteLength);
        return this._bufToBase64(buf);
    }

    async decryptByPIN(encryptedData, pin) {
        const buf = this._base64ToBuf(encryptedData);
        const salt = buf.slice(0, 16);
        const iv = buf.slice(16, 28);
        const data = buf.slice(28);

        const key = await this.deriveKey(pin, salt)

        const decrypted = await crypto.subtle.decrypt(
            {name: 'AES-GCM', iv},
            key,
            data
        );
        return new TextDecoder().decode(decrypted);
    }

    // -------------------------------------------------------------------------
    // Convert Key
    // -------------------------------------------------------------------------

    // Import a base64 key string as a extractable CryptoKey
    async importKey(base64) {
        const bytes = this._base64ToBuffer(base64);
        return crypto.subtle.importKey(
            'raw', bytes, 'AES-GCM',
            true,  // extractable
            ['encrypt', 'decrypt']
        );
    }

    // Export to a base64 key string as a non-extractable CryptoKey
    async exportKey(key) {
        const exported = await crypto.subtle.exportKey('raw', key);
        return btoa(String.fromCharCode(...new Uint8Array(exported)));
    }

    // -------------------------------------------------------------------------
    // Private helpers
    // -------------------------------------------------------------------------

    _bufToBase64(buf) {
        return btoa(new Uint8Array(buf).reduce((s, b) => s + String.fromCharCode(b), ''));
    }

    _base64ToBuf(b64) {
        return Uint8Array.from(atob(b64), c => c.charCodeAt(0));
    }

    _base64ToBuffer(base64) {
        const s = atob(base64);
        const bytes = new Uint8Array(s.length);
        for (let i = 0; i < s.length; i++) bytes[i] = s.charCodeAt(i);
        return bytes.buffer;
    }
}
