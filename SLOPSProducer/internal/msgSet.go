package internal

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
)

// This struct is used when moving a key from one partition to another.
// A key's `message set` is a set of consecutive messages for that key sent on the same partition.
// The `message set` index is incremented by 1 when switching partitions.
// This allows for forcing a total ordering on each stream (represented by a key)
// irrespective of changing partitions.
// To be repeated in consumer.
type MessageSet struct {
	Key             string
	SrcPartition    int32
	SrcMsgsetIndex  int32
	DestPartition   int32
	DestMsgsetIndex int32
}

func (m *MessageSet) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintln(&b,
		m.Key,
		m.SrcPartition,
		m.SrcMsgsetIndex,
		m.DestPartition,
		m.DestMsgsetIndex,
	)
	return b.Bytes(), nil
}

func (m *MessageSet) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &m.Key,
		&m.SrcPartition,
		&m.SrcMsgsetIndex,
		&m.DestPartition,
		&m.DestMsgsetIndex,
	)
	return err
}

type MessageSetMap struct {
	mu sync.RWMutex
	KV map[string]MessageSet
}

func (m *MessageSetMap) AddKey(rec MessageSet) *MessageSet {
	m.mu.Lock()
	defer m.mu.Unlock()

	if val, exist := m.KV[rec.Key]; exist {
		m.KV[rec.Key] = rec
		return &val
	}
	m.KV[rec.Key] = rec
	return nil
}

func (m *MessageSetMap) GetKey(key string) (*MessageSet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if val, ok := m.KV[key]; !ok {
		return nil, errors.New("no such value")
	} else {
		return &val, nil
	}
}

func (m *MessageSetMap) Del(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.KV, key)
}

func (m *MessageSetMap) Len() int {
	return len(m.KV)
}
