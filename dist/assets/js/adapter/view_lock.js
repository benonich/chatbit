import {Events} from "./event.js";
import {Pages} from "./page.js";

export class ViewLock {
    constructor(events, keyStore, db) {
        this._events = events;
        this._db   = db;
        this._keyStore = keyStore;
    }

    init() {
        this._events.on(Events.STATE.PAGE, async (pageName) => {
            if(pageName === Pages.LOCK){
                await this.setAliasName();
            }
        });
    }

    async setAliasName(){
        const profile = await this._db.getProfile();
        console.log("Setting alias name to:", profile.name);
        document.getElementById('ls_alias').textContent = profile.name;
    }

    async unlock(form){
        const password = form.querySelector('#ls_password').value;

        const ok = await this._keyStore.unlock(password);
        if (!ok) {
            notification("ls_pwd_incorrect", 5000);
            form.querySelector('#ls_password').value = "";
        } else {
            console.log("unlock database");

            await this._unlockEvent();
        }
    }

    async clear() {
        this._keyStore.lock();
        await this._db.clear();

        this._events.emit(Events.CHANGE.PAGE, Pages.LOGIN);
    }

    async _unlockEvent(){
        const profile = await this._db.getProfile();

        this._events.emit(Events.STATE.UNLOCK, profile);

        this._events.emit(Events.CHANGE.PAGE, Pages.HOME);
    }

}