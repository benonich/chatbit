package store

import (
	"encoding/binary"
	"fmt"
)

type AdapterInternal[T ObjectMessageInternal] struct {
	db     Database[T]
	prefix []byte
}

func NewAdapterInternal[T ObjectMessageInternal](db Database[T], prefix uint16) *AdapterInternal[T] {
	return &AdapterInternal[T]{
		db:     db,
		prefix: convertUint16ToByte(prefix),
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
	err := aI.db.AddObject(append(aI.prefix, []byte(obj.GetID())...), obj)
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
