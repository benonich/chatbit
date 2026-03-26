package database

import (
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

func (a *Adapter[T]) DeleteObjectByID(ID []byte) error {
	err := a.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(ID)
	})
	if err != nil {
		return fmt.Errorf("not possible to delete object from database: %w", err)
	}

	return nil
}

func (a *Adapter[T]) DeleteObjectsByPrefix(prefix []byte) error {
	err := a.db.Update(func(txn *badger.Txn) error {
		itr := txn.NewIterator(badger.IteratorOptions{
			Prefix: prefix,
		})
		defer itr.Close()

		for itr.Rewind(); itr.Valid(); itr.Next() {
			item := itr.Item()
			key := item.KeyCopy(nil)
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("not possible to delete objects by prefix from database: %w", err)
	}

	return nil
}

func (a *Adapter[T]) AddObject(ID []byte, obj T) error {
	objB, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("not possible to convert go object to byte")
	}

	err = a.db.Update(func(txn *badger.Txn) error {
		return txn.Set(ID, objB)
	})
	if err != nil {
		return fmt.Errorf("not possible to store object to database")
	}

	return nil
}

func (a *Adapter[T]) GetObjectByID(ID []byte) (T, error) {
	var err error

	var objCopy []byte

	var objItem *badger.Item

	var obj T

	err = a.db.View(func(txn *badger.Txn) error {
		objItem, err = txn.Get(ID)
		if err != nil {
			return err
		}

		//Important to copy
		objCopy, err = objItem.ValueCopy(nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return obj, fmt.Errorf("not possible to get object by ID from the database")
	}

	err = json.Unmarshal(objCopy, obj)
	if err != nil {
		return obj, fmt.Errorf("not possible to convert byte  to go object")
	}

	return obj, nil
}

func (a *Adapter[T]) GetObjectsByPrefixAndSinceTs(prefix []byte, timestamp uint64) ([]T, error) {
	var objects []T

	err := a.db.View(func(txn *badger.Txn) error {
		itr := txn.NewIterator(badger.IteratorOptions{
			Prefix:  prefix,
			SinceTs: timestamp,
		})

		defer itr.Close()

		for itr.Rewind(); itr.Valid(); itr.Next() {
			item := itr.Item()

			//Important to copy
			objCopyBin, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			var obj T

			err = json.Unmarshal(objCopyBin, &obj)
			if err != nil {
				return fmt.Errorf("failed to unmarshal object: %w", err)
			}

			objects = append(objects, obj)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return objects, nil
}
