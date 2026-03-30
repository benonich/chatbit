package store

import (
	"fmt"

	"github.com/google/uuid"
)

type MessageAdapterInternal[T ObjectMessageInternal] struct {
	DBInternal[T]
	db Database[T]
}

func NewMessageAdapterInternal[T ObjectMessageInternal](db Database[T]) *MessageAdapterInternal[T] {
	return &MessageAdapterInternal[T]{
		DBInternal: NewAdapterInternal[T](db),
		db:         db,
	}
}

func (aI *MessageAdapterInternal[T]) GetAllSinceTimeStamp(roomID string, timestamp uint64) ([]T, error) {
	parsedUUID, err := uuid.Parse(roomID)
	if err != nil {
		return nil, fmt.Errorf("not possible to parse roomID: %w", err)
	}

	prefix := append(convertUint16ToByte(PrefixRoom), parsedUUID[:]...)

	objArr, err := aI.db.GetObjectsByPrefixAndSinceTs(prefix, timestamp)
	if err != nil {
		return nil, fmt.Errorf("not possible to parse roomID: %w", err)
	}

	return objArr, nil
}
func (aI *MessageAdapterInternal[T]) DeleteAll(roomID string) error {
	return nil
}
