package internal

import (
	"errors"
	"sync"
)

type KeyRecord struct {
	Key       string
	Count     int
	Partition int
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

func (k *KeyMap) Del(key string) {
	k.Lock()
	defer k.Unlock()

	delete(k.KV, key)
}

func (k *KeyMap) Len() int {
	return len(k.KV)
}