let room_id;
let room_name;
let room_key;

async function PageLoadLock() {

    CheckIsUserLoggedIn()

    document.getElementById('lock_submit').addEventListener('submit', e => {
        e.preventDefault();
        Unlock();
    });
}

async function PageLoadMain() {
    GetAllRooms();

    document.getElementById('change_to_lock').addEventListener('click', e => {
        e.preventDefault();
        Lock();
    });

    document.getElementById('create_new_chat').addEventListener('click', e => {
        e.preventDefault();
    });

    document.getElementById('join_new_chat').addEventListener('click', e => {
        e.preventDefault();
        JoinChat();
    });
}

async function PageLoadLogin() {
    document.getElementById('login_submit').addEventListener('submit', e => {
        e.preventDefault();
        Login();
    });
}

async function PageLoadRoom() {
    // Get room ID
    room_id = sessionStorage.getItem(SS_room_id);

    console.log("load room ID" + room_id);

    db.room.get({id:room_id}).then(roomObj => {
         decryptData(roomObj.name, db_key).then(k => {
             room_name = k;
             $("#ro_room_name").text(room_name);
         });
    });

    await LoadRoomKey();

    await GetAllMessages();

    $("#ct_msg_input").on("keydown", async function(e) {
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            await SendMessage();
        }
    });

    console.log("room loaded")

    const el = $("#ct_room")[0];
    el.scrollTop = el.scrollHeight;

    document.getElementById('goBackToMain').addEventListener('click', e => {
        e.preventDefault();
        SetActualPage(pageMain);
    });
}

let timeStampNow;


async function AddRoom(){
    const key = await getNewAesKey();

    const room_id = createUUID();

    const notification = $("#in_new_room_push").is(":checked");

    db.room.add({
        id: room_id,
        name: await encryptData($("#in_new_room_name").val(), db_key),
        key: await encryptData(key, db_key),
        push_notifications: notification
    }).then(r => {
        $("#in_new_room_name").val("");
        $('#in_add_room').modal('hide');
        $("#in_new_room_push").prop("checked", false);
        OpenRoom(room_id);
    });

    // join room
    joinRooms(room_id, notification);
}

async function UpdateRoom(){

    const notification = $("#ct_edit_room_push").is(":checked");

    db.room.update(room_id, {
        name: await encryptData($("#ct_edit_room_name").val(), db_key),
        push_notifications: notification
    }).then(r => {
        $("#ct_edit_room_name").val("");
        $('#ct_edit_room').modal('hide');
        $("#ct_edit_room_push").prop("checked", false);
        PageLoadRoom()
    });

    // join room
    joinRooms(room_id, notification);
}

function AddMessageToRoom(id, msg, alias_id, alias, timeStamp, synced){
    let isOnBottom = (window.innerHeight + window.scrollY) >= (document.documentElement.scrollHeight - 25);

    if(timeStampNow === undefined || timeStampNow.getDate() !== timeStamp.getDate() || timeStampNow.getMonth() !== timeStamp.getMonth() || timeStampNow.getFullYear() !== timeStamp.getFullYear()){
        AddDateToRoom(timeStamp);
        timeStampNow = timeStamp;
    }

    let msgHtml = "<div class=\"message-item";
    if(alias_id === db_alias.uid){
        msgHtml += " user";
    }
    msgHtml += "\">\n" +
    "            <div id='"+id+"' class=\"content\">\n";
    if(alias_id !== db_alias.uid){
        msgHtml += "                <div class=\"title\" alias-id='"+alias_id+"'>"+alias+"</div>\n";
    }
    msgHtml += "                <div class=\"bubble text-line-break\">\n" + $("<div/>").text(msg).html() + "\n" +
    "                </div>\n" +
    "                <div class=\"footer\">"+timeStamp.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})+ " ";
    if(alias_id === db_alias.uid){
        msgHtml += renderStatus(synced);
    }
    msgHtml +=  "</div>\n" +
    "            </div>\n" +
    "        </div>"

    $("#ct_room").append(msgHtml)

    if (isOnBottom) {
        const el = $("#ct_room")[0];
        el.scrollTop = el.scrollHeight;
    }
}

function renderStatus(synced) {
    const sent = "<ion-icon name=\"checkmark-outline\"></ion-icon>";
    const received = "<ion-icon name=\"checkmark-done-outline\"></ion-icon>";

    if(synced){
        return received;
    }else{
        return sent;
    }
}

function setMessageStatus(uuid, synced) {
    const footer = $(`#${uuid} .footer`);
    footer.find('ion-icon').remove();

    if (synced) {
        footer.append('<ion-icon name="checkmark-done-outline"></ion-icon>');
    } else {
        footer.append('<ion-icon name="checkmark-outline"></ion-icon>');
    }
}

function AddDateToRoom(timeStamp){
    const msgHtml = "<div class=\"message-divider\">" + timeStamp.toDateString() + "</div>";

    $("#ct_room").append(msgHtml)
}

async function LoadRoomKey(){
    const roomObj = await db.room.get({
        id: room_id
    });

    const roomKeyDec = await decryptData(roomObj.key, db_key);

    room_key = await base64ToKey(roomKeyDec);
}

async function EncryptMsg(msg) {
    return await encryptData(msg, room_key)
}

async function DecryptMsg(msg) {
    return await decryptData(msg, room_key)
}

async function GetAllMessages() {
    let msgs = await db.chat.where({room_id:room_id}).limit(100).sortBy('timestamp');

    for (const msg of msgs) {
        let msgD = await DecryptMsg(msg.message);
        let aliasD = await DecryptMsg(msg.alias);
        let aliasIDD = await DecryptMsg(msg.alias_id);
        let timeStamp = new Date(msg.timestamp);
        let timeStamp_received = new Date(msg.timestamp_received);

        AddMessageToRoom(msg.id, msgD, aliasIDD, aliasD, timeStamp, timeStamp_received, msg.protocol);
    }
}

async function SendMessage() {
    let msgRaw = $("#ct_msg_input").val();

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

    let msgE = await EncryptMsg(msgRaw);
    let aliasE = await EncryptMsg(db_alias.name);
    let aliasIDE = await EncryptMsg(db_alias.uid);

    let msg = {
        id: createUUID(),
        room_id: room_id,
        alias: aliasE,
        alias_id: aliasIDE,
        message: msgE,
        Received: [],
        timestamp: timeStamp.getTime(),
        synced: false
    };

    $("#ct_msg_input").val("");
    $("#ct_msg_input")[0].oninput();

    AddMessageToRoom(msg.id, msgRaw, db_alias.uid, db_alias.name, timeStamp, false, msg.protocol);
    db.chat.add(msg);

    ws.sendMessage(msg);
}

function auto_height(elem) {  /* javascript */
    elem.style.height = "1px";
    elem.style.height = (elem.scrollHeight)+"px";
}

function LeaveActiveRoomModal(){
    $('#ct_leave_room').modal('show');
}

async function LeaveActiveRoom(){
    ws.leaveRooms({rooms: [room_id]});
    await db.chat.where('room_id').equals(room_id).delete()

    await db.room.where('id').equals(room_id).delete();

    leaveRoom(room_id);

    room_id = "";
    room_name = "";
    room_key = "";

    sessionStorage.removeItem(SS_room_id)

    SetActualPage(pageMain);
}

function EditActiveRoom(){
    $('#ct_edit_room').modal('show');

    db.room.get({id:room_id}).then(roomObj => {
        decryptData(roomObj.name, db_key).then(name => {
            $('#ct_edit_room_name').val(name);
            $("#ct_edit_room_push").prop("checked", roomObj.push_notifications);
        });
    });
}
