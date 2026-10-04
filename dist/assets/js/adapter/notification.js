import {Events} from "./event.js";

export class Notification {
    constructor(events) {
        this._events = events;
    }

    async init() {
        this._events.on(Notifications.INFO, (data) => {
            this._notification("info", data.header, data.msg, data.time);
        });
        this._events.on(Notifications.DANGER, (data) => {
            this._notification("danger", data.header, data.msg, data.time);
        });
        this._events.on(Notifications.WARNING, (data) => {
            this._notification("warning", data.header, data.msg, data.time);
        });
        this._events.on(Notifications.PRIMARY, (data) => {
            this._notification("primary", data.header, data.msg, data.time);
        });
    }

    _notification(type, title, subtitle, msg, time) {
        const appHeader = document.getElementById("appHeader") || document.body;
        if (!appHeader) return;

        if (!time) time = 3000;

        let subtitleHtml;
        if (subtitle) subtitleHtml = `<h3 class="subtitle">${subtitle}</h3>`

        // Vorherige Notifications schließen und entfernen
        const existingBoxes = appHeader.querySelectorAll(".notification-box");
        existingBoxes.forEach((el) => {
            el.classList.remove("show");
            setTimeout(() => el.remove(), 300);
        });

        const template = document.createElement("template");
        template.innerHTML = `
            <div class="notification-box">
                <div class="notification-dialog ios-style bg-${type}">
                    <div class="notification-header">
                        <strong>${title || ""}</h5>
                        <div class="right">
                            <button type="button" class="close-button" aria-label="Close">
                                <ion-icon name="close-circle"></ion-icon>
                            </button>
                        </div>
                    </div>
                    <div class="notification-content">
                        <div class="in">
                            ${subtitleHtml || ""}
                            <div class="text">${msg || ""}</div>
                        </div>
                    </div>
                </div>
            </div>
        `.trim();

        const notificationBox = template.content.firstElementChild;
        appHeader.appendChild(notificationBox);

        let autoCloseTimeout = null;

        const closeNotification = () => {
            if (autoCloseTimeout) {
                clearTimeout(autoCloseTimeout);
                autoCloseTimeout = null;
            }
            notificationBox.classList.remove("show");
            setTimeout(() => {
                if (notificationBox.parentElement) {
                    notificationBox.remove();
                }
            }, 300);
        };

        const closeButton = notificationBox.querySelector(".close-button");
        if (closeButton) {
            closeButton.addEventListener("click", (e) => {
                e.preventDefault();
                closeNotification();
            });
        }

        const dialog = notificationBox.querySelector(".notification-dialog");
        if (dialog) {
            dialog.addEventListener("click", (e) => {
                if (!e.target.closest(".close-button")) {
                    closeNotification();
                }
            });
        }

        // Notification einblenden
        setTimeout(() => {
            notificationBox.classList.add("show");
        }, 50);

        // Nach x Sekunden / Millisekunden wieder entfernen
        if (time) {
            const duration = time < 100 ? time * 1000 : time;
            autoCloseTimeout = setTimeout(() => {
                closeNotification();
            }, duration);
        }
    }
}


export const Notifications = {
    INFO: "notification:info",
    DANGER: "notification:danger",
    WARNING: "notification:warning",
    PRIMARY: "notification:primary"
};