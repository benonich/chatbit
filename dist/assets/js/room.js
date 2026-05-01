let room_id;
let room_name;
let room_key;
let room_ttl;
let room_store;

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

    $('#in_new_room_store').on('change', function () {
        if ($(this).is(':checked')) {
            $('#in_new_room_ttl_row').show();
        } else {
            $('#in_new_room_ttl_row').hide();
        }
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
        room_ttl = roomObj.ttl;
        room_store = roomObj.store;

    });

    $("#ct_msg_input").on("keydown", async function(e) {
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            await SendMessage();
        }
    });

    await LoadRoomKey();

    await GetAllMessages();

    console.log("room loaded")

    const el = $("#ct_room")[0];
    el.scrollTop = el.scrollHeight;

    document.getElementById('goBackToMain').addEventListener('click', e => {
        e.preventDefault();
        SetActualPage(pageMain);
    });

    $('#ct_edit_room_store').on('change', function () {
        if ($(this).is(':checked')) {
            $('#ct_edit_room_ttl_row').show();
        } else {
            $('#ct_edit_room_ttl_row').hide();
        }
    });
}

let timeStampNow;


async function AddRoom(){
    const key = await getNewAesKey();

    const room_id = createUUID();

    const notification = $("#in_new_room_push").is(":checked");

    // join room
    joinRooms(room_id, notification);

    db.room.add({
        id: room_id,
        name: await encryptData($("#in_new_room_name").val(), db_key),
        key: await encryptData(key, db_key),
        push_notifications: notification,
        store: $("#in_new_room_store").prop("checked"),
        ttl: $("#in_new_room_ttl").val() * $("#in_new_room_ttl_unit").val()
    }).then(r => {
        $("#in_new_room_name").val("");
        $('#in_add_room').modal('hide');
        $("#in_new_room_push").prop("checked", false);
        $("#in_new_room_ttl").val(0);
        $("#in_new_room_ttl_unit").val(0);

        OpenRoom(room_id);
    });
}

async function UpdateRoom(){

    const notification = $("#ct_edit_room_push").is(":checked");

    db.room.update(room_id, {
        name: await encryptData($("#ct_edit_room_name").val(), db_key),
        push_notifications: notification,
        ttl: $("#ct_edit_room_ttl").val() * $("#ct_edit_room_ttl_unit").val(),
        store: $("#ct_edit_room_store").prop("checked")
    }).then(r => {
        $("#ct_edit_room_name").val("");
        $('#ct_edit_room').modal('hide');
        $("#ct_edit_room_push").prop("checked", false);
    });

    // join room
    joinRooms(room_id, notification);
}

function AddMessageToRoom(id, msg, alias_id, alias, timeStamp, synced, typeI){
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
    console.log("typeI:", typeI)
    if(typeI === 1){
        msg = "<button type=\"button\" class=\"btn btn-secondary\" onclick='showImageModal(\""+id+"\", \""+msg+"\")'>Image</button>"
    }else{
        msg = $("<div/>").text(msg).html();
    }
    msgHtml += "                <div class=\"bubble text-line-break\">\n" + msg + "\n" +
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

    console.log("set status", synced)
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
    $("#ct_room").html("");
    timeStampNow = undefined;

    let msgs = await db.chat.where({room_id:room_id}).limit(100).sortBy('timestamp');

    for (const msg of msgs) {
        let msgD = await DecryptMsg(msg.message);
        let aliasD = await DecryptMsg(msg.alias);
        let aliasIDD = await DecryptMsg(msg.alias_id);
        let timeStamp = new Date(msg.timestamp);
        let timeStamp_received = new Date(msg.timestamp_received);


        console.log("msgT:", msg.type)
        AddMessageToRoom(msg.id, msgD, aliasIDD, aliasD, timeStamp, msg.synced, msg.type);
    }
}

async function SendFile(f) {
    // ignore empty messages
    if(f.files.length === 0){
        return
    }

    const file = f.files[0];

    const totalChunks = Math.ceil(file.size / CHUNK_SIZE);

    let timeStamp = new Date();

    let msgB = await EncryptMsg(file.type);
    let aliasE = await EncryptMsg(db_alias.name);
    let aliasIDE = await EncryptMsg(db_alias.uid);

    let msg = {
        id: createUUID(),
        room_id: room_id,
        alias: aliasE,
        alias_id: aliasIDE,
        message: msgB,
        timestamp: timeStamp.getTime(),
        synced: false,
        ttl: room_ttl,
        store: room_store,
        type: 1,

        mime_type: file.type,
        total_size: file.size,

        chunk_size: CHUNK_SIZE,
        chunks_total: totalChunks,
    };

    ws.transferStart(msg)

    AddMessageToRoom(msg.id, file.type, db_alias.uid, db_alias.name, timeStamp, false, msg.type);

    const fileEnc = await encryptFile(file, room_key)

    await db.file.add({
        id: msg.id,
        room_id: room_id,
        blob: fileEnc,
        mime_type: file.type,
        timestamp: timeStamp.getTime(),
    })

    await ws.sendFile(fileEnc, totalChunks, msg.id);

    f.files = null;

    db.chat.add(msg);

    ws.sendMessage(msg);
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
        synced: false,
        ttl: room_ttl,
        store: room_store
    };

    $("#ct_msg_input").val("");
    $("#ct_msg_input")[0].oninput();

    AddMessageToRoom(msg.id, msgRaw, db_alias.uid, db_alias.name, timeStamp, false, msg.type);
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

            const ttl = roomObj.ttl;

            let ttl_unit = 60;

            if (ttl > 86400) {
                ttl_unit = 86400;
            } else if (ttl > 3600) {
                ttl_unit = 3600;
            }

            $("#ct_edit_room_ttl").val( ttl / ttl_unit);
            $("#ct_edit_room_ttl_unit").val(ttl_unit);
            $("#ct_edit_room_store").prop("checked", roomObj.store);

            if ($('#ct_edit_room_store').is(':checked')) {
                $('#ct_edit_room_ttl_row').show();
            } else {
                $('#ct_edit_room_ttl_row').hide();
            }
        });
    });



    $('#ct_edit_room_store').on('change', function () {
        if ($(this).is(':checked')) {
            $('#ct_edit_room_ttl_row').show();
        } else {
            $('#ct_edit_room_ttl_row').hide();
        }
    });

}


async function showImageModal(fileId) {
    const file = await db.file.get(fileId);
    if (!file) return;

    const url = await decryptFile(file.blob, "image/jpg", room_key);

    console.log("Image URL:", url);

    const img = document.querySelector('#ct_image img');

    if (img.dataset.objectUrl) URL.revokeObjectURL(img.dataset.objectUrl);

    img.src = url;
    img.dataset.objectUrl = url;

    $("#ct_image").modal('show');


    document.getElementById('share-btn').addEventListener('click', async (e) => {
        e.preventDefault();
        const imgSrc = document.getElementById('modal-image').src;

        // Web Share API (iPhone/Android)
        if (navigator.share) {
            try {
                const response = await fetch(imgSrc);
                const blob = await response.blob();
                const file = new File([blob], 'image.jpg', { type: blob.type });

                await navigator.share({
                    files: [file],
                    title: 'Image',
                });
            } catch (err) {
                if (err.name !== 'AbortError') console.error(err);
            }
        } else {
            // Fallback: Download
            const a = document.createElement('a');
            a.href = imgSrc;
            a.download = 'image.jpg';
            a.click();
        }
    });

    $("#ct_image").on('hidden.bs.modal', () => {
        const img = document.querySelector('#ct_image img');
        if (img.dataset.objectUrl) {
            URL.revokeObjectURL(img.dataset.objectUrl);
            delete img.dataset.objectUrl;
            img.src = '';
        }
    });
}