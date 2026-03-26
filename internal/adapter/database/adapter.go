package database

import (
	"log/slog"

	badger "github.com/dgraph-io/badger/v4"
)

const (
	Path       = "chatbit_db.kv"
	PrefixRoom = uint16(0)
	PrefixChat = uint16(1)
)

var (
	prefixRoom = ConvertUint16ToByte(PrefixRoom)
	prefixChat = ConvertUint16ToByte(PrefixChat)
)

type Adapter[T any] struct {
	db *badger.DB
}

func NewAdapter[T any]() *Adapter[T] {
	return &Adapter[T]{}
}

func (a *Adapter[T]) Open() {
	var err error

	a.db, err = badger.Open(badger.DefaultOptions(Path))
	if err != nil {
		slog.Error("not possible to open badger db",
			"error", err)
	}
}

func (a *Adapter[T]) Close() {
	err := a.db.Close()
	if err != nil {
		slog.Error("not possible to close badger db",
			"error", err)
	}
}

func ConvertUint16ToByte(v uint16) []byte {
	return []byte{
		byte(v >> 8), // High Byte
		byte(v),      // Low Byte
	}
}
