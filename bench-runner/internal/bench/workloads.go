package bench

import (
	"math/rand"
	"strconv"

	"cache-bench/internal/util"
)

// Workload is a pre-generated request stream. Each key has a fixed object
// size; on a miss the benchmark "loads" the object by allocating a fresh
// buffer of that size, as a real service would after fetching from a source.
type Workload struct {
	Keys     []string // key per operation (pre-built so key formatting is not measured)
	Sizes    []int    // object size per operation (size of the key's object)
	Source   []byte   // source bytes copied into each loaded object
	MaxSize  int
	MeanSize float64 // mean object size over the key space
}

// WorkloadConfig describes a request stream.
type WorkloadConfig struct {
	Seed         int64
	KeySpaceSize int
	TotalOps     int
	Skew         float64 // Zipf exponent s; 0 = uniform
	// Either a single ValueSize, or a weighted mix of sizes (SizeMix with
	// SizeWeights, percentages). Each key gets one fixed size.
	ValueSize   int
	SizeMix     []int
	SizeWeights []int
}

// GenerateWorkload builds the request stream deterministically from Seed.
func GenerateWorkload(cfg WorkloadConfig) *Workload {
	rng := rand.New(rand.NewSource(cfg.Seed))

	// Per-key object sizes.
	keySize := make([]int, cfg.KeySpaceSize)
	maxSize := cfg.ValueSize
	if len(cfg.SizeMix) == 0 {
		for k := range keySize {
			keySize[k] = cfg.ValueSize
		}
	} else {
		total := 0
		for _, w := range cfg.SizeWeights {
			total += w
		}
		maxSize = 0
		for _, s := range cfg.SizeMix {
			if s > maxSize {
				maxSize = s
			}
		}
		for k := range keySize {
			r := rng.Intn(total)
			for i, w := range cfg.SizeWeights {
				if r < w {
					keySize[k] = cfg.SizeMix[i]
					break
				}
				r -= w
			}
		}
	}
	var sum float64
	for _, s := range keySize {
		sum += float64(s)
	}

	// Popularity ranks are mapped to keys through a random permutation so
	// that popular keys are not clustered by id (and, for mixed sizes, so
	// that popularity is independent of size).
	perm := rng.Perm(cfg.KeySpaceSize)
	zipf := util.NewZipfGenerator(rng, float64(cfg.KeySpaceSize), cfg.Skew)

	keys := make([]string, cfg.TotalOps)
	sizes := make([]int, cfg.TotalOps)
	names := make(map[int]string)
	for i := 0; i < cfg.TotalOps; i++ {
		id := perm[zipf.Next()]
		name, ok := names[id]
		if !ok {
			name = GenerateKey(uint64(id))
			names[id] = name
		}
		keys[i] = name
		sizes[i] = keySize[id]
	}

	src := make([]byte, maxSize)
	rng.Read(src)
	return &Workload{
		Keys:     keys,
		Sizes:    sizes,
		Source:   src,
		MaxSize:  maxSize,
		MeanSize: sum / float64(cfg.KeySpaceSize),
	}
}

// GenerateKey formats a key id.
func GenerateKey(id uint64) string {
	return "key_" + strconv.FormatUint(id, 10)
}
