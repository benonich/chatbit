import {Events} from "./event.js";

export class Push {
    constructor(events) {
        this._events = events;
    }

    async init() {
        this._events.on(Events.WS.READY, async ()=> {
            await this.subscribe();
        });
    }

    async subscribe(){
        console.log('subscribe push notifications');

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
        if (!swRegistration) {
            console.warn('No service worker registration found');
            return;
        }
        let pushManager = swRegistration.pushManager;
        if (!this.isPushManagerActive(pushManager)) {
            return;
        }

        let subscriptionOptions = {
            userVisibleOnly: true,
            applicationServerKey: this.urlBase64ToUint8Array(vapidPublicKey)
        };

        try {
            let subscription = await pushManager.subscribe(subscriptionOptions);

            this._events.emit(Events.WS.SUBSCRIBE_PUSH, {endpoint: subscription.endpoint,
                p256dh: this.arrayBufferToBase64url(subscription.getKey('p256dh')),
                auth: this.arrayBufferToBase64url(subscription.getKey('auth'))})

            // Here you can send fetch request with subscription data to your backend API for next push sends from there
        } catch (error) {
            console.warn('Some error occurred, unable to subscribe: ', error);
        }
    }

    async unsubscribe() {
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

    arrayBufferToBase64url(buffer) {
        return btoa(String.fromCharCode(...new Uint8Array(buffer)))
            .replace(/\+/g, '-')
            .replace(/\//g, '_')
            .replace(/=+$/, '');
    }

    isPushManagerActive(pushManager) {
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

    urlBase64ToUint8Array(base64String) {
        const padding = '='.repeat((4 - base64String.length % 4) % 4);
        const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
        const rawData = atob(base64);
        return Uint8Array.from([...rawData].map(c => c.charCodeAt(0)));
    }

}