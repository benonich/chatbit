


async function subscribe() {
    const keyRes = await fetch("/vapidPublicKey");
    const { publicKey } = await keyRes.json();

    navigator.serviceWorker.ready
        .then(function(registration) {
            return registration.pushManager.subscribe({
                userVisibleOnly: true,
                applicationServerKey: urlBase64ToUint8Array(publicKey),
            });
        })
        .then(function(subscription) {
            console.log(
                JSON.stringify({
                    subscription: subscription,
                })
            );
        })
        .catch(err => console.error(err));
}

function urlBase64ToUint8Array(base64String) {
    const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
    const base64 = (base64String + padding)
        .replace(/\-/g, '+')
        .replace(/_/g, '/');
    const rawData = window.atob(base64);
    return Uint8Array.from([...rawData].map(char => char.charCodeAt(0)));
}

if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('service-worker.js');
    navigator.serviceWorker.ready
        .then(function(registration) {
            return registration.pushManager.getSubscription();
        })
        .then(function(subscription) {
            if (!subscription) {
                subscribe();
            } else {
                console.log(
                    JSON.stringify({
                        subscription: subscription,
                    })
                );
            }
        });
}