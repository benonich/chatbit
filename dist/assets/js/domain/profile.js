// domain/Room.js
// Represents a chat room — always holds decrypted name
// key stays encrypted (only KeyStore ever decrypts it)

export class Profile {

    constructor({id, uid, name, salt, verify, clientId}) {
        this.id = id ?? 0;
        this.uid = uid ?? crypto.randomUUID();
        this.name = name;
        this.salt = salt;
        this.verify = verify;
        this.clientId = clientId ?? crypto.randomUUID();
    }

    // -------------------------------------------------------------------------
    // Factories — JSON → Room
    // -------------------------------------------------------------------------

    // From a DB row — adapter decrypts name before calling this
    static fromDB(row) {
        return new Profile({
            id: row.id,
            uid: row.uid,
            name: row.name,
            salt: row.salt,
            verify: row.verify,
            clientId: row.clientId,
        });
    }

    // -------------------------------------------------------------------------
    // Serialization — Room → JSON (adapter encrypts name before storing)
    // -------------------------------------------------------------------------

    toDB() {
        return {
            id: this.id,
            uid: this.uid,
            name: this.name,
            salt: this.salt,
            verify: this.verify,
            clientId: this.clientId,
        };
    }

}
