
// QR Code Generator

let sharePayload = null;

function ShareRoom(roomId) {
    db.room.get({id: roomId}).then(roomObj => {
        exportRoomKey(roomObj.key).then(key => {
            sharePayload = JSON.stringify({
                i: roomObj.id,
                n: room_name,
                k: key
            });

            $("#ct_share_pin").val("");
            $("#ct_qr_code").html("");
            $("#ct_qr_code").qrcode({
                size: 300,
                fill: '#000',
                background: '#fff',
                text: sharePayload,
                mode: 4,
                ecLevel: 'M'
            });

            $('#ct_share_room').modal('show');
        });
    });
}

async function encryptSharePayload(payload, pin) {
    const salt = crypto.getRandomValues(new Uint8Array(16));
    const iv   = crypto.getRandomValues(new Uint8Array(12));
    const key  = await deriveAESKey(pin, salt);

    const encrypted = await crypto.subtle.encrypt(
        { name: 'AES-GCM', iv },
        key,
        new TextEncoder().encode(payload)
    );

    return JSON.stringify({
        v:    1,
        enc:  true,
        salt: buff_to_base64(salt),
        iv:   buff_to_base64(iv),
        data: buff_to_base64(encrypted)
    });
}

async function decryptSharePayload(obj, pin) {
    const salt = new Uint8Array(base64ToArrayBuffer(obj.salt));
    const iv   = new Uint8Array(base64ToArrayBuffer(obj.iv));
    const data = base64ToArrayBuffer(obj.data);
    const key  = await deriveAESKey(pin, salt);

    const decrypted = await crypto.subtle.decrypt(
        { name: 'AES-GCM', iv },
        key,
        data
    );

    return new TextDecoder().decode(decrypted);
}


/*
function OnPastePayloadChange() {
    const raw = $("#in_paste_payload").val().trim();
    $("#in_paste_error").hide();
    try {
        const obj = JSON.parse(raw);
        $("#in_paste_pin_wrapper").toggle(obj.v === 1 && obj.enc === true);
    } catch {
        $("#in_paste_pin_wrapper").hide();
    }
}
**/


function showPasteError(msg) {
    const el = document.getElementById('in_paste_error');
    el.textContent = msg;
    el.style.display = '';
    setTimeout(() => { el.style.display = 'none'; }, 4000);
}


// QR Code Scanner

let scannerStream = null;
let html5QrScanner = null;

async function onScanSuccess(rawValue) {
    let roomObj = JSON.parse(rawValue);
    console.log("QR-Code Scanned: " + roomObj);
    try {
        const eKey = await encryptData(roomObj.k, db_key);

        const room_n = await encryptData(roomObj.n, db_key);

        await db.room.add({ id: roomObj.i, name: room_n, key: eKey });
        CloseJoinChat();
        OpenRoom(roomObj.i);
        joinRooms(roomObj.i, true);
    } catch (err) {
        console.log("ERROR: " + err);
    }
}

function CloseJoinChat() {
    if (html5QrScanner) {
        html5QrScanner.stop().catch(() => {});
        html5QrScanner = null;
    }
    if (scannerStream) { scannerStream.getTracks().forEach(t => t.stop()); scannerStream = null; }
    $("#in_join_room").modal("hide");
}

