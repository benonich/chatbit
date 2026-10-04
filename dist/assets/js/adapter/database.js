// adapters/DatabaseAdapter.js
// Responsible for: IndexedDB access and alias/key management
//
// Receives AESAdapter as a constructor argument (= Dependency Injection)
// DatabaseAdapter does not care HOW encryption works, only THAT it works

import {Events} from "./event.js";
import {Profile} from "../domain/profile.js";
import {Room} from "../domain/room.js";
import {Message} from "../domain/message.js";

export class DatabaseAdapter {

    constructor(aes, events) {
        this._events = events;
    }

    async init() {
        this.db = new Dexie('ChatBit_DB');

        this.db.version(1).stores({
            profile:  '&id, &uid, name',
            message:   '&id, roomId, timestamp',
            room:   '&id',
            file:   '&id, roomId, timestamp',
        });

        // subscribe to new messages
        this._events.on(Events.WS.RECEIVED_MESSAGE, async (msg) => {
            this.db.message.add(msg);
        });
    }

    async isInitialized() {
        const profile = await this.db.profile.get({id: 0});
        return profile !== undefined;
    }

    async addProfile(profile) {
        await this.db.profile.add(profile.toDB());
    }

    async getProfile() {
        const profile =  await this.db.profile.get({id: 0})
        return Profile.fromDB(profile);
    }

    async addFile(file) {
        await this.db.message.add(file);
    }

    async getFile(fileId) {
        return  await this.db.message.where({id:fileId});
    }

    async updateMessage(msg) {
        await this.db.message.add(msg.id, msg);
    }

    async addMessage(msg) {
        await this.db.message.add(msg);
    }

    async getMessage(id) {
        return this.db.message.where({id:id});
    }

    async getLatestTimestamp() {
        const messages = await this.db.message.orderBy('timestamp').last();

        return messages ? messages.timestamp : Date.now();
    }

    async getLatestMessages(roomId) {
        const messages = await this.db.message.where({ roomId: roomId }).limit(100).sortBy('timestamp');

        let messagesArray = [];

        messages.forEach(room => {
            console.log(room);
            messagesArray.push(Message.fromDB(room));
        })

        return messagesArray;
    }

    async removeRoom(roomId) {
        await this.db.room.where('id').equals(roomId).delete();
    }

    async updateRoom(room) {
        await this.db.room.put(room, room.id);
    }

    async addRoom(room) {
        await this.db.room.add(room);
    }

    async cleanRoom(roomId) {
        await this.db.message.where('roomId').equals(roomId).delete();
        await this.db.file.where('roomId').equals(roomId).delete();
    }

    async getRoom(roomId) {
        const roomDB =  await this.db.room.where({id: roomId}).first();

        return Room.fromDB(roomDB);
    }

    async getRooms() {
        const rooms = await this.db.room.toArray();

        let roomsArray = [];

        rooms.forEach(room => {
            console.log(room);
            roomsArray.push(Room.fromDB(room));
        })

        return roomsArray;
    }


    // --- Cleanup ---

    async clear() {
        await this.db.profile.clear();
        await this.db.message.clear();
        await this.db.room.clear();
        await this.db.file.clear();
    }
}
