package store

import "fmt"

type MessageAdapterInternal struct {
	db     Database
	prefix []byte
}

func NewMessageAdapterInternal(db Database, prefix uint16) *MessageAdapterInternal {
	return &MessageAdapterInternal{
		db:     db,
		prefix: convertUint16ToByte(prefix),
	}
}

func (aI *MessageAdapterInternal) GetAll(roomID string) error {
	return nil
}

func (aI *MessageAdapterInternal) GetAllFromTimeStamp(roomID string, timestamp int64) error {
	return nil
}
func (aI *MessageAdapterInternal) DeleteAll(roomID string) error {
	return nil
}

func (aI *MessageAdapterInternal) Delete(ID, roomID []byte) error {
	err := aI.db.DeleteObjectByID(append(aI.prefix, append(roomID, ID...)...))
	if err != nil {
		return fmt.Errorf("not possible to delete object from database: %w", err)
	}

	return nil
}

func (aI *MessageAdapterInternal) Add(obj ObjectMessageInternal) error {
	err := aI.db.AddObject(append(aI.prefix, append(obj.GetRoomIDByte(), obj.GetIDByte()...)...), obj)
	if err != nil {
		return fmt.Errorf("not possible store object to database: %v", err)
	}

	return nil
}

func (aI *MessageAdapterInternal) Get(ID, roomID []byte, obj ObjectMessageInternal) error {
	err := aI.db.GetObjectByID(append(aI.prefix, append(roomID, ID...)...), obj)
	if err != nil {
		return fmt.Errorf("not possible get object from database: %v", err)
	}

	return nil
}
