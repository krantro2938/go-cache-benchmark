package bench

import (
	"fmt"
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
	Operations  []Operation
	SharedValue []byte
}

type Operation struct {
	KeyID uint64
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

	// Optimization: Use a shared value buffer to avoid OOM with large datasets.
	sharedValue := make([]byte, cfg.ValueSize)
	rng.Read(sharedValue)

	for i := 0; i < cfg.TotalOps; i++ {
		keyID := zipf.Next() % uint64(cfg.KeySpaceSize)
		ops[i] = Operation{KeyID: keyID}
	}
	return &Workload{Operations: ops, SharedValue: sharedValue}
}

func GenerateKey(id uint64) string {
	return fmt.Sprintf("key_%d", id)
}
