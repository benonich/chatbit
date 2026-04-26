const vapidPublicKey = document.querySelector('meta[name="vapid-public-key"]').content;

function urlBase64ToUint8Array(base64String) {
    const padding = '='.repeat((4 - base64String.length % 4) % 4);
    const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
    const rawData = atob(base64);
    return Uint8Array.from([...rawData].map(c => c.charCodeAt(0)));
}

function isPushManagerActive(pushManager) {
    if (!pushManager) {
        if (!window.navigator.standalone) {
            document.getElementById('add-to-home-screen').style.display = 'block';
        } else {
            throw new Error('PushManager is not active');
        }
        document.getElementById('subscribe_btn').style.display = 'none';
        return false;
    } else {
        return true;
    }
}

function arrayBufferToBase64url(buffer) {
    return btoa(String.fromCharCode(...new Uint8Array(buffer)))
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '');
}

async function subscribe() {
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
        console.warn('Push not supported');
        return;
    }

    let permission = await Notification.requestPermission();

    if (permission !== 'granted') {
        console.warn('Notification permission denied');
        permission = Notification.permission;
    }

    if (permission !== 'granted') {
        console.warn('Notification permission denied');
        return;
    }

    let swRegistration = await navigator.serviceWorker.getRegistration();
    let pushManager = swRegistration.pushManager;
    if (!isPushManagerActive(pushManager)) {
        return;
    }

    let subscriptionOptions = {
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(vapidPublicKey)
    };

    try {
        let subscription = await pushManager.subscribe(subscriptionOptions);
        ws.subscribePush({id: 0, endpoint: subscription.endpoint,
            p256dh: arrayBufferToBase64url(subscription.getKey('p256dh')),
            auth: arrayBufferToBase64url(subscription.getKey('auth'))});

        // Here you can send fetch request with subscription data to your backend API for next push sends from there
    } catch (error) {
        console.warn('Some error occurred, unable to subscribe: ', error);
    }

    /*

    const registration = await navigator.serviceWorker.ready;

    // Bestehende Subscription prüfen
    let subscription = await registration.pushManager.getSubscription();

    if (!subscription) {
        subscription = await registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: urlBase64ToUint8Array(vapidPublicKey)
        });
    }

    ws.subscribePush({id: 0, endpoint: subscription.endpoint,
        p256dh: btoa(String.fromCharCode(...new Uint8Array(subscription.getKey('p256dh')))),
        auth: btoa(String.fromCharCode(...new Uint8Array(subscription.getKey('auth'))))});
*/
    console.log('Push subscription saved');
}

async function unsubscribe() {
    const registration = await navigator.serviceWorker.getRegistration();
    if (!registration) return;

    const subscription = await registration.pushManager.getSubscription();
    if (!subscription) return;

    await fetch('/api/push/unsubscribe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ endpoint: subscription.endpoint })
    });

    await subscription.unsubscribe();
    console.log('Unsubscribed');
}