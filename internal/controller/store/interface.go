package store

type Database[T any] interface {
	DeleteObjectByID(ID []byte) error
	AddObject(ID []byte, obj T) error
	GetObjectByID(ID []byte) (T, error)
	DeleteObjectsByPrefix(prefix []byte) error
	GetObjectsByPrefixAndSinceTs(prefix []byte, timestamp uint64) ([]T, error)
}

type DBInternal interface {
	Delete(ID string) error
	Add(obj ObjectInternal) error
	Get(ID string, obj ObjectInternal) error
}

type MessageInternal interface {
	DBInternal
	GetAll(roomID string) error
	GetAllFromTimeStamp(roomID string, timestamp int64) error
	DeleteAll(roomID string) error
}

type ObjectInternal interface {
	GetID() string
	GetIDByte() []byte
}

type ObjectMessageInternal interface {
	GetID() string
	GetIDByte() []byte
	GetRoomID() string
	GetRoomIDByte() []byte
}
