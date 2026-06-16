package gcaware

import "math"

// CountMinSketch is a probabilistic frequency estimator used by the admission
// policy. It uses d = depth hash functions over rows of width w; an estimate
// is the minimum count across the d rows, which bounds over-estimation.
//
// Dimensions follow the standard Count-Min guarantees:
//
//	w = ceil(e / epsilon)      (additive error <= epsilon * N)
//	d = ceil(ln(1 / delta))    (held with probability >= 1 - delta)
//
// A periodic halving ("aging") of all counters keeps the sketch responsive to
// shifts in the access distribution, as in the TinyLFU family.
type CountMinSketch struct {
	depth   int // d = number of hash functions (k_h)
	width   int // w = counters per row
	rows    [][]uint16
	seeds   []uint32
	adds    uint64 // total increments since last aging
	ageEvry uint64 // halve counters after this many increments
}

// NewCountMinSketch builds a sketch sized for the given error bounds.
// epsilon is the relative additive error and delta the failure probability.
func NewCountMinSketch(epsilon, delta float64, ageEvery uint64) *CountMinSketch {
	width := int(math.Ceil(math.E / epsilon))
	depth := int(math.Ceil(math.Log(1.0 / delta)))
	if width < 1 {
		width = 1
	}
	if depth < 1 {
		depth = 1
	}
	rows := make([][]uint16, depth)
	for i := range rows {
		rows[i] = make([]uint16, width)
	}
	seeds := make([]uint32, depth)
	for i := range seeds {
		// Distinct odd seeds for independent hash functions.
		seeds[i] = uint32(i)*0x9E3779B1 + 1
	}
	return &CountMinSketch{
		depth:   depth,
		width:   width,
		rows:    rows,
		seeds:   seeds,
		ageEvry: ageEvery,
	}
}

// Depth reports d (the number of hash functions, k_h in the paper).
func (c *CountMinSketch) Depth() int { return c.depth }

// Width reports w (counters per row).
func (c *CountMinSketch) Width() int { return c.width }

// hash mixes a 64-bit key fingerprint with a per-row seed (FNV-style mix).
func (c *CountMinSketch) hash(fp uint64, seed uint32) int {
	h := fp ^ uint64(seed)
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return int(h % uint64(c.width))
}

// Increment records one access to the key fingerprint and ages the sketch
// when the increment budget is exhausted.
func (c *CountMinSketch) Increment(fp uint64) {
	for i := 0; i < c.depth; i++ {
		j := c.hash(fp, c.seeds[i])
		if c.rows[i][j] < math.MaxUint16 {
			c.rows[i][j]++
		}
	}
	c.adds++
	if c.ageEvry > 0 && c.adds >= c.ageEvry {
		c.age()
	}
}

// Estimate returns the conservative (minimum) frequency for the fingerprint.
func (c *CountMinSketch) Estimate(fp uint64) uint32 {
	min := uint32(math.MaxUint32)
	for i := 0; i < c.depth; i++ {
		j := c.hash(fp, c.seeds[i])
		if v := uint32(c.rows[i][j]); v < min {
			min = v
		}
	}
	return min
}

// age halves every counter, decaying stale popularity.
func (c *CountMinSketch) age() {
	for i := range c.rows {
		row := c.rows[i]
		for j := range row {
			row[j] >>= 1
		}
	}
	c.adds = 0
}
