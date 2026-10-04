// domain/Room.js
// Represents a chat room — always holds decrypted name
// key stays encrypted (only KeyStore ever decrypts it)

export class Room {

    constructor({ id, name, key, pushNotifications, store, ttl }) {
        this.id                = id;
        this.name              = name;              // decrypted
        this.key               = key;              // encrypted base64 — only KeyStore uses this
        this.pushNotifications = pushNotifications ?? false;
        this.store             = store ?? false;
        this.ttl               = ttl ?? 0;
    }

    // -------------------------------------------------------------------------
    // Factories — JSON → Room
    // -------------------------------------------------------------------------

    // From a DB row — adapter decrypts name before calling this
    static fromDB(row) {
        return new Room({
            id:                row.id,
            name:              row.name,
            key:               row.key,
            pushNotifications: row.pushNotifications,
            store:             row.store,
            ttl:               row.ttl,
        });
    }

    // -------------------------------------------------------------------------
    // Serialization — Room → JSON (adapter encrypts name before storing)
    // -------------------------------------------------------------------------

    toJSON() {
        return {
            id:                 this.id,
            name:               this.name,
            key:                this.key,
            pushNotifications: this.pushNotifications,
            store:              this.store,
            ttl:                this.ttl,
        };
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    get ttlFormatted() {
        if (!this.ttl) return 'Never';
        if (this.ttl >= 86400) return `${this.ttl / 86400}d`;
        if (this.ttl >= 3600)  return `${this.ttl / 3600}h`;
        return `${this.ttl / 60}min`;
    }

    _escapeHtml(str) {
        return $('<div>').text(str).html();
    }
}
