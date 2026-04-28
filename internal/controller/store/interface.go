package store

import "time"

type Database[T any] interface {
	DeleteObjectByID(ID []byte) error
	AddObject(ID []byte, obj T) error
	AddObjectWithTTL(ID []byte, obj T, ttl time.Duration) error
	GetObjectByID(ID []byte) (T, error)
	DeleteObjectsByPrefix(prefix []byte) error
}

type DatabaseTs[T any] interface {
	DeleteObjectByID(ID []byte) error
	AddObject(ID []byte, obj T) error
	AddObjectWithTTL(ID []byte, obj T, ttl time.Duration) error
	GetObjectByID(ID []byte) (T, error)
	DeleteObjectsByPrefix(prefix []byte) error
	GetObjectsByPrefixAndSinceTs(prefix []byte, timestamp uint64) ([]T, error)
}

type DBInternal[T ObjectInternal] interface {
	Delete(ID string) error
	Add(obj T) error
	AddWithTTL(obj T, ttl int64) error
	Get(ID string) (T, error)
	Update(ID string, fn func(obj T) error) error
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
	GetTimestamp() uint64
}
