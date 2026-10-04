
async function getNewAesKey(){
    const key = await window.crypto.subtle.generateKey(
        {
            name: "AES-GCM",
            length: 256,
        },
        true,
        ["encrypt", "decrypt"]
    )

    return await keyToBase64(key);

}

async function keyToBase64(key){
    const exported = await window.crypto.subtle.exportKey("raw", await key);
    const exportedKeyBuffer = new Uint8Array(exported);

    return btoa(String.fromCharCode(...exportedKeyBuffer));
}

async function base64ToKey(base64) {
    const bytes = base64ToArrayBuffer(base64)

    return await window.crypto.subtle.importKey(
        "raw",
        bytes,
        "AES-GCM",
        true,
        [
            "encrypt",
            "decrypt",
        ]
    )
}

const base64_to_buf = (b64) =>
    Uint8Array.from(atob(b64), (c) => c.charCodeAt(null));


const deriveKey = (passwordKey, salt, keyUsage) =>
    window.crypto.subtle.deriveKey(
        {
            name: "PBKDF2",
            salt: salt,
            iterations: 250000,
            hash: "SHA-256",
        },
        passwordKey,
        {name: "AES-GCM", length: 256},
        true,
        keyUsage
    );

const enc = new TextEncoder();
const dec = new TextDecoder();

async function encryptFile(fileOrBlob, key) {
    const iv = window.crypto.getRandomValues(new Uint8Array(12));
    const buf = await fileOrBlob.arrayBuffer();

    const encrypted = await window.crypto.subtle.encrypt(
        { name: "AES-GCM", iv },
        await key,
        buf
    );

    const result = new Uint8Array(12 + encrypted.byteLength);
    result.set(iv, 0);
    result.set(new Uint8Array(encrypted), 12);
    return result.buffer; // ArrayBuffer — kein Base64-Overhead
}

async function decryptFile(encryptedBuffer, mimeType, key) {
    const data = new Uint8Array(encryptedBuffer);
    const iv = data.slice(0, 12);
    const payload = data.slice(12);

    const decrypted = await window.crypto.subtle.decrypt(
        { name: "AES-GCM", iv },
        await key,
        payload
    );

    return URL.createObjectURL(new Blob([decrypted], { type: mimeType }));
}

async function encryptData(secretData, key) {
    try {
        const iv = window.crypto.getRandomValues(new Uint8Array(12));

        const encryptedContent = await window.crypto.subtle.encrypt(
            {
                name: "AES-GCM",
                iv: iv,
            },
            await key,
            enc.encode(secretData)
        );

        const encryptedContentArr = new Uint8Array(encryptedContent);
        let buff = new Uint8Array(iv.byteLength + encryptedContentArr.byteLength);

        buff.set(iv, 0);
        buff.set(encryptedContentArr, iv.byteLength);
        return buff_to_base64(buff);

    } catch (e) {
        console.log(`Error - ${e}`);
        return "";
    }
}

async function decryptData(encryptedData, key) {
    try {
        const encryptedDataBuff = base64_to_buf(encryptedData);
        const iv = encryptedDataBuff.slice(0, 12);
        const data = encryptedDataBuff.slice(12);

        const decryptedContent = await window.crypto.subtle.decrypt(
            {
                name: "AES-GCM",
                iv: iv,
            },
            await key,
            data
        );
        return dec.decode(decryptedContent);

    } catch (e) {
        console.log(`Error - ${e}`);
        return "";
    }
}
