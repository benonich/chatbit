const config = {
    iceServers: [
        { urls: "stun:chat.benoni.ch:3478" },
        {
            urls: [
                "turn:chat.benoni.ch:3478?transport=udp",
                "turn:chat.benoni.ch:3478?transport=tcp",
                "turns:chat.benoni.ch:5349"
            ],
            username: "benoni",
            credential: "benoniPassword"
        }
    ]

};

class RTCPeer {
    constructor(config, alias, signalingChannel, onMessageReceived) {
        this.config = config;
        this.peers = new Map();
        this.candidates = new Map();
        this.alias = alias;
        this.signalingChannel = signalingChannel;
        this.onMessageReceived = onMessageReceived;
    }

    _setupDataChannel(dc) {
        let dataChannel = dc;

        dataChannel.onopen = () => {
            console.log("DataChannel ist offen – du kannst jetzt chatten.");
        };

        dataChannel.onclose = () => {
            console.log("DataChannel geschlossen.");
        };

        dataChannel.onmessage = this.onMessageReceived;
    }

    // ICE Gathering komplett abwarten, damit Offer/Answer alle Kandidaten enthält
    _waitForIceGatheringComplete(rtc) {
        if (rtc.iceGatheringState === "complete") {
            return Promise.resolve();
        }
        return new Promise(resolve => {
            function checkState() {
                if (rtc.iceGatheringState === "complete") {
                    rtc.removeEventListener("icegatheringstatechange", checkState);
                    resolve();
                }
            }
            rtc.addEventListener("icegatheringstatechange", checkState);
        });
    }

    async handleCandidate(data){
        console.log("got ICE Candidate : "+" " + data.candidate.from);

        let obj = this.peers.get(data.candidate.from)

        if(obj !== undefined && obj.rtc.remoteDescription){
            const candidateInit = data.candidate.candidate;
            await obj.rtc.addIceCandidate(candidateInit);
        }

        let candidates = this.candidates.get(data.candidate.from)

        if(candidates === undefined){
            candidates = [];
            this.candidates.set(data.candidate.from, candidates);
        }

        candidates.push(data.candidate.candidate)

    }

    async handleAnswer(data){
        console.log("got RTC answer : "+" "+data.answer.from);

        let obj = this.peers.get(data.answer.from)

        if(obj.rtc.signalingState !== "have-local-offer"){
            console.warn("RTC signaling state is not have-local-offer: " + obj.rtc.signalingState);
            return;
        }

        try {
            console.log("Got answer from peer: "+" "+data.answer.from);
            console.log(data.answer.answer);

            const answerText = data.answer.answer;
            const answerDesc = new RTCSessionDescription(answerText);
            await obj.rtc.setRemoteDescription(answerDesc);

            let pendingCandidates =  this.candidates.get(data.answer.from);
            if(pendingCandidates !== undefined && pendingCandidates.length > 0) {
                for (const c of pendingCandidates) {
                    await rtc.addIceCandidate(c);
                }
                pendingCandidates.length = 0;
            }
        } catch (err) {
            console.error(err);
        }
    }

    async handleOffer(data){
        console.log("start peer connection id: "+" "+data.offer.from);

        let rtc = new RTCPeerConnection(this.config);
        let dc = rtc.createDataChannel(data.offer.from);
        let disconnectTimer = null;

        this.peers.set(data.offer.from, {
            rtc: rtc,
            dc: dc,
            disconnectTimer: disconnectTimer,
        });

        rtc.onicecandidate = (e) => {
            if (e.candidate) {
                this.signalingChannel("rtc_candidate",
                    {
                        room_id: data.room_id,
                        candidate: {
                            from: this.alias,
                            to: data.offer.from,
                            candidate: e.candidate
                        },
                    });
            }
        }


        rtc.onconnectionstatechange = () => {
            console.log("Verbindungsstatus: " + rtc.connectionState);
        };

        rtc.ondatachannel = (event) => {
            this._setupDataChannel(event.channel);
        };

        try {
            console.log("Got offer from peer: "+" "+data.offer.from);
            console.log(data.offer.offer);
            const offerText = data.offer.offer;

            const offerDesc = new RTCSessionDescription(offerText);
            await rtc.setRemoteDescription(offerDesc);

            let pendingCandidates =  this.candidates.get(data.offer.from);
            if(pendingCandidates !== undefined && pendingCandidates.length > 0) {
                for (const c of pendingCandidates) {
                    await rtc.addIceCandidate(c);
                }
                pendingCandidates.length = 0;
            }

            const answer = await rtc.createAnswer();
            await rtc.setLocalDescription(answer);
            //await this._waitForIceGatheringComplete(rtc);

            console.log("send answer to:"+" "+data.offer.from);
            this.signalingChannel("rtc_answer",
                {
                    room_id: data.room_id,
                    answer: {
                        from: this.alias,
                        to: data.offer.from,
                        answer: rtc.localDescription,
                    },
                });
        } catch (err) {
            console.error(err);
        }
    }

    isPeerOnline(peerID){
        if (!this.peers.has(peerID)){
            return false;
        }

        return this.peers.get(peerID).rtc.connectionState === "connected";
    }

    send(peerID, type, msg){
        let rtcMsg = {
            type: type,
            data: msg,
        }

        this.peers.get(peerID).dc.send(JSON.stringify(rtcMsg));
    }

    sendMessage(peerID, msg){
        this.peers.has(peerID)

        let rtcMsg = {
            type: "message",
            data: msg,
        }
        this.peers.get(peerID).dc.send(JSON.stringify(rtcMsg));
    }

    async init(room_id, alias){
        console.log("start peer init");
        let rtc = new RTCPeerConnection(this.config);
        let dc = rtc.createDataChannel(alias);

        this.peers.set(alias, {
            rtc: rtc,
            dc: dc,
        });

        rtc.onicecandidate =  (e) => {
            if (e.candidate) {
                console.log("candidate");
                console.log(e.candidate);
                this.signalingChannel("rtc_candidate",
                    {
                        room_id: room_id,
                        candidate: {
                            from: this.alias,
                            to: alias,
                            candidate: e.candidate
                        },
                    });
            }
        }

        rtc.oniceconnectionstatechange = async () => {
            const state = rtc.iceConnectionState;
            console.log("ICE:", state);

            if (state === "disconnected") {
                await new Promise(resolve => setTimeout(resolve, 1000) );
                if (rtc.iceConnectionState === "disconnected") {
                    console.warn("ICE still disconnected → restart");
                    const offer = await rtc.createOffer({ iceRestart: true });
                    await rtc.setLocalDescription(offer);
                    this.signalingChannel("rtc_offer",
                        {
                            room_id: room_id,
                            offer: {
                                from: this.alias,
                                to: alias,
                                offer: rtc.localDescription,
                            },
                        });
                }
            }

            if (state === "connected" || state === "completed") {
                // Nothing to do here
            }

            if (state === "failed") {
                console.error("ICE failed → hard reconnect");
                await new Promise(resolve => setTimeout(resolve, 1000) );
                await this.init(room_id, alias)

            }
        };

        rtc.onconnectionstatechange = () => {
            console.log("Verbindungsstatus: " + rtc.connectionState);
        };

        rtc.ondatachannel = (event) => {
            this._setupDataChannel(event.channel);
        };

        try {
            console.log("setup data channel ");
            this._setupDataChannel(dc);
            const offer = await rtc.createOffer();
            await rtc.setLocalDescription(offer);
            this.signalingChannel("rtc_offer",
                {
                    room_id: room_id,
                    offer: {
                        from: this.alias,
                        to: alias,
                        offer: rtc.localDescription,
                    },
                });

            console.log("send offer");
        } catch (err) {
            console.error(err);
        }

    }
}

const rtcMessageHandler = async (evt) => {
    console.log("New Message received");

    // The server may bundle multiple messages separated by newlines
    const messages = evt.data.split('\n');

    for (const messageText of messages) {
        if (!messageText.trim()) continue;

        try {
            const msg = JSON.parse(messageText);
            await handleIncomingMessageRTC(msg);
        } catch (e) {
            console.error("Failed to parse message JSON:", messageText, e);
        }
    }
};

async function handleIncomingMessageRTC(msg) {
    console.log(msg);
    switch (msg.type) {
        case "message":
            await receiveMessageWS(msg.data);
            console.log("received new message");
            break;
        default:
            console.warn("Unknown message type:", msg.type);
    }
}




function signalingChannel(type, msg) {
   ws.send(type, msg);
}

let peers= null;
let presenceRequestUID = createUUID();

async function CheckPresenceAll(){
    console.log("check presence");
    if (peers !== null){
        return;
    }

    peers = new Map();

    const rooms = await db.room.toArray();

    for (const room of rooms) {
        ws.send("presence_request", {
            id: presenceRequestUID,
            room_id: room.id,
            alias_id: db_alias.uid,
        });
    }
}

function JoinRoomRTC(room_id){
    if (peers === null){
        return;
    }

    ws.send("presence_request", {
        id: presenceRequestUID,
        room_id: room_id,
        alias_id: db_alias.uid,
    });
}

function GetPresenceAnswer(data){
    if (data.id !== presenceRequestUID){
        return;
    }
    console.log("get presence answer");

    console.log(data.alias_id + " is online room ID: " + data.room_id);

    db.peer.add({alias_id:data.alias_id, room_id:data.room_id}).then(r => {})

    peers.set(data.alias_id, {
        connected: true,
        timestamp: new Date(),
    });
}


function SendPresenceAnswer(data){
    if (data.id === presenceRequestUID){
        return;
    }

    console.log("get presence request");

    console.log(data.alias_id + " is online room ID: " + data.room_id);

    db.peer.add({alias_id:data.alias_id, room_id:data.room_id}).then(r => {})

    peers.set(data.alias_id, {
        connected: true,
        timestamp: new Date(),
    });

    ws.send("presence_answer", {
        id: data.id,
        room_id: data.room_id,
        alias_id: db_alias.uid,
    });
}

async function SendRTCMessage(room_id, msg){

    const peers = await db.peer.where("room_id").equals(room_id).toArray();

    if (peers.length === 0){
        return false;
    }

    for (const peer of peers) {
        if (!rtc.isPeerOnline(peer.alias_id)){
            return false;
        }

        rtc.sendMessage(peer.alias_id, msg);
    }

    return true;
}