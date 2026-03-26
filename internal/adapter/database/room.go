package database

import (
	"chatbit/internal/domain"
	"fmt"
)

func (a *Adapter) DeleteRoomByID(roomID string) error {
	return a.DeleteObjectByID(prefixRoom, roomID)
}

func (a *Adapter) AddRoom(room domain.Room) error {
	err := a.AddObject(prefixRoom, room.ID, room)
	if err != nil {
		return fmt.Errorf("not possible store Room ID: %s to database: %v", room.ID, err)
	}

	return nil
}

func (a *Adapter) GetRoom(ID string) error {
	var room domain.Room
	err := a.GetObjectByID(prefixRoom, ID, &room)
	if err != nil {
		return fmt.Errorf("not possible get Room ID: %s from database: %v", ID, err)
	}

	return nil
}
