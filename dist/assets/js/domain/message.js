// domain/message.js
// Represents a single chat message — always holds decrypted values
// Encryption/decryption is handled by the adapter layer, not here

export class Message {
    static TYPE_TEXT  = 0;
    static TYPE_IMAGE = 1;

    constructor({ id, roomId, aliasId, alias, content, timestamp, synced, type, ttl, store,
                  mimeType, totalSize, chunkSize, chunksTotal }) {
        this.id          = id ?? crypto.randomUUID();
        this.roomId      = roomId;
        this.aliasId     = aliasId;
        this.alias       = alias;
        this.content     = content;
        this.timestamp   = timestamp ?? Date.now();
        this.synced      = synced ?? false;
        this.type        = type ?? Message.TYPE_TEXT;
        this.ttl         = ttl ?? 0;
        this.store       = store ?? false;
        // File-only fields (undefined for text messages)
        this.mimeType    = mimeType;
        this.totalSize   = totalSize;
        this.chunkSize   = chunkSize;
        this.chunksTotal = chunksTotal;
    }

    // -------------------------------------------------------------------------
    // Factories — JSON → Message
    // -------------------------------------------------------------------------

    // From a decrypted DB row or decrypted WS payload
    // The adapter decrypts alias, alias_id and message before calling this
    static fromJSON(row) {
        return new Message({
            // Required fields
            id:        row.id,
            roomId:    row.roomId,
            aliasId:   row.aliasId,
            alias:     row.alias,
            content:   row.content,
            timestamp: row.timestamp,
            synced:    row.synced,
            type:      row.type,
            ttl:       row.ttl,
            store:     row.store,
            // Optional file fields — undefined when not present, that is fine
            mimeType:    row.mimeType,
            totalSize:   row.totalSize,
            chunkSize:   row.chunkSize,
            chunksTotal: row.chunksTotal,
        });
    }

    static fromDB(row) {
        return new Message({
            // Required fields
            id:        row.id,
            roomId:    row.roomId,
            aliasId:   row.aliasId,
            alias:     row.alias,
            content:   row.content,
            timestamp: row.timestamp,
            synced:    row.synced,
            type:      row.type,
            ttl:       row.ttl,
            store:     row.store,
            // Optional file fields — undefined when not present, that is fine
            mimeType:    row.mimeType,
            totalSize:   row.totalSize,
            chunkSize:   row.chunkSize,
            chunksTotal: row.chunksTotal,
        });
    }

    // -------------------------------------------------------------------------
    // Serialization — Message → JSON (adapter encrypts fields before storing/sending)
    // -------------------------------------------------------------------------

    toJSON() {
        const json = {
            id:        this.id,
            roomId:    this.roomId,
            aliasId:   this.aliasId,
            alias:     this.alias,
            content:   this.content,
            timestamp: this.timestamp,
            synced:    this.synced,
            type:      this.type,
            ttl:       this.ttl,
            store:     this.store,
        };

        // File-only fields — only included when present
        if (this.mimeType    != null) json.mimeType    = this.mimeType;
        if (this.totalSize   != null) json.totalSize   = this.totalSize;
        if (this.chunkSize   != null) json.chunkSize   = this.chunkSize;
        if (this.chunksTotal != null) json.chunksTotal = this.chunksTotal;

        return json;
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------
    isRoom(roomId) { return this.roomId === roomId; }

    isImage()    { return this.type === Message.TYPE_IMAGE; }

    get date()          { return new Date(this.timestamp); }
    get formattedTime() { return this.date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }); }
    get formattedDate() { return this.date.toDateString(); }


    // -------------------------------------------------------------------------
    // Private rendering helpers
    // -------------------------------------------------------------------------

    _renderBubble(content) {
        if (this.isImage()) {
            return `<button type="button" class="btn btn-secondary"
                        data-action="chat:open_image"
                        data-body="${this.id}"
                    >
                        Image
                    </button>`;
        }

        const conR = this._escape(content);
        console.log("ESCAPED CONTENT", conR);
        return conR;
    }

    _renderStatus() {
        return this.synced
            ? '<ion-icon name="checkmark-done-outline"></ion-icon>'
            : '<ion-icon name="checkmark-outline"></ion-icon>';
    }

    _escape(str) {
        const el = document.createElement('div');

        el.textContent = str ?? '';

        return el.innerHTML;
    }
}