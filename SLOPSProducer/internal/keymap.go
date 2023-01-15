package internal

import (
	"errors"
	"sync"
)

type KeyRecord struct {
	Key       string
	Count     int
	Partition int32
	New       bool // Is this just added? Send message to the old and new partitions about change.
	Delete    bool // Is this to be deleted? Send message to the old and new partitions about change.
}

type KeyMap struct {
	sync.Mutex
	KV map[string]KeyRecord
}

func (k *KeyMap) AddKey(rec KeyRecord) *KeyRecord {
	k.Lock()
	defer k.Unlock()

	if val, exist := k.KV[rec.Key]; exist {
		k.KV[rec.Key] = rec
		return &val
	}
	rec.Delete = false
	rec.New = true
	k.KV[rec.Key] = rec
	return nil
}

func (k *KeyMap) GetKey(key string) (*KeyRecord, error) {
	k.Lock()
	defer k.Unlock()

	if val, ok := k.KV[key]; !ok {
		return nil, errors.New("no such value")
	} else {
		return &val, nil
	}
}

// Mark the key for deletion, but don't delete.
func (k *KeyMap) MarkForDeletion(key string) {
	k.Lock()
	defer k.Unlock()
	if kv, exists := k.KV[key]; exists {
		kv.Delete = true
		k.KV[key] = kv
	}
}

func (k *KeyMap) Del(key string) {
	k.Lock()
	defer k.Unlock()

	delete(k.KV, key)
}

func (k *KeyMap) Len() int {
	return len(k.KV)
}
