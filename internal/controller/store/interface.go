package store

type Database[T any] interface {
	DeleteObjectByID(ID []byte) error
	AddObject(ID []byte, obj T) error
	GetObjectByID(ID []byte) (T, error)
	DeleteObjectsByPrefix(prefix []byte) error
	GetObjectsByPrefixAndSinceTs(prefix []byte, timestamp uint64) ([]T, error)
}

type DBInternal[T ObjectInternal] interface {
	Delete(ID string) error
	Add(obj T) error
	Get(ID string) (T, error)
}

type DBMessageInternal[T ObjectMessageInternal] interface {
	DBInternal[T]
	GetAllSinceTimeStamp(roomID string, timestamp uint64) ([]T, error)
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
