package store

import (
	"fmt"

	"github.com/google/uuid"
)

type MessageAdapterInternal[T ObjectMessageInternal] struct {
	DBInternal[T]
	db DatabaseTs[T]
}

func NewMessageAdapterInternal[T ObjectMessageInternal](db DatabaseTs[T]) *MessageAdapterInternal[T] {
	return &MessageAdapterInternal[T]{
		DBInternal: NewAdapterInternal[T](db),
		db:         db,
	}
}

func (aI *MessageAdapterInternal[T]) GetAllSinceTimeStamp(roomID uuid.UUID, timestamp uint64) ([]T, error) {
	prefix := append(convertUint16ToByte(PrefixRoom), roomID[:]...)

	objArr, err := aI.db.GetObjectsByPrefixAndSinceTs(prefix, timestamp)
	if err != nil {
		return nil, fmt.Errorf("not possible to parse roomID: %w", err)
	}

	return objArr, nil
}
func (aI *MessageAdapterInternal[T]) DeleteAll(roomID uuid.UUID) error {
	return nil
}
