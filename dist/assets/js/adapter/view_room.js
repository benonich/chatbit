import {Events} from "./event.js";
import {Pages} from "./page.js";
import {Message} from "../domain/message.js";
import {Modals} from "./modal.js";
import {Room} from "../domain/room.js";

export class ViewRoom {
    constructor(events, aes, keyStore, db) {
        this._events = events;
        this._aes   = aes;
        this._db   = db;
        this._keyStore = keyStore;

        this._roomId = null;
        this._roomTimestamp = null;
        this._roomKey = null;
        this._roomTTL = null;
        this._roomName = null;
        this._roomStore = null;
        this._roomNotification = null;

        this._alias = null;
        this._aliasId = null;
    }

    async init() {
        this._events.on(Events.WS.RECEIVED_MESSAGE, async (msg) => {
            if (msg.isRoom(this._roomId)) {
                console.log("WS Message received");
                console.log(msg);


                const aliasId = await this._aes.decrypt(msg.aliasId, this._roomKey);

                console.log(msg);
                console.log(aliasId);
                if (aliasId === this._aliasId){
                    console.log("Own message received");

                    this._updateStatusHTML(msg);
                }else{
                    console.log("Message received from WS");
                    await this._newMessage(msg);
                }

                const el = document.getElementById("ct_room");
                el.scrollTop = el.scrollHeight;
            }
        });

        this._events.on(Events.CHAT.SEND_MESSAGE, async (msg) => {
            await this.sendMessage();
        });

        this._events.on(Events.CHAT.COPY_ACTIVE_ROOM, async (msg) => {
            await this._copySharedRoom();
        });

        this._events.on(Events.CHAT.SHARE_ACTIVE_ROOM, async (msg) => {
            await this._shareRoom();
        });

        this._events.on(Events.CHAT.EDIT_ACTIVE_ROOM, async (msg) => {
            console.log("Edit Active Room");
            this._events.emit(Events.MODAL.OPEN, Modals.ROOM.EDIT_ROOM);
            this._fillEditRoomModal();
        });

        this._events.on(Events.CHAT.UPDATE_ACTIVE_ROOM, async (msg) => {
            console.log("Update Active Room");
            this._events.emit(Events.MODAL.CLOSE, Modals.ROOM.EDIT_ROOM);
            await this._updateRoomFromModal();
        });

        this._events.on(Events.CHAT.LEAVE_ACTIVE_ROOM_ASK, async (msg) => {
            console.log("Leave Active Room Ask");
            this._events.emit(Events.MODAL.OPEN, Modals.ROOM.LEAVE_ROOM);
        });

        this._events.on(Events.CHAT.LEAVE_ACTIVE_ROOM, async (msg) => {
            console.log("Leave Active Room");

            await this._leaveRoom(this._roomId);

            this._events.emit(Events.WS.LEAVE_ROOM, this._roomId);

            this._events.emit(Events.CHANGE.PAGE, Pages.HOME);
        });

        this._events.on(Events.CHAT.CLEAN_ACTIVE_ROOM, async (msg) => {
            console.log("Clean Active Room");
            this._cleanActiveRoom();

            console.log("Clean Active Room DB ID:", this._roomId);
            await this._db.cleanRoom(this._roomId);
        });

        this._events.on(Events.CHAT.MESSAGE, async (msg) => {
            if (msg.isRoom(this._roomId)) {
                await this._newMessage(msg);
            }
        });

        // this will open the room with the given id (action is coming from the room list)
        this._events.on(Events.ROOM.OPEN_ROOM, async (roomId) => {
            this._events.emit(Events.STATE.ROOM, roomId);
        })

        this._events.on(Events.STATE.ROOM, async (roomId) => {
            console.log("View Room", roomId);

            this._roomId = roomId;
            this._roomKey = await this._keyStore.getRoomKey(roomId);

            console.log("Room Key:", this._roomKey);

            const room = await this._db.getRoom(roomId);

            console.log("Get Room from DB");
            console.log(room);
            this._roomTTL   = room.ttl;
            this._roomStore = room.store;
            this._roomNotification = room.pushNotifications;
            this._roomName = await this._aes.decrypt(room.name, this._keyStore.getMasterKey());

            const profile = await this._db.getProfile();
            this._alias = profile.name;
            this._aliasId = profile.uid;

            this._events.emit(Events.CHANGE.PAGE, Pages.ROOM);
        });

        document.addEventListener('submit', (e) => {
            if (e.target.id === 'send_message_form') {
                e.preventDefault();
                console.log("Send Message");

                this.sendMessage();
            }
        });

        this._events.on(Events.STATE.PAGE, async (pageName) => {
            if(pageName === Pages.ROOM){
                console.log("View Room Page");

                document.getElementById('ro_room_name').innerHTML = this._roomName;

                this._cleanActiveRoom();

                await this._getLatestMessages();
            }
        });
    }

    async _updateRoomFromModal(){
        const modalEl = document.getElementById(Modals.ROOM.EDIT_ROOM);

        const roomName = modalEl.querySelector('input[name="room_name"]');
        const roomNotification = modalEl.querySelector('input[name="room_notification"]');
        const roomDeleteAfter = modalEl.querySelector('input[name="room_delete_after"]');
        const roomDeleteAfterUnit = modalEl.querySelector('select[name="room_delete_after_unit"]');
        const roomStore = modalEl.querySelector('input[name="room_store"]');

        const updateRoom = new Room({
            id: this._roomId,
            name: await this._aes.encrypt(roomName.value, this._keyStore.getMasterKey()),
            key: await this._aes.encrypt(await this._aes.exportKey(this._roomKey), this._keyStore.getMasterKey()),
            pushNotifications: roomNotification.checked,
            store: roomStore.checked,
            ttl: roomDeleteAfter.value * roomDeleteAfterUnit.value
        })

        console.log(updateRoom);

        await this._db.updateRoom(updateRoom);

        this._events.emit(Events.STATE.ROOM, updateRoom.id);
    }

    _fillEditRoomModal(){
        const modalEl = document.getElementById(Modals.ROOM.EDIT_ROOM);

        const roomName = modalEl.querySelector('input[name="room_name"]');
        const roomNotification = modalEl.querySelector('input[name="room_notification"]');
        const roomDeleteAfter = modalEl.querySelector('input[name="room_delete_after"]');
        const roomDeleteAfterUnit = modalEl.querySelector('select[name="room_delete_after_unit"]');
        const roomStore = modalEl.querySelector('input[name="room_store"]');

        roomName.value = this._roomName;
        roomNotification.checked = this._roomNotification;
        roomStore.checked = this._roomStore;

        if(roomStore.checked){
            if (this._roomTTL > 86400){
                roomDeleteAfter.value = this._roomTTL / 86400;
                roomDeleteAfterUnit.value = 86400;
            }else if (this._roomTTL > 3600) {
                roomDeleteAfter.value = this._roomTTL / 3600;
                roomDeleteAfterUnit.value = 3600;
            }else{
                roomDeleteAfter.value = this._roomTTL  / 60;
                roomDeleteAfterUnit.value = 60;
            }
        }
    }

    async _copySharedRoom(){
        const modalEl = document.getElementById(Modals.ROOM.SHARE_ROOM);

        const pinInput = modalEl.querySelector('input[name="share_pin"]');
        const copyBtn = modalEl.querySelector('[data-field="copy_btn"]');

        const pin = pinInput.value;
        let exportStr;

        const sharePayload = JSON.stringify({
            i: this._roomId,
            n: this._roomName,
            k: await this._aes.exportKey(this._roomKey)
        });

        if (pin) {
            const encSharePayload = await this._aes.encryptByPIN(sharePayload, pin);
            exportStr = JSON.stringify({ v: 1, enc: true, data: encSharePayload });
        } else {
            exportStr = JSON.stringify({ v: 1, enc: false, data: sharePayload });
        }

        const bytes = new TextEncoder().encode(exportStr);
        const binString = Array.from(bytes, b => String.fromCharCode(b)).join('');

        await navigator.clipboard.writeText(btoa(binString));

        const original = copyBtn.innerHTML;
        copyBtn.innerHTML = '<ion-icon name="checkmark-outline"></ion-icon> Copied!';
        setTimeout(() => { copyBtn.innerHTML = original; }, 2000);
    }

    async _shareRoom(){
        const sharePayload = JSON.stringify({
            i: this._roomId,
            n: this._roomName,
            k: await this._aes.exportKey(this._roomKey)
        });

        const modalEl = document.getElementById(Modals.ROOM.SHARE_ROOM);

        const qrContainer = modalEl.querySelector('[data-field="qr_code"]');
        const pinInput = modalEl.querySelector('input[name="share_pin"]');

        pinInput.value = "";

        qrContainer.innerHTML = "";

        new QRCode(qrContainer, {
            text: sharePayload,
            width: 300,
            height: 300,
            colorDark: "#000",
            colorLight: "#fff",
            correctLevel: QRCode.CorrectLevel.M
        });

        this._events.emit(Events.MODAL.OPEN, Modals.ROOM.SHARE_ROOM);
    }

    async _leaveRoom(roomId){
        this._db.cleanRoom(roomId);
        this._db.removeRoom(roomId);
    }

    async sendMessage(){
        const msgRaw = document.getElementById("ct_msg_input").value;

        // ignore empty messages
        if(msgRaw.length === 0){
            return
        }

        // check max length
        if(msgRaw.length > 1024){
            notification("ct_msg_maxlength", 5000);
            return
        }

        let timeStamp = new Date();

        let msg = new Message({
            id: crypto.randomUUID(),
            roomId: this._roomId,
            alias: await this._aes.encrypt(this._alias, this._roomKey),
            aliasId: await this._aes.encrypt(this._aliasId, this._roomKey),
            content: await this._aes.encrypt(msgRaw, this._roomKey),
            timestamp: timeStamp.getTime(),
            type: Message.TYPE_TEXT,
            synced: false,
            ttl: this._roomTTL,
            store: this._roomStore
        });

        document.getElementById("ct_msg_input").value = "";

        await this._newMessage(msg);

        this._events.emit(Events.WS.SEND_MESSAGE, msg);

    }

    // -------------------------------------------------------------------------
    // HTML rendering
    // -------------------------------------------------------------------------

    // Render a new message in the active room and add it to the room view
    async _newMessage(msg){
        this._checkTimeDivider(msg);

        await this._renderMessage(msg);
    }

    async _renderMessage(msg) {
        const alias = await this._aes.decrypt(msg.alias, this._roomKey);
        const aliasId = await this._aes.decrypt(msg.aliasId, this._roomKey);

        console.log("Rendering message", msg);

        const content = await this._aes.decrypt(msg.content, this._roomKey);

        const isOwn    = aliasId === this._aliasId;
        const bubble   = msg._renderBubble(content);
        const status   = isOwn ? msg._renderStatus() : '';
        const title    = !isOwn
            ? `<div class="title" alias-id="${msg._escape(aliasId)}">${msg._escape(alias)}</div>`
            : '';

        document.getElementById('ct_room').insertAdjacentHTML('beforeend', `
        <div class="message-item${isOwn ? ' user' : ''}">
            <div id="${msg.id}" class="content">
                ${title}
                <div class="bubble text-line-break">${bubble}</div>
                <div class="footer">${msg.formattedTime} ${status}</div>
            </div>
        </div>`);
    }

    // Clean the active room by removing all message items
    _cleanActiveRoom(){
        $("#ct_room").html("");

        this._roomTimestamp = null;
    }

    // request last x messages and render them
    async _getLatestMessages(){
        const messageArray = await this._db.getLatestMessages(this._roomId);

        for (const msg of messageArray) {
            await this._newMessage(Message.fromDB(msg));
        }
    }

    // Returns the date divider HTML (call when date changes between messages)
    _checkTimeDivider(msg) {
        const timeStamp = msg.date;

        if(this._roomTimestamp === undefined || this._roomTimestamp === null || this._roomTimestamp.getDate() !== timeStamp.getDate() || this._roomTimestamp.getMonth() !== timeStamp.getMonth() || this._roomTimestamp.getFullYear() !== timeStamp.getFullYear()){
            $("#ct_room").append(`<div class="message-divider">${msg.formattedDate}</div>`);
        }

        this._roomTimestamp = timeStamp;
    }

    // Update synced status icon in existing DOM element
    _updateStatusHTML(msg) {
        const footer = $(`#${msg.id} .footer`);
        footer.find('ion-icon').remove();
        footer.append(msg.synced
            ? '<ion-icon name="checkmark-done-outline"></ion-icon>'
            : '<ion-icon name="checkmark-outline"></ion-icon>'
        );
    }

}