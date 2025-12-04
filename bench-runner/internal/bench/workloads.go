package bench

import (
	"math/rand"

	"cache-bench/internal/util"
)

const (
	KeySpaceSize = 1_000_000
	TotalOps     = 2_000_000
	KeySize      = 16
	ValueSize    = 1028
)

type Workload struct {
	Operations []Operation
}

type Operation struct {
	Key   string
	Value []byte
}

func GenerateZipfWorkload(seed int64) *Workload {
	rng := rand.New(rand.NewSource(seed))
	zipf := util.NewZipfGenerator(rng, KeySpaceSize, 0.99)
	ops := make([]Operation, TotalOps)

	for i := 0; i < TotalOps; i++ {
		keyID := zipf.Next() % KeySpaceSize
		key := generateKey(keyID)
		value := make([]byte, ValueSize)
		rng.Read(value)
		ops[i] = Operation{Key: key, Value: value}
	}
	return &Workload{Operations: ops}
}

func generateKey(id uint64) string {
	return "key_" + string(rune('a'+id%26)) + "_" + string(rune('0'+id%10))
}
