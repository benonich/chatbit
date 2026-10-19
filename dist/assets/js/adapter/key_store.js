import {Profile} from "../domain/profile.js";

// Minimum length of the password that protects the local profile (register and change)
export const MIN_PASSWORD_LENGTH = 8;

// Holds the unlocked key material in memory only:
//  - the master key, derived from the user password (PBKDF2 via AesAdapter)
//  - a cache of decrypted room keys (room keys are stored encrypted with the master key)
// register() creates the profile, unlock() verifies the password by decrypting profile.verify.
export class KeyStore {
    constructor(aes, db) {
        this._aes       = aes;
        this._db        = db;
        this._masterKey = null;        // derived from password
        this._roomKeys  = new Map();   // roomId → CryptoKey (cache)
        this.uid        = null;
        this.clientId   = null;
    }

    isUnlocked()       { return this._masterKey !== null; }

    lock()             { this._masterKey = null; this._roomKeys.clear(); this.uid = null; this.clientId = null;}

    async register(alias, password){
        const salt = await crypto.getRandomValues(new Uint8Array(16));

        let profile = new Profile({
            name: alias,
            salt: salt,
        });

        this.uid = profile.uid;

        this._masterKey = await this._aes.deriveKey(password, salt);

        profile.verify = await this._aes.encrypt(this.uid, this._masterKey);

        await this._db.addProfile(profile);
    }

    async unlock(password) {
        const profile = await this._db.getProfile();

        const salt = new Uint8Array(profile.salt);

        let key;

        try {
            key = await this._aes.deriveKey(password, salt);

            const verifyResult = await this._aes.decrypt(profile.verify, key)

            if (verifyResult !== profile.uid) return false;
        } catch (err) {
            console.error('Login failed:', err);

            return false
        }

        this._masterKey = key;

        this.uid = profile.uid;

        this.clientId = profile.clientId;

        console.log("unlock success");

        return true;

    }

    // Re-encrypts everything that depends on the master key (profile check value, room names
    // and room keys) with a key derived from the new password. Returns false if the current
    // password is wrong. Messages are encrypted with the room keys and stay untouched.
    async changePassword(currentPassword, newPassword) {
        const profile = await this._db.getProfile();

        let currentKey;
        try {
            currentKey = await this._aes.deriveKey(currentPassword, new Uint8Array(profile.salt));
            if (await this._aes.decrypt(profile.verify, currentKey) !== profile.uid) return false;
        } catch {
            return false;
        }

        const salt = crypto.getRandomValues(new Uint8Array(16));
        const newKey = await this._aes.deriveKey(newPassword, salt);

        // Encrypt everything first — the database write itself must not await WebCrypto
        const rooms = [];
        for (const room of await this._db.getRooms()) {
            rooms.push({
                id: room.id,
                name: await this._aes.encrypt(await this._aes.decrypt(room.name, currentKey), newKey),
                key: await this._aes.encrypt(await this._aes.decrypt(room.key, currentKey), newKey),
            });
        }
        const verify = await this._aes.encrypt(profile.uid, newKey);

        await this._db.replaceMasterKeyData({salt, verify}, rooms);

        this._masterKey = newKey;
        return true;
    }

    async getRoomKey(roomId) {
        if (this._roomKeys.has(roomId)) return this._roomKeys.get(roomId);

        const room = await this._db.getRoom(roomId);

        const key = await this._aes.importKey(
            await this._aes.decrypt(room.key, this._masterKey)
        );

        this._roomKeys.set(roomId, key);

        return key;
    }

    getMasterKey() {
        return this._masterKey;
    }
}