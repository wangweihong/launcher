package bolt

import (
	. "ufleet/launcher/code/model/base/interf"

	"bytes"
	"errors"
	"github.com/coreos/bbolt"
	"ufleet/launcher/code/config"
	"fmt"
)

type Bolt struct {
	Name   string
	Bucket string
	db     *bolt.DB
}

func (b *Bolt) Put(key string, value string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(b.Bucket))
		if err != nil {
			return err
		}
		err = b.Put([]byte(key), []byte(value))
		return err
	})
}

func (b *Bolt) Get(key string) (map[string]string, error) {
	var value []byte
	err := b.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(b.Bucket))
		if err != nil {
			return err
		}
		v := b.Get([]byte(key))
		if v == nil {
			return errors.New("key not exist")
		}
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})

	result := make(map[string]string)

	if err != nil {
		result[key] = string(value)
	}

	tList := b.list(key)
	for key := range tList {
		result[key] = string(tList[key])
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("key not exist")
	}

	return result, nil
}

func (b *Bolt) list(prefix string) map[string][]byte {
	result := make(map[string][]byte, 0)
	b.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		bucket := tx.Bucket([]byte(b.Bucket))
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()

		for k, v := c.Seek([]byte(prefix)); k != nil && bytes.HasPrefix(k, []byte(prefix)); k, v = c.Next() {
			result[string(k)] = v
		}

		return nil
	})
	return result
}

func (b *Bolt) Delete(key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(b.Bucket))
		if err != nil {
			return err
		}
		return b.Delete([]byte(key))
	})
}

func (b *Bolt) Close() {
	b.db.Close()
}

func (b *Bolt) New(args ...string) (IDB, error) {
	s := &Bolt{
		Name:   config.GDefault.StorePath,
		Bucket: "default",
	}
	db, err := bolt.Open(s.Name, 0600, nil)
	if err != nil {
		return nil, err
	}
	s.db = db
	return s, nil
}