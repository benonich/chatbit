package store

import (
	"chatbit/internal/domain"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type AdapterInternal[T ObjectInternal] struct {
	db     Database[T]
	prefix []byte
	mutex  sync.RWMutex
}

func NewAdapterInternal[T ObjectInternal](db Database[T]) *AdapterInternal[T] {
	var obj T

	var prefix []byte

	switch any(obj).(type) {
	case domain.Message:
		prefix = convertUint16ToByte(PrefixRoom)
	case domain.Room:
		prefix = convertUint16ToByte(PrefixMessage)
	case domain.Push:
		prefix = convertUint16ToByte(PrefixPush)
	case domain.Peer:
		prefix = convertUint16ToByte(PrefixPeer)
	}

	return &AdapterInternal[T]{
		db:     db,
		prefix: prefix,
		mutex:  sync.RWMutex{},
	}
}

func (aI *AdapterInternal[T]) Delete(ID string) error {
	aI.mutex.Lock()
	defer aI.mutex.Unlock()

	return aI.delete(ID)
}

func (aI *AdapterInternal[T]) Add(obj T) error {
	aI.mutex.Lock()
	defer aI.mutex.Unlock()

	return aI.add(obj)
}

func (aI *AdapterInternal[T]) Get(ID string) (T, error) {
	aI.mutex.RLock()
	defer aI.mutex.RUnlock()

	return aI.get(ID)
}

func (aI *AdapterInternal[T]) Update(ID string, fn func(obj T) error) error {
	aI.mutex.Lock()
	defer aI.mutex.Unlock()

	obj, err := aI.get(ID)
	if err != nil {
		return err
	}

	err = fn(obj)
	if err != nil {
		return err
	}

	if &obj == nil {
		return aI.delete(ID)
	}

	return aI.add(obj)
}

func (aI *AdapterInternal[T]) get(ID string) (T, error) {
	parsedUUID, err := uuid.Parse(ID)
	if err != nil {
		var obj T
		return obj, fmt.Errorf("not possible to parse uuid: %w", err)
	}

	obj, err := aI.db.GetObjectByID(append(aI.prefix, parsedUUID[:]...))
	if err != nil {
		return obj, fmt.Errorf("not possible get object from database: %w", err)
	}

	return obj, nil
}

func (aI *AdapterInternal[T]) add(obj T) error {
	err := aI.db.AddObject(append(aI.prefix, obj.GetIDByte()...), obj)
	if err != nil {
		return fmt.Errorf("not possible store object to database: %w", err)
	}

	return nil
}

func (aI *AdapterInternal[T]) AddWithTTL(obj T, ttl int64) error {
	err := aI.db.AddObjectWithTTL(append(aI.prefix, obj.GetIDByte()...), obj, time.Duration(ttl)*time.Second)
	if err != nil {
		return fmt.Errorf("not possible store object to database: %w", err)
	}

	return nil
}

func (aI *AdapterInternal[T]) delete(ID string) error {
	parsedUUID, err := uuid.Parse(ID)
	if err != nil {
		return fmt.Errorf("not possible to parse uuid: %v", err)
	}
	err = aI.db.DeleteObjectByID(append(aI.prefix, parsedUUID[:]...))
	if err != nil {
		return fmt.Errorf("not possible to delete object from database: %w", err)
	}

	return nil
}

func convertUint16ToByte(input uint16) []byte {
	var outByte = make([]byte, 2)

	binary.LittleEndian.PutUint16(outByte, input)

	return outByte
}
