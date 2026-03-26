class WebSocketChannel {
    constructor(onMessageReceived) {
        this.conn = null;
        this.onMessageReceived = onMessageReceived;
    }

    _getEndpoint() {
        const protocol = location.protocol === "https:" ? "wss://" : "ws://";
        return `${protocol}${document.location.host}/ws`;
    }

    async _joinAllRooms(db_room){
        let rooms = await db_room.toArray();
        let allRooms = rooms.map(room => room.id);

        console.log("join rooms " + allRooms)

        ws.joinRooms({rooms: allRooms});
     }

    init(db_room){
        if(this.conn !== null && this.conn.readyState === WebSocket.OPEN){
            return;
        }
        return new Promise((resolve, reject) => {
        if (window["WebSocket"]) {

            this.conn = new WebSocket(this._getEndpoint());

            this.conn.onclose = async (evt) => {
                console.log("WS Connection closed");
                console.log(evt);
                await new Promise(resolve => setTimeout(resolve, 1000) );
                console.log("Try again to connect to WS");
                this.init(db_room);
            };

            this.conn.onmessage = this.onMessageReceived;

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
            type: "join_rooms",
            data: msg,
        }
        this.conn.send(JSON.stringify(wsMsg));
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
        case "presence_request":
            SendPresenceAnswer(msg.data);
            break;
        case "presence_answer":
            GetPresenceAnswer(msg.data);
            break;
        case "rtc_offer":
            if(msg.data.offer.to === db_alias.uid) {
                console.log("got offer from peer");
                await rtc.handleOffer(msg.data);
            }
            break;
        case "rtc_answer":
            if(msg.data.answer.to === db_alias.uid) {
                console.log("got answer from peer");
                await rtc.handleAnswer(msg.data);
            }
            break;
        case "rtc_candidate":
            if(msg.data.candidate.to === db_alias.uid) {
                console.log("got candidate from peer");
                await rtc.handleCandidate(msg.data);
            }
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
        console.log(item);

        let msgD = await DecryptMsg(item.message);

        let timeStamp = new Date(item.timestamp);

        console.log(msgD);

        if (aliasIDD !== db_alias.uid){
            AddMessageToRoom(msgD, aliasIDD, aliasD, timeStamp, item.protocol)
        }
    }

    if (aliasD !== db_alias.name){
        db.chat.add(item);
    }else{
        // add message is on the server side
    }
}

async function joinAllRooms() {

}


function joinRooms(room_id) {
    console.log("join room " + room_id)
    ws.joinRooms({rooms: [room_id]});
    JoinRoomRTC(room_id)
}

function leaveRoom(room_id) {
    console.log("leave room " + room_id)
    ws.leaveRooms({rooms: [room_id]});
}