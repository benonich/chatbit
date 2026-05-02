let db;
let db_key;
let db_alias;
let db_rooms;

const queryString = window.location.search;
const urlParams = new URLSearchParams(queryString);
const mainPath = "/";
const SS_key = "chatbit_key";
const SS_room_id = "chatbit_room_id";

async function InitDatabase(){
    console.log("open DB")
    db = new Dexie("ChatBit_DB");

// DB with single table "friends" with primary key "id" and
// indexes on properties "name" and "age"
    console.log("set DB struct")
    db.version(1).stores({
        client:` 
                    &id,
                    &uid`,
        alias: ` 
                    &id,
                    &uid, 
                    name`,
        chat: `
                    &id,
                    room_id,
                    synced,
                    protocol,
                    timestamp`,
        room: `
                    &id,
                    name,
                    key,
                    push_notifications`,
        peer: `
                    &[alias_id+room_id],
                    alias_id,
                    room_id`,
        file:`
                    &id,
                    room_id,
                    filename,
                    mime_type`
    });

    console.log("check if alias exist")
    // get user information from DB
    db_alias = await GetAlias();

    console.log("load pwd from session")
    db_key = base64ToKey(sessionStorage.getItem(SS_key));
}

async function CleanActiveRoom(){
    await db.chat.where('room_id').equals(room_id).delete();
    await db.file.where('room_id').equals(room_id).delete();

    $("#ct_room").html("");

    timeStampNow = undefined;
}


function GetAllRooms(){
    $("#id_room_list").html("");

    db.room.toArray().then(arr => {
        arr.forEach(en => {
            decryptData(en.name, db_key).then(name => {
                $("#id_room_list").append("    <li>\n" +
                    "                        <a href=\"/room\" data-id=\""+en.id+"\" class=\"item col\">\n" +
                    "                    <div class=\"icon-box bg-primary\">\n" +
                    "                        <ion-icon name=\"chatbubble-ellipses-outline\" class=\"md hydrated\"></ion-icon>\n" +
                    "                    </div>" +
                    "                    <div class=\"in\">" +
                    "                            <div>"+name+"</div>\n" +
                    "                    </div>\n" +
                    // "                    <span class=\"badge badge-danger\">0</span>" +
                    "                        </a>\n" +
                    "                    </li>");
            });
        });
    });

    document.getElementById('id_room_list').addEventListener('click', e => {
        const link = e.target.closest('a[data-id]');
        if (!link) return;

        e.preventDefault();

        OpenRoom(link.dataset.id);
    });

}

function handleAutoScroll() {
    const container = document.documentElement; // Oder das spezifische Chat-Element

    // Prüfen, ob der User fast ganz unten ist (mit 50px Toleranz)
    const isAtBottom = (window.innerHeight + window.scrollY) >= (container.scrollHeight - 50);

    if (isAtBottom) {
        window.scrollTo({
            top: container.scrollHeight,
            behavior: 'smooth' // 'smooth' für sanftes Gleiten, 'auto' für sofortigen Sprung
        });
    }
}


function ShareActiveRoom(){
    return ShareRoom(sessionStorage.getItem(SS_room_id));
}


function OpenRoom(roomID){
    sessionStorage.setItem(SS_room_id, roomID);

    SetActualPage(pageRoom);
}

async function exportRoomKey(roomKey){
    return await decryptData(roomKey, db_key);
}

async function getRoomKey(roomKey){

}

function LoadRooms(){
    db.room.toArray().then(all => {
        db_rooms = all;
    })
}


function Logout(){
    sessionStorage.clear();

    DeleteDB()

    window.location = mainPath + "login";
}

// Login
function Login(){
    SetAlias($("#lo_alias").val(), $("#lo_password").val()).then(r => {
        OpenDatabase($("#lo_password").val()).then(r => {
            // only if user want to store pwd
            keyToBase64(db_key).then(key => {
                sessionStorage.setItem(SS_key, key);

                SetActualPage(pageMain);
            });
        });
    });
}

function Lock(){

    sessionStorage.clear();

    SetActualPage(pageLock);
}


function Unlock(){
    OpenDatabase($("#ls_password").val()).then((resposne) => {
        if(!resposne){
            // show notification password incorrect
            notification("ls_pwd_incorrect", 5000);

            // clean up password field
            $("#ls_password").val("");
        }else{
            keyToBase64(db_key).then(key => {
                sessionStorage.setItem(SS_key, key);
                SetActualPage(pageMain);
            })
        }
    });

}

function DeleteDB(){
    db.delete()
}

async function SetAlias(alias, password){
    let salt = await crypto.getRandomValues(new Uint8Array(16));
    let iv = await crypto.getRandomValues(new Uint8Array(12));
    let aesKey = await deriveAESKey(password, salt);

    let verify = await crypto.subtle.encrypt(
        { name: 'AES-GCM', iv },
        aesKey,
        new TextEncoder().encode('verify')
    );

    db.alias.add(
        {
            id: 0, uid: createUUID(),
            name: alias,
            verify: Array.from(new Uint8Array(verify)),
            salt: salt,
            iv: iv
        });

    db_alias = await GetAlias();
}

async function GetAlias(){
    return db.alias.get({id: 0});
}

async function GetClientUID(){
    let client = await db.client.get({id: 0});
    if (client?.uid) return client.uid;
    const uid = crypto.randomUUID();
    await db.client.put({id: 0, uid});
    return uid;
}

// generate key for aes encryption
async function deriveAESKey(password, salt) {
    // Passwort importieren
    const keyMaterial = await crypto.subtle.importKey(
        'raw',
        new TextEncoder().encode(password),
        'PBKDF2',
        false,
        ['deriveKey']
    );

    // AES-256 Key ableiten
    return await crypto.subtle.deriveKey(
        {
            name: 'PBKDF2',
            salt: salt, // Uint8Array(16)
            iterations: 600000,
            hash: 'SHA-256'
        },
        keyMaterial,
        { name: 'AES-GCM', length: 256 },
        true,
        ['encrypt', 'decrypt']
    );
}

async function ConvertAesKeyToBase64(key){
    const exported = await crypto.subtle.exportKey('raw', key);

    return btoa(String.fromCharCode(...new Uint8Array(exported)));
}

// Beim Login – Passwort prüfen
async function checkPassword(password, check, saltI, ivI) {

    const salt = new Uint8Array(saltI);
    const iv = new Uint8Array(ivI);
    const verify = new Uint8Array(check);

    try {
        const key = await deriveAESKey(password, salt);
        await crypto.subtle.decrypt(
            { name: 'AES-GCM', iv },
            key,
            verify
        );
        return true; // Entschlüsselung erfolgreich → Passwort korrekt
    } catch {
        return false; // Entschlüsselung fehlgeschlagen → falsches Passwort
    }
}

async function OpenDatabase(password){
    let key =  await deriveAESKey(password, db_alias.salt);

    if(await checkPassword(password, db_alias.verify, db_alias.salt, db_alias.iv)){
        console.log("password correct...");

        db_key = key;

        return true;

    }

    db_key = null;

    console.log("password not correct...");

    return false;
}


function createUUID() {
    return crypto.randomUUID();
}


// for large strings, use this from https://stackoverflow.com/a/49124600
const buff_to_base64 = (buff) => btoa(
    new Uint8Array(buff).reduce(
        (data, byte) => data + String.fromCharCode(byte), ''
    )
);

function base64ToArrayBuffer(base64) {
    const binaryString = atob(base64);
    const bytes = new Uint8Array(binaryString.length);
    for (let i = 0; i < binaryString.length; i++) {
        bytes[i] = binaryString.charCodeAt(i);
    }
    return bytes.buffer;
}





