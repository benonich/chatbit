const pageLogin = "login";
const pageRoom = "room";
const pageLock = "lockscreen";
const pageMain = "home";

let ws;
//let rtc;

window.onload = async (event) => {

    await InitDatabase();

    // check if user logged in and redirect to login page or lock page
    CheckIsUserLoggedIn();

    ws = new WebSocketChannel(wsMessageHandler);

    console.log("init WS")

    await PageLoad();

}

async function PageLoad() {
    let response = await fetch("assets/view/" + GetActualPage() + ".html");
    let htmlString = await response.text()

    document.getElementById('appContent').innerHTML = htmlString;

    // HTML-String in ein DOM-Objekt parsen
    let parser = new DOMParser();
    let doc = parser.parseFromString(htmlString, 'text/html');

    let contentDiv = doc.getElementById('appContent');
    let headerDiv = doc.getElementById('appHeader');

    document.getElementById('appContent').innerHTML = contentDiv.innerHTML;
    document.getElementById('appHeader').innerHTML = headerDiv.innerHTML;

    //if (rtc === undefined && db_alias !== undefined) {
    //    rtc = new RTCPeer(config, db_alias.uid, signalingChannel, rtcMessageHandler);
    //}

    switch(GetActualPage()) {
        case pageLogin:
            await PageLoadLogin();
            break;
        case pageRoom:
            room_id = sessionStorage.getItem(SS_room_id);
            await ws.init(db.room);
            await PageLoadRoom();
            await subscribe();
            break;
        case pageMain:
            console.log("init Main Page")
            $("#in_alias").text(db_alias.name);
            await ws.init(db.room);
            await PageLoadMain();
            await subscribe();
            break;
        case pageLock:
            await PageLoadLock();
            break;
    }
}

function SetActualPage(name){
    window.history.pushState({}, "", "/"+name);

    console.log("Set actual page to " + name);

    PageLoad().then();
}

function GetActualPage(){
    return window.location.pathname.slice(1)
}

function CheckIsUserLoggedIn(){
    if (sessionStorage.getItem(SS_key) === null){

        if (db_alias === undefined) {
            if (GetActualPage() !== pageLogin) {
                window.history.pushState({}, "", "/"+pageLogin);
            }
        } else {
            if (GetActualPage() !== pageLock) {
                window.history.pushState({}, "", "/"+pageLock);
            }

            $("#ls_alias").text(db_alias.name);
        }
    }else if (GetActualPage() === pageLock || GetActualPage() === pageLogin){
        window.history.pushState({}, "", "/"+pageMain);
    }
}

function InitLoginPage(){

}