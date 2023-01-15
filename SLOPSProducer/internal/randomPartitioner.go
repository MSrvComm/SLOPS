package internal

import (
	"math/rand"
	"time"
)

type RandomPartitioner struct {
	generator *rand.Rand
}

func NewRandomPartitioner() *RandomPartitioner {
	return &RandomPartitioner{
		generator: rand.New(rand.NewSource(time.Now().UTC().UnixNano())),
	}
}

func (r *RandomPartitioner) Partition(numPartitions int32) int32 {
	return int32(r.generator.Intn(int(numPartitions)))
}