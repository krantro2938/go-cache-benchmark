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

type WorkloadConfig struct {
	Seed         int64
	KeySpaceSize int
	TotalOps     int
	ValueSize    int
	Skew         float64
}

func GenerateWorkload(cfg WorkloadConfig) *Workload {
	rng := rand.New(rand.NewSource(cfg.Seed))
	zipf := util.NewZipfGenerator(rng, float64(cfg.KeySpaceSize), cfg.Skew)
	ops := make([]Operation, cfg.TotalOps)

	for i := 0; i < cfg.TotalOps; i++ {
		keyID := zipf.Next() % uint64(cfg.KeySpaceSize)
		key := generateKey(keyID)
		value := make([]byte, cfg.ValueSize)
		rng.Read(value)
		ops[i] = Operation{Key: key, Value: value}
	}
	return &Workload{Operations: ops}
}

func generateKey(id uint64) string {
	return "key_" + string('a'+byte(id%26)) + "_" + string('0'+byte(id%10))
}
