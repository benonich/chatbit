export class EventBus {
    constructor() {
        this._listeners = {};
    }

    on(event, callback) {
        if (!this._listeners[event]) this._listeners[event] = [];
        this._listeners[event].push(callback);
    }

    emit(event, data) {
        console.log("Emit event " + event + " with data " + data);
        this._listeners[event]?.forEach(cb => cb(data));
    }

    emitPrefix(event, data) {
        for (const key of Object.keys(this._listeners)) {
            if (key.endsWith(':*') && event.startsWith(key.slice(0, -1))) {
                this._listeners[key].forEach(cb => cb(data));
            }
        }
        this._listeners[event]?.forEach(cb => cb(data));
    }

    off(event, callback) {
        this._listeners[event] = this._listeners[event]
            ?.filter(cb => cb !== callback);
    }
}

export const Events = {
    WS: {
        SEND_MESSAGE: 'ws:send_message',
        RECEIVED_MESSAGE: 'ws:received_message',
        JOIN_ROOM: 'ws:join_room',
        LEAVE_ROOM: 'ws:leave_room',
        SUBSCRIBE_PUSH: 'ws:subscribe_push',
        READY: 'ws:ready' // this event is send if ws ready
    },
    CHAT: {
        MESSAGE: 'chat:message',  // todo
        SEND_MESSAGE: 'chat:send_message',
        SYNCED: 'chat:synced',
        FILE_REQUEST: 'chat:file_request',
        FILE_READY: 'chat:file_ready',

        SHARE_ACTIVE_ROOM: 'chat:share_active_room',
        EDIT_ACTIVE_ROOM: 'chat:edit_active_room',
        UPDATE_ACTIVE_ROOM: 'chat:update_active_room',
        LEAVE_ACTIVE_ROOM: 'chat:leave_active_room',
        LEAVE_ACTIVE_ROOM_ASK: 'chat:leave_active_room_ask',
        CLEAN_ACTIVE_ROOM: 'chat:clean_active_room',
        CLEAN_ACTIVE_ROOM_ASK: 'chat:clean_active_room_ask',
        COPY_ACTIVE_ROOM: 'chat:copy_active_room',
    },
    ROOM: {
        NEW_ROOM: 'room:new_room',
        NEW_ROOM_MODAL: 'room:new_room_modal', // this will open the modal to join a room
        JOIN_ROOM_MODAL: 'room:join_room_modal', // this will open the modal to join a room
        JOIN_ROOM: 'room:join_room', // no body read from modal
        EDIT_ROOM: 'room:edit_room', // todo
        LEAVE_ROOM: 'room:leave_room', // todo
        OPEN_ROOM: 'room:open_room' // body: room_id
    },
    // This is the event to prepare the content of the page
    STATE: {
        ROOM: 'state:room',
        PAGE: 'state:page',
        USER: 'state:user',
        UNLOCK: 'state:unlock',
    },
    // CHANGE.PAGE will just load the Html Pages
    CHANGE: {
        PAGE: 'change:page',
    },
    // CHANGE.PAGE will just load the Html Pages
    MODAL: {
        OPEN: 'modal:open',  // body: modal_id
        CLOSE: 'modal:open:',  // body: modal_id
    }
};