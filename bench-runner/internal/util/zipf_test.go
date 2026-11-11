package util

import (
	"math"
	"math/rand"
	"testing"
)

// The empirical frequency of the top ranks must match the theoretical Zipf
// probabilities (the previous generator sent ~95% of requests to one key).
func TestZipfMatchesTheory(t *testing.T) {
	const n = 100_000
	const draws = 2_000_000
	for _, s := range []float64{0.8, 0.9, 0.99} {
		z := NewZipfGenerator(rand.New(rand.NewSource(1)), n, s)
		counts := make([]int, n)
		for i := 0; i < draws; i++ {
			counts[z.Next()]++
		}
		h := 0.0
		for k := 1; k <= n; k++ {
			h += 1 / math.Pow(float64(k), s)
		}
		for k := 0; k < 3; k++ {
			want := 1 / math.Pow(float64(k+1), s) / h
			got := float64(counts[k]) / draws
			if math.Abs(got-want)/want > 0.05 {
				t.Errorf("s=%.2f rank %d: got %.4f want %.4f", s, k, got, want)
			}
		}
	}
}

func TestUniformRange(t *testing.T) {
	z := NewZipfGenerator(rand.New(rand.NewSource(1)), 10, 0)
	seen := map[uint64]bool{}
	for i := 0; i < 1000; i++ {
		v := z.Next()
		if v >= 10 {
			t.Fatalf("out of range: %d", v)
		}
		seen[v] = true
	}
	if len(seen) != 10 {
		t.Fatalf("uniform generator covered %d of 10 values", len(seen))
	}
}
