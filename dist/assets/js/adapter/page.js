import {Events} from "./event.js";

export class Page {
    constructor(events) {
        this._events = events;
    }

    async init() {
        this._events.on(Events.CHANGE.PAGE, async (pageName) => {
            await this.setPage(pageName);
        });
    }


    async setPage(pageName){
        window.history.pushState({}, "", "/" + pageName);

        const responseHeader = await fetch("assets/view/" + pageName + "_header.html");
        const htmlStringHeader = await responseHeader.text()

        const responseContent = await fetch("assets/view/" + pageName + "_content.html");
        const htmlStringContent = await responseContent.text()

        document.getElementById('appContent').innerHTML = htmlStringContent;
        document.getElementById('appHeader').innerHTML = htmlStringHeader;

        this._events.emit(Events.STATE.PAGE, pageName);
    }

    async _OLDsetPage(pageName){
        window.history.pushState({}, "", "/"+pageName);

        console.log("Set actual page to " + pageName);

        const response = await fetch("assets/view/" + pageName + ".html");
        const htmlString = await response.text()

        document.getElementById('appContent').innerHTML = htmlString;

        const parser = new DOMParser();
        const doc = parser.parseFromString(htmlString, 'text/html');

        const contentDiv = doc.getElementById('appContent');
        const headerDiv = doc.getElementById('appHeader');

        document.getElementById('appContent').innerHTML = contentDiv.innerHTML;
        document.getElementById('appHeader').innerHTML = headerDiv.innerHTML;

        await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));

        this._events.emit(Events.STATE.PAGE, pageName);
    }

    getPage(){
        return window.location.pathname.slice(1)
    }




}


export const Pages = {
    LOGIN: "login",
    ROOM: "room",
    LOCK: "lockscreen",
    HOME: "home",
};