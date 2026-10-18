import {Events} from "./event.js";
import {Pages} from "./page.js";
import {Room} from "../domain/room.js";
import {Notifications} from "./notification.js";
import {Modals} from "./modal.js";
import {InviteCode, InviteError} from "./invite.js";

// Home page: room list, creating new rooms and joining rooms via invite code or QR code.
class ViewHome {
    constructor(events, aes, keyStore, db, qrScanner) {
        this._events = events;
        this._db = db;
        this._aes = aes;   // AESAdapter instance
        this._keyStore = keyStore;
        this._qrScanner = qrScanner;
        this._invite = new InviteCode(aes);
    }

    init() {
        console.log("View Home init");

        this._events.on(Events.STATE.PAGE, async (pageName) => {
            // Safety net: never keep the camera running after the page changed
            this._qrScanner.Stop();

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
            // The scanner starts the camera when the modal is shown and stops it when it is hidden
            this._qrScanner.Start((code) => this._joinRoomByScan(code));
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

    async _joinRoomByPaste() {
        const modalEl = document.getElementById(Modals.HOME.JOIN_ROOM);

        const inviteCodeField = modalEl.querySelector('textarea[name="invite_code"]');
        const inviteCodePinField = modalEl.querySelector('input[name="invite_code_pin"]');

        const code = inviteCodeField.value.trim();
        if (!code) return;

        const invite = await this._decodeInvite(code, inviteCodePinField.value);
        if (!invite) return;

        inviteCodeField.value = "";
        inviteCodePinField.value = "";

        await this._joinRoom(invite);
    }

    // A scanned QR code. A PIN protected code is put into the invite code field,
    // so the user only has to enter the PIN and press "Join by Code".
    async _joinRoomByScan(code) {
        if (this._invite.needsPin(code)) {
            const modalEl = document.getElementById(Modals.HOME.JOIN_ROOM);
            modalEl.querySelector('textarea[name="invite_code"]').value = code;
            modalEl.querySelector('input[name="invite_code_pin"]').focus();
            this._events.emit(Notifications.WARNING, {
                header: "The invite code is protected",
                msg: "Please enter the PIN and press Join by Code",
            });
            return;
        }

        const invite = await this._decodeInvite(code, "");
        if (invite) await this._joinRoom(invite);
    }

    // Returns {i, n, k} or null after showing what is wrong with the code.
    async _decodeInvite(code, pin) {
        try {
            return await this._invite.decode(code, pin);
        } catch (err) {
            if (err instanceof InviteError && err.reason === "pin") {
                this._events.emit(Notifications.WARNING, {
                    header: "The invite code is protected",
                    msg: pin ? "The PIN is not correct for this invite code" : "Please enter the PIN of the invite code",
                });
            } else {
                console.error("Invalid invite code:", err);
                this._events.emit(Notifications.DANGER, {
                    header: "The invite code is not valid",
                    msg: "Please use the invite code or QR code of a ChatBit room",
                });
            }
            return null;
        }
    }

    // invite: {i: roomId, n: roomName, k: base64 room key}
    async _joinRoom(roomObj) {

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
            this._events.emit(Events.WS.JOIN_ROOM, {id: joinRoom.id, notification: true});
        } catch (err) {
            // Room was joined before — just open it
            if (err.name !== "ConstraintError") {
                console.error("Failed to join room:", err);
                return;
            }
        }

        // Close the modal completely before switching the page: the page change replaces the
        // modal HTML, and if that happens during the fade out, "hidden.bs.modal" never fires,
        // the camera keeps running and the backdrop stays.
        this._qrScanner.Stop();
        await this._closeJoinModal();

        this._events.emit(Events.ROOM.OPEN_ROOM, joinRoom.id);
    }

    // Hides the join modal and resolves once the fade out is finished.
    _closeJoinModal() {
        const modalEl = document.getElementById(Modals.HOME.JOIN_ROOM);
        if (!modalEl || !modalEl.classList.contains("show")) return Promise.resolve();

        return new Promise(resolve => {
            // Fallback in case the event never fires
            const timer = setTimeout(resolve, 1000);
            modalEl.addEventListener("hidden.bs.modal", () => {
                clearTimeout(timer);
                resolve();
            }, { once: true });

            this._events.emit(Events.MODAL.CLOSE, Modals.HOME.JOIN_ROOM);
        });
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