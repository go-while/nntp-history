package history

import (
	"fmt"
	//"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
)

func boltCreateBucket(db *bolt.DB, char *string, bucket *string) (retbool bool, err error) {
	if char == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=nil")
	}
	if db == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s db=nil", *char)
	}
	if bucket == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s bucket=nil", *char)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		//_, err := tx.CreateBucketIfNotExists([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		if err != nil {
			return err
		}
		logf(DEBUG2, "OK boltCreateBucket char=%s bucket='%s'", *char, *bucket)
		retbool = true
		return nil
	}); err != nil {
		if err != bolt.ErrBucketExists {
			log.Printf("ERROR boltCreateBucket char=%s bucket='%s' err='%v'", *char, *bucket, err)
		}
		return false, err
	}
	return
} // end func boltCreateBucket

func boltBucketPutString(db *bolt.DB, char *string, bucket *string, key *string, val *string) (err error) {
	if char == nil {
		return fmt.Errorf("ERROR boltBucketPut char=nil")
	}
	if db == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s db=nil", *char)
	}
	if bucket == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s bucket=nil", *char)
	}
	if key == nil {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] key=nil", *char, *bucket)
	}
	if val == nil {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] val=nil", *char, *bucket)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		err := b.Put([]byte(*key), []byte(*val))
		return err
	}); err != nil {
		log.Printf("ERROR boltBucketPut [%s|%s] key=%s val=%s err='%v'", *char, *bucket, *key, *val, err)
		return err
	}
	return
} // end func boltBucketPutString

func boltBucketGetBytes(db *bolt.DB, char *string, bucket *string, key *string) (retval *[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltBucketGet char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s bucket=nil", *char)
	}
	if key == nil || *key == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet [%s|%s] key=nil", *char, *bucket)
	}
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			log.Printf("NOTFOUND boltBucketGet [%s|%s] key=%s", *char, *bucket, *key)
			return nil
		}
		//log.Printf("GOT boltBucketGet [%s|%s] key=%s val='%s'", *char, *bucket, *key, string(v))
		retval = &v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGet [%s|%s] key=%s err='%v'", *char, *bucket, *key, err)
		return nil, err
	}
	return
} // end func boltBucketGetBytes

func boltGetAllKeysVals(db *bolt.DB, char *string, bucket *string) (keyvals map[*string]*[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s bucket=nil", char)
	}
	keyvals = make(map[*string]*[]byte)
	if err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(*bucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			key := string(k)
			keyvals[&key] = &v
			//*keys = append(*keys, &key)
			//*vals = append(*vals, &v)
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return
} // end func boltGetAllKeysVals

func boltGetAllKeys(db *bolt.DB, char *string, bucket *string) (retkeys *[]*string, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=%s bucket=nil", char)
	}
	var keys []*string
	if err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(*bucket))
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			//fmt.Printf("key=%s", k)
			key := string(k)
			keys = append(keys, &key)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	retkeys = &keys
	return
} // end func boltGetAllKeys
