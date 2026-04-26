
// QR Code Generator

function ShareRoom(roomId){
    db.room.get({id:roomId}).then(roomObj => {

        exportRoomKey(roomObj.key).then(key => {
            let payload = JSON.stringify({
                i: roomObj.id,
                n: room_name,
                k: key
            });

            $("#ct_qr_code").html("");
            $("#ct_qr_code").qrcode({
                size: 300,
                fill: '#000',
                background: '#fff',
                text: payload,
                mode: 4,
                ecLevel: 'M'
            });

            $('#ct_share_room').modal('show');

        });

    });
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

async function JoinChat() {
    const modal = $("#in_join_room");

    modal.one("shown.bs.modal", async () => {
        try {
            const select = document.getElementById("camera-select");

            const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent);
            if ("BarcodeDetector" in window && !isIOS) {
                // Native: Chrome / Android (not iOS – BarcodeDetector unreliable in iOS PWA/WKWebView)
                const video = document.getElementById("reader-video");
                video.style.display = "";
                const detector = new BarcodeDetector({ formats: ["qr_code"] });

                const startNativeCamera = async (deviceId) => {
                    if (scannerStream) scannerStream.getTracks().forEach(t => t.stop());
                    const constraints = deviceId
                        ? { video: { deviceId: { exact: deviceId }, width: { ideal: 1280 }, height: { ideal: 720 } } }
                        : { video: { facingMode: "environment", width: { ideal: 1280 }, height: { ideal: 720 } } };
                    scannerStream = await navigator.mediaDevices.getUserMedia(constraints);
                    video.srcObject = scannerStream;
                };

                await startNativeCamera();

                const devices = (await navigator.mediaDevices.enumerateDevices()).filter(d => d.kind === "videoinput");
                select.innerHTML = devices.map((d, i) =>
                    `<option value="${d.deviceId}">${d.label || "Kamera " + (i + 1)}</option>`
                ).join("");
                select.onchange = () => startNativeCamera(select.value);

                function scan() {
                    if (!scannerStream) return;
                    detector.detect(video).then(codes => {
                        if (codes.length > 0) { onScanSuccess(codes[0].rawValue); return; }
                        setTimeout(scan, 100);
                    }).catch(() => setTimeout(scan, 100));
                }
                video.addEventListener("playing", scan, { once: true });

            } else {
                // Fallback: iOS Safari – Html5Qrcode (ZXing-basiert)
                html5QrScanner = new Html5Qrcode("reader");
                const cameras = await Html5Qrcode.getCameras();
                select.innerHTML = cameras.map(c =>
                    `<option value="${c.id}">${c.label}</option>`
                ).join("");
                const startCamera = async (deviceId) => {
                    if (html5QrScanner.isScanning) await html5QrScanner.stop();
                    await html5QrScanner.start(
                        deviceId,
                        {
                            fps: 25,
                            qrbox: (w, h) => {
                                const size = Math.floor(Math.min(w, h) * 0.8);
                                return { width: size, height: size };
                            }
                        },
                        onScanSuccess,
                        undefined
                    );
                };
                select.onchange = () => startCamera(select.value);
                const defaultCamera = cameras.find(c => /back|rear|environment/i.test(c.label)) ?? cameras[0];
                select.value = defaultCamera.id;
                await startCamera(defaultCamera.id);
            }
        } catch (err) {
            if (err.name === "NotAllowedError") {
                alert("Kamera-Zugriff verweigert.");
            } else {
                console.error("Kamera-Fehler:", err);
            }
            modal.modal("hide");
        }
    });

    modal.modal("show");
}