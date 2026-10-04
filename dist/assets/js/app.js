import {EventBus, Events} from './adapter/event.js';
import { AesAdapter }        from './adapter/aes.js';
import { DatabaseAdapter }        from './adapter/database.js';
import { WebSocketAdapter }        from './adapter/websocket.js';
import { KeyStore }        from './adapter/key_store.js';
import { Page, Pages }        from './adapter/page.js';
import { ViewLock } from "./adapter/view_lock.js";
import ViewHome from "./adapter/view_home.js";
import { ViewRoom } from "./adapter/view_room.js";
import {QRScanner} from "./adapter/scanner.js";
import {Modal} from "./adapter/modal.js";
import {Notification} from "./adapter/notification.js";
import {Push} from "./adapter/push.js";

export class App {
    constructor() {
        this.events = new EventBus();
        this.aes    = new AesAdapter();
        this.db     = new DatabaseAdapter(this.aes, this.events );
        this.ws     = new WebSocketAdapter(this.events, this.db);
        this.keyStore = new KeyStore(this.aes, this.db);
        this.page = new Page(this.events);
        this.viewLock = new ViewLock(this.events, this.keyStore, this.db);
        this.viewHome = new ViewHome(this.events, this.aes, this.keyStore, this.db, new QRScanner());
        this.viewRoom = new ViewRoom(this.events, this.aes, this.keyStore, this.db);
        this.modal = new Modal(this.events);
        this.notification = new Notification(this.events);
        this.push = new Push(this.events);
    }

    async init() {
        console.log("App init");
        await this.db.init();
        await this.page.init();
        this.ws.init();
        this.viewLock.init();
        this.viewHome.init();
        await this.viewRoom.init();
        await this.modal.init();
        await this.notification.init();
        await this.push.init();

        await this._checkIsUserLoggedIn();

        this._activateListener();

    }

    _activateListener(){
        console.log("start event listener")

        document.addEventListener("click", (e) => {
            console.log("click event");

            const btn = e.target.closest("[data-action]");
            if (!btn) return;

            console.log("click", btn.dataset.action, btn.dataset.body);

            this.events.emit(btn.dataset.action, btn.dataset.body);
        })
    }

    async _checkIsUserLoggedIn(){
        const page = ""

        console.log("Check if user is logged in");

        if (!this.keyStore.isUnlocked()){
            console.log("database is locked");
            if (!await this.db.isInitialized()) {
                console.log("database is not initialized");
                this.events.emit(Events.CHANGE.PAGE, Pages.LOGIN);
            } else {
                console.log("database is not initialized");
                this.events.emit(Events.CHANGE.PAGE, Pages.LOCK);
                $("#ls_alias").text("TODO");
            }
        } else {
            this.events.emit(Events.CHANGE.PAGE, Pages.HOME);
        }
    }

    async register(form){
        console.log("New Register");

        const password = form.querySelector('#lo_password').value;
        const alias = form.querySelector('#lo_alias').value;

        await this.keyStore.register(alias, password);



        await this._unlockEvent();
    }



    async _unlockEvent(){
        const profile = await this.db.getProfile();

        this.events.emit(Events.STATE.UNLOCK, profile);

        this.events.emit(Events.CHANGE.PAGE, Pages.HOME);
    }
}