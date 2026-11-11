package util

import (
	"math"
	"math/rand"
	"sort"
)

// ZipfGenerator draws ranks 0..n-1 from a (truncated) Zipf distribution with
// exponent s >= 0: P(rank k) is proportional to 1/(k+1)^s. s = 0 gives the
// uniform distribution.
//
// The standard library's rand.Zipf only supports s > 1, while the workloads in
// the paper use s in [0.8, 0.99]. We therefore sample by exact inversion of
// the precomputed cumulative distribution (binary search over n doubles).
// This is exact for any s >= 0; the table costs 8*n bytes and is built once
// per workload, outside the measured loop.
type ZipfGenerator struct {
	rng *rand.Rand
	n   int
	s   float64
	cdf []float64 // cdf[k] = P(rank <= k); nil for the uniform case
}

// NewZipfGenerator builds a generator over n ranks with exponent s.
func NewZipfGenerator(rng *rand.Rand, n float64, s float64) *ZipfGenerator {
	z := &ZipfGenerator{rng: rng, n: int(n), s: s}
	if s == 0 {
		return z
	}
	z.cdf = make([]float64, z.n)
	sum := 0.0
	for k := 0; k < z.n; k++ {
		sum += 1.0 / math.Pow(float64(k+1), s)
		z.cdf[k] = sum
	}
	for k := range z.cdf {
		z.cdf[k] /= sum
	}
	z.cdf[z.n-1] = 1.0
	return z
}

// Next returns a rank in [0, n). Rank 0 is the most popular key.
func (z *ZipfGenerator) Next() uint64 {
	if z.cdf == nil {
		return uint64(z.rng.Intn(z.n))
	}
	u := z.rng.Float64()
	return uint64(sort.SearchFloat64s(z.cdf, u))
}

// ProbTop returns the theoretical probability of rank 0 (for self-checks).
func (z *ZipfGenerator) ProbTop() float64 {
	if z.cdf == nil {
		return 1.0 / float64(z.n)
	}
	return z.cdf[0]
}
