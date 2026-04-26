//--------------------------------------------------------------------------
// You can find dozens of practical, detailed, and working examples of 
// service worker usage on https://github.com/mozilla/serviceworker-cookbook
//--------------------------------------------------------------------------

// Cache version
var CACHE_VERSION = '0.0.0-dev5'

// Cache name
var CACHE_NAME = 'chatbit-v'+CACHE_VERSION;

// Files
var ASSETS_TO_CACHE = [
  'index.html',
  'view/login.html',
  'view/room.html',
  'view/lockscreen.html',
  'view/home.html',
  'assets/css/style.css',
  'assets/css/inc/bootstrap/bootstrap.min.css',
  'assets/css/inc/splide/splide.min.css',
  'assets/js/lib/html5-qrcode-2.3.8.min.js',
  'assets/js/lib/bootstrap.min.js',
  'assets/js/lib/dexie.js',
  'assets/js/lib/jquery-3.7.0.min.js',
  'assets/js/lib/jquery-qrcode.min.js',
  'assets/js/base.js',
  'assets/js/database.js',
  'assets/js/index.js',
  'assets/js/room.js',
  'assets/js/ws.js',
  'assets/img/LogoBenoni.svg',
  'manifest.json'
];

const IGNORED_PATHS = ['/api/', '/ws/'];

const IS_LOCALHOST =
    self.location.hostname === 'localhost' ||
    self.location.hostname === '127.0.0.1';

/* =========================
   INSTALL
========================= */
self.addEventListener('install', event => {
    if (IS_LOCALHOST) {
        // Kein Caching im Dev-Modus
        self.skipWaiting();
        return;
    }

    event.waitUntil(
        caches.open(CACHE_NAME).then(cache => {
            return cache.addAll(ASSETS_TO_CACHE);
        })
    );
});

/* =========================
   ACTIVATE
========================= */
self.addEventListener('activate', event => {
    if (IS_LOCALHOST) {
        return;
    }

    event.waitUntil(
        caches.keys().then(keys =>
            Promise.all(
                keys
                    .filter(key => key !== CACHE_NAME)
                    .map(key => caches.delete(key))
            )
        )
    );

    self.clients.claim();
});

/* =========================
   FETCH
========================= */
self.addEventListener('fetch', event => {
    const { request } = event;

    /* 🚫 DEV-MODUS: immer Network */
    if (IS_LOCALHOST) {
        return;
    }

    const url = new URL(event.request.url);

    /* 🚫 Non-HTTP schemes (e.g. chrome-extension://) */
    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
        return;
    }

    /* 🚫 API-Requests niemals cachen */
    if (IGNORED_PATHS.some(path => url.pathname.startsWith(path))) {
        return;
    }


    /* 🧭 SPA-Navigation → index.html */
    if (request.mode === 'navigate') {
        event.respondWith(
            fetch(request).catch(() => caches.match('/index.html'))
        );
        return;
    }

    /* 📦 Assets: Cache First */
    event.respondWith(
        caches.match(request).then(cached => {
            if (cached) {
                return cached;
            }

            return fetch(request).then(response => {
                if (!response || response.status !== 200) {
                    return response;
                }

                const responseClone = response.clone();
                const requestUrl = new URL(request.url);
                if (requestUrl.protocol === 'http:' || requestUrl.protocol === 'https:') {
                    caches.open(CACHE_NAME).then(cache => {
                        cache.put(request, responseClone);
                    });
                }

                return response;
            });
        })
    );
});


self.addEventListener('push', (event) => {
    // PushData keys structure standart https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerRegistration/showNotification
    let pushData = event.data.json();
    if (!pushData || !pushData.title) {
        console.error('Received WebPush with an empty title. Received body: ', pushData);
    }
    self.registration.showNotification(pushData.title, pushData)
        .then(() => {
            console.log("Push received title: ", pushData.title);



            // You can save to your analytics fact that push was shown
            // fetch('https://your_backend_server.com/track_show?message_id=' + pushData.data.message_id);
        });
});

self.addEventListener('notificationclick', function (event) {
    event.notification.close();

    if (!event.notification.data) {
        console.error('Click on WebPush with empty data, where url should be. Notification: ', event.notification)
        return;
    }
    if (!event.notification.data.url) {
        console.error('Click on WebPush without url. Notification: ', event.notification)
        return;
    }

    clients.openWindow(event.notification.data.url)
        .then(() => {
            console.log("Notification was clicked: ", event.notification);
            // You can send fetch request to your analytics API fact that push was clicked
            // fetch('https://your_backend_server.com/track_click?message_id=' + pushData.data.message_id);
        });
});


/*

  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      icon: '/icon-192.png',        // App Icon
      badge: '/badge.png',          // Kleines Icon in Statusleiste
      image: '/preview.png',        // Grosses Bild (optional)
      tag: 'message',               // Gruppiert Notifications
      renotify: true,               // Vibriert auch bei gleichem tag
      data: { url: data.url }       // Für click-handler
    })
  );
});

self.addEventListener('notificationclick', event => {
  event.notification.close();

  const chatUrl = event.notification.data.url; // z.B. "/chat/123"

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true })
      .then(clientList => {
        // PWA bereits offen? → fokussieren und navigieren
        for (const client of clientList) {
          if (client.url.includes(self.location.origin)) {
            client.focus();
            return client.navigate(chatUrl);
          }
        }
        // PWA nicht offen → neues Fenster
        return clients.openWindow(chatUrl);
      })
  );
});




self.addEventListener("push", (event) => {

    console.log("Push received");
    console.log(event.data?.json());


    //let data = { title: "Push", body: "Neue Nachricht", url: "/" };
    //try { if (event.data) data = { ...data, ...event.data.json() }; } catch { console.error("Push data is not valid JSON")}
    const data = event.data.json();
    event.waitUntil(
        self.registration.showNotification(data.title, {
            body: data.body,
            icon: "/assets/img/favicon.png",
            badge: "/assets/img/favicon.png",
            data: { url: "/" },
            vibrate: [200, 100, 200],
            requireInteraction: true,
            silent: false
        })
    );
});

self.addEventListener('notificationclick', (event) => {
    event.notification.close();

    event.waitUntil(
        clients.matchAll({
            type: 'window',
            includeUncontrolled: true
        }).then((clientList) => {
            // Vergleich entfernt — einfach erstes Fenster fokussieren
            if (clientList.length > 0) {
                return clientList[0].focus();
            }
            // Kein Fenster offen — neues öffnen
            return clients.openWindow('/');
        })
    );
});

 */