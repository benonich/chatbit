package store

import (
	"chatbit/internal/domain"
	"encoding/binary"
	"fmt"
)

type AdapterInternal[T ObjectInternal] struct {
	db     Database[T]
	prefix []byte
}

func NewAdapterInternal[T ObjectInternal](db Database[T]) *AdapterInternal[T] {
	var obj T

	var prefix []byte

	switch any(obj).(type) {
	case domain.Message:
		prefix = convertUint16ToByte(PrefixRoom)
	case domain.Room:
		prefix = convertUint16ToByte(PrefixMessage)
	}

	return &AdapterInternal[T]{
		db:     db,
		prefix: prefix,
	}
}

func (aI *AdapterInternal[T]) Delete(ID string) error {
	err := aI.db.DeleteObjectByID(append(aI.prefix, []byte(ID)...))
	if err != nil {
		return fmt.Errorf("not possible to delete object from database: %w", err)
	}

	return nil
}

func (aI *AdapterInternal[T]) Add(obj T) error {
	err := aI.db.AddObject(append(aI.prefix, obj.GetIDByte()...), obj)
	if err != nil {
		return fmt.Errorf("not possible store object to database: %v", err)
	}

	return nil
}

func (aI *AdapterInternal[T]) Get(ID string) (T, error) {
	obj, err := aI.db.GetObjectByID(append(aI.prefix, []byte(ID)...))
	if err != nil {
		return obj, fmt.Errorf("not possible get object from database: %v", err)
	}

	return obj, nil
}

func convertUint16ToByte(input uint16) []byte {
	var outByte = make([]byte, 2)

	binary.LittleEndian.PutUint16(outByte, input)

	return outByte
}
