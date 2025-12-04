package util

import (
	"math"
	"math/rand"
)

type ZipfGenerator struct {
	rng   *rand.Rand
	n     float64
	s     float64
	alpha float64
	c     float64
}

func NewZipfGenerator(rng *rand.Rand, n, s float64) *ZipfGenerator {
	if s == 0.0 {
		// Uniform distribution
		return &ZipfGenerator{rng: rng, n: n, s: s}
	}
	alpha := 1.0 / (1.0 - s)
	c := float64(0)
	for i := 1; i <= int(n); i++ {
		c += 1.0 / math.Pow(float64(i), s)
	}
	return &ZipfGenerator{rng: rng, n: n, s: s, alpha: alpha, c: c}
}

func (z *ZipfGenerator) Next() uint64 {
	if z.s == 0.0 {
		// Uniform: return 1 to n
		return uint64(z.rng.Intn(int(z.n))) + 1
	}
	eta := z.rng.Float64()
	xi := math.Pow(eta*z.c, -z.alpha)
	return uint64(math.Ceil(xi))
}
