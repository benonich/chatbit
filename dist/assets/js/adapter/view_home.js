import {Events} from "./event.js";
import {Pages} from "./page.js";
import {Room} from "../domain/room.js";
import {Notifications} from "./notification.js";
import {Modals} from "./modal.js";

class ViewHome {
    constructor(events, aes, keyStore, db, qrScanner) {
        this._events = events;
        this._db = db;
        this._aes = aes;   // AESAdapter instance
        this._keyStore = keyStore;
        this._qrScanner = qrScanner;
    }

    init() {
        console.log("View Home init");

        this._events.on(Events.STATE.PAGE, async (pageName) => {
            if (pageName === Pages.HOME) {
                console.log("View Home Page");

                await this.showRooms();
            }
        });

        this._events.on(Events.ROOM.NEW_ROOM_MODAL, async (msg) => {
            console.log("New Room Modal");
            this._events.emit(Events.MODAL.OPEN, Modals.HOME.NEW_ROOM);
        });

        this._events.on(Events.ROOM.JOIN_ROOM_MODAL, async (msg) => {
            console.log("Join Room Modal");
            this._qrScanner.Start();
            this._events.emit(Events.MODAL.OPEN, Modals.HOME.JOIN_ROOM);

        });

        this._events.on(Events.ROOM.NEW_ROOM, async (msg) => {
            console.log("Add Room");
            this.addRoom().catch(err => console.error('addRoom failed:', err));
            this._events.emit(Events.MODAL.CLOSE, Modals.HOME.NEW_ROOM);
            // todo catch error
        });

        this._events.on(Events.ROOM.JOIN_ROOM, async (msg) => {
            console.log("join Room");
            await this._joinRoomByPaste()
            // todo catch error
        });
    }

    async _subscribeToRooms(){

    }

    async _joinRoomByPaste() {
        const modalEl = document.getElementById(Modals.HOME.JOIN_ROOM);

        const inviteCodeField = modalEl.querySelector('textarea[name="invite_code"]');
        const inviteCodePinField = modalEl.querySelector('input[name="invite_code_pin"]');

        const raw = inviteCodeField.value.trim();
        if (!raw) return;

        const binString = atob(raw);
        const bytes = Uint8Array.from(binString, c => c.charCodeAt(0));
        const joinRoomJson = new TextDecoder().decode(bytes);

        let payload;
        try {
            const obj = JSON.parse(joinRoomJson);
            if (obj.v === 1 && obj.enc) {
                const pin = inviteCodePinField.value;
                if (!pin) {
                    this._events.emit(Notifications.WARNING, {
                        header: "The Join string is encrypted",
                        msg: "Please enter the PIN to encrypt the join string",
                    });
                    return;
                }
                try {
                    console.log("Decrypting join string with PIN:", pin);
                    payload = await this._aes.decryptByPIN(obj.data, pin);
                } catch {
                    this._events.emit(Notifications.DANGER, {
                        header: "The Join string is encrypted",
                        msg: "The PIN is not correct for this join string",
                    });
                    return;
                }
            } else if (obj.v === 1 && !obj.enc) {
                payload = obj.data;
            } else {
                this._events.emit(Notifications.DANGER, {
                    header: "The version of the invite code is not known",
                    msg: "Please enter a valid invite code",
                });
                return;
            }
        } catch {
            this._events.emit(Notifications.DANGER, {
                header: "The Join string is not valid",
                msg: "Please enter a valid join string",
            });
            return;
        }

        inviteCodeField.value = "";
        inviteCodePinField.value = "";

        console.log("Joining room: " + payload);

        await this._joinRoom(payload);
    }

    async _joinRoom(rawValue) {
        let roomObj = JSON.parse(rawValue);

        const joinRoom = new Room({
            id: roomObj.i,
            name: await this._aes.encrypt(roomObj.n, this._keyStore.getMasterKey()),
            key: await this._aes.encrypt(roomObj.k, this._keyStore.getMasterKey()),
            pushNotifications: true,
            store: true,
            ttl: 0
        })

        try {
            await this._db.addRoom(joinRoom);

            this._events.emit(Events.ROOM.OPEN_ROOM, joinRoom.id);
            this._events.emit(Events.WS.JOIN_ROOM, {id: joinRoom.id, notification: true});
            this._events.emit(Events.MODAL.CLOSE, Modals.HOME.JOIN_ROOM);
        } catch (err) {
            console.log("ERROR: " + err);
        }
    }

    async addRoom() {
        const newRoom = new Room({
            id: crypto.randomUUID(),
            name: await this._aes.encrypt(document.getElementById("in_new_room_name").value, this._keyStore.getMasterKey()),
            key: await this._aes.encrypt(await this._aes.exportKey(await this._aes.generateKey()), this._keyStore.getMasterKey()),
            pushNotifications: document.getElementById("in_new_room_push").checked,
            store: document.getElementById("in_new_room_store").checked,
            ttl: document.getElementById("in_new_room_ttl").value * document.getElementById("in_new_room_ttl_unit").value
        })

        // send join room request to server over WS
        this._events.emit(Events.WS.JOIN_ROOM, {id: newRoom.id, notification: newRoom.pushNotifications});

        // store new room in the database
        await this._db.addRoom(newRoom);

        this._cleanNewRoomForm();

        this._events.emit(Events.STATE.ROOM, newRoom.id);
    }

    _cleanNewRoomForm() {
        document.getElementById("in_new_room_name").value = "";
        document.getElementById("in_new_room_push").checked = false;
        document.getElementById("in_new_room_ttl").value = 0;
        document.getElementById("in_new_room_ttl_unit").value = 0;

        bootstrap.Modal.getInstance(document.getElementById("in_new_room"))?.hide();
    }

    // -------------------------------------------------------------------------
    // HTML rendering
    // -------------------------------------------------------------------------

    async showRooms() {
        document.getElementById('id_room_list').innerHTML = '';

        const rooms = await this._db.getRooms();

        for (const room of rooms) {
            console.log("Room:", room);

            const name = await this._aes.decrypt(room.name, this._keyStore.getMasterKey());

            document.getElementById('id_room_list').insertAdjacentHTML('beforeend', "    <li>\n" +
                "                        <a href=\"#\" data-action=\"room:open_room\" data-body=\"" + room.id + "\" class=\"item col\">\n" +
                "                    <div class=\"icon-box bg-primary\">\n" +
                "                        <ion-icon name=\"chatbubble-ellipses-outline\" class=\"md hydrated\"></ion-icon>\n" +
                "                    </div>" +
                "                    <div class=\"in\">" +
                "                            <div>" + name + "</div>\n" +
                "                    </div>\n" +
                // "                    <span class=\"badge badge-danger\">0</span>" +
                "                        </a>\n" +
                "                    </li>");
        }
    }
}
export default ViewHome