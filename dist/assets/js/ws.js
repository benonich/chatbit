const CHUNK_SIZE = 64 * 1024; // 64KB

class WebSocketChannel {
    constructor(onMessageReceived) {
        this.conn = null;
        this.onMessageReceived = onMessageReceived;
        this._pendingAck = null;
        this._bufferedAcks = new Map();
        this._incomingTransfers = new Map();
    }

    _getEndpoint() {
        const protocol = location.protocol === "https:" ? "wss://" : "ws://";
        return `${protocol}${document.location.host}/ws`;
    }

    async _joinAllRooms(db_room){
        let rooms = await db_room.toArray();
        let allRooms = rooms.map(room => ({id: room.id, notification: room.push_notifications}));

        console.log("join rooms", allRooms)

        const lastMsg = await db.chat.orderBy('timestamp').last();
        const last_connection_time = lastMsg ? lastMsg.timestamp : Date.now();

        ws.joinRooms({rooms: allRooms, last_connection_time});
     }

    init(db_room){
        if(this.conn !== null && this.conn.readyState === WebSocket.OPEN){
            return;
        }

        return new Promise(async (resolve, reject) => {
            if (window["WebSocket"]) {

                const clientUid = await GetClientUID();

                this.conn = new WebSocket(this._getEndpoint() + "?client_uid=" + clientUid);

                this.conn.onclose = async (evt) => {
                    console.log("WS Connection closed");
                    console.log(evt);
                    await new Promise(resolve => setTimeout(resolve, 1000));
                    console.log("Try again to connect to WS");
                    await this.init(db_room);
                };

                this.conn.onmessage = (e) => {
                    if (e.data instanceof ArrayBuffer) {
                        this._handleIncomingChunk(e.data);
                    } else {
                        this.onMessageReceived(e);
                    }
                }

                this.conn.binaryType = "arraybuffer";

                this.conn.onopen = () => {
                    console.log("WS Connection connected");
                    this._joinAllRooms(db_room).then();
                    resolve();
                }

                this.conn.onerror = (err) => {
                    console.log("WS Connection error: " + err.message);
                    reject(err);
                }

            } else {
                // ToDo: Error handling
                reject(new Error("WebSockets not supported"));
            }
        });
    }

    send(type, msg){
        let wsMsg = {
            type: type,
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
    }

    sendMessage(msg){
        let wsMsg = {
            type: "message",
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
    }

    joinRooms(msg){
        let wsMsg = {
            type: "join_rooms",
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
    }

    leaveRooms(msg){
        let wsMsg = {
            type: "leave_rooms",
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
    }

    // subscribe to VAPID Push notifications
    subscribePush(msg){
        let wsMsg = {
            type: "sub_push",
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
    }

    transferRequest(msg){
        let wsMsg = {
            type: "transfer_request",
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
    }

    // incoming transfer <-
    handleTransferStart(id, room_id, mime_type, timestamp, chunks_total, chunk_size) {
        console.log("handle transfer start", id, room_id, mime_type, timestamp, chunks_total, chunk_size);

        this._incomingTransfers.set(id, {
            room_id: room_id,
            mime_type: mime_type,
            timestamp: timestamp,
            chunks: new Array(chunks_total).fill(null),
            chunks_size: chunk_size,
            received: 0,
            chunks_total: chunks_total,
        });
    }

    _handleIncomingChunk(data) {
        const view = new DataView(data);
        const uuidBytes = new Uint8Array(data, 0, 16);
        const id = [
            uuidBytes.slice(0,4), uuidBytes.slice(4,6),
            uuidBytes.slice(6,8), uuidBytes.slice(8,10),
            uuidBytes.slice(10,16),
        ].map(b => Array.from(b).map(x => x.toString(16).padStart(2,'0')).join('')).join('-');

        const chunkId = view.getUint32(16, false);
        const encrypted = data.slice(20);

        const t = this._incomingTransfers.get(id);
        console.log("handle incoming chunk", id, chunkId);
        if (!t) return;


        t.chunks[chunkId] = encrypted;
        t.received++;
    }

    async getIncomingTransfer(fileId) {
        console.log("get incoming transfer", fileId);
        const t = this._incomingTransfers.get(fileId);
        if (!t) throw new Error("unknown transfer");

        return t
    }

    async deleteIncomingTransfer(fileId) {
        console.log("delete incoming transfer", fileId);
        this._incomingTransfers.delete(fileId);
    }

    // outgoing transfer ->
    transferStart(msg){
        let wsMsg = {
            type: "transfer_start",
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
    }

    async sendFile(buf, totalChunks, id) {

        for (let i = 0; i < totalChunks; i++) {
            await this.waitForAck(id, i); // Flow control
            const start = i * CHUNK_SIZE;
            const chunk = buf.slice(start, start + CHUNK_SIZE);

            // Chunk-Header: 16 bytes UUID + 4 bytes index
            const header = this.encodeChunkHeader(id, i);
            const packet = this.concat(header, chunk);

            this.conn.send(packet);
        }

        this.conn.send(JSON.stringify({ type: 'transfer_done', data: {id: id} }));
    }

    encodeChunkHeader(id, index) {
        // Format: [16 bytes UUID als bytes][4 bytes uint32 chunk index]
        const buf = new ArrayBuffer(20);
        const view = new DataView(buf);
        const uuidBytes = id.replace(/-/g, '')
            .match(/.{2}/g)
            .map(h => parseInt(h, 16));
        uuidBytes.forEach((b, i) => view.setUint8(i, b));
        view.setUint32(16, index, false); // big-endian
        return buf;
    }

    concat(a, b) {
        const tmp = new Uint8Array(a.byteLength + b.byteLength);
        tmp.set(new Uint8Array(a), 0);
        tmp.set(new Uint8Array(b), a.byteLength);
        return tmp.buffer;
    }

    resolveAck(id, chunkIndex) {
        if (this._pendingAck &&
            id === this._pendingAck.id &&
            chunkIndex === this._pendingAck.chunkIndex) {
            this._pendingAck.resolve();
            this._pendingAck = null;
        } else {
            // ACK kam vor waitForAck → puffern
            this._bufferedAcks.set(`${id}:${chunkIndex}`, true);
        }
    }

    waitForAck(id, chunkIndex) {
        console.log("wait for ack", id, chunkIndex, this._pendingAck)
        const key = `${id}:${chunkIndex}`;
        if (this._bufferedAcks.has(key)) {
            this._bufferedAcks.delete(key);
            return Promise.resolve();
        }
        return new Promise(resolve => {
            this._pendingAck = { id, chunkIndex, resolve };
        });
    }
}

const wsMessageHandler = async (evt) => {
    console.log("New Message received");

    // The server may bundle multiple messages separated by newlines
    const messages = evt.data.split('\n');

    for (const messageText of messages) {
        if (!messageText.trim()) continue;

        try {
            const msg = JSON.parse(messageText);
            await handleIncomingMessage(msg);
        } catch (e) {
            console.error("Failed to parse message JSON:", messageText, e);
        }
    }
};

async function handleIncomingMessage(msg) {
    switch (msg.type) {
        case "message":
            await receiveMessageWS(msg.data);
            console.log("received new message");
            break;
        case "transfer_ack":
            console.log("received ack", msg);
            ws.resolveAck(msg.data.id, msg.data.chunk)
            break;
        case "transfer_nack":
            console.log("received nack");
            break;
        case "transfer_start":
            console.log("transfer_start", msg.data);
            ws.handleTransferStart(msg.data.id, msg.data.room_id, msg.data.mime_type, msg.data.timestamp, msg.data.chunks_total, msg.data.chunk_size)
            break;
        case "transfer_done":
            console.log("received done", msg.data);

            const t = await ws.getIncomingTransfer(msg.data.id);

            console.log("transfer", t);

            const totalLength = t.chunks.reduce((sum, c) => sum + c.byteLength, 0);

            const combined = new Uint8Array(totalLength);
            let offset = 0;
            for (const chunk of t.chunks) {
                combined.set(new Uint8Array(chunk), offset);
                offset += chunk.byteLength;
            }
            const buffer = combined.buffer;

            console.log("combined", buffer);

            await db.file.add({
                id: msg.data.id,
                room_id: t.room_id,
                blob: buffer,
                mime_type: t.mime_type,
                timestamp: t.timestamp,
            })

            await ws.deleteIncomingTransfer(msg.data.id);

            window.dispatchEvent(new CustomEvent('file_ready', {
                detail: { fileId: msg.data.id }
            }));
            break;
        default:
            console.warn("Unknown message type:", msg.type);
    }
}

async function receiveMessageWS(item) {
    let aliasD = await DecryptMsg(item.alias);
    let aliasIDD = await DecryptMsg(item.alias_id);

     if ( item.room_id === room_id ){
        console.log("received message in room");

        let msgD = await DecryptMsg(item.message);

        const timeStamp = new Date(item.timestamp);

        if (aliasIDD !== db_alias.uid){
            AddMessageToRoom(item.id, msgD, aliasIDD, aliasD, timeStamp, item.synced, item.type)
        }
    }



    if (aliasIDD !== db_alias.uid){
        console.log("received not own message", item);
        await db.chat.add(item);
    }else{
        console.log("received own message", item);

        await db.chat.update(item.id, {synced: true});
        setMessageStatus(item.id, true);
        // add message is on the server side
    }

    const el = $("#ct_room")[0];
    el.scrollTop = el.scrollHeight;
}

async function joinAllRooms() {

}


function joinRooms(room_id, notification) {
    console.log("join room " + room_id)
    ws.joinRooms({rooms: [{id: room_id, notification: notification}]});
}

function leaveRoom(room_id) {
    console.log("leave room " + room_id)
    ws.leaveRooms({rooms: [room_id]});
}