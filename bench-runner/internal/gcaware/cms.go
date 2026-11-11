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
	adds    uint64 // total increments since last aging
	ageEvry uint64 // halve counters after this many increments
	resets  uint64 // number of agings performed so far
}

// NewCountMinSketch builds a sketch sized for the given error bounds.
// epsilon is the relative additive error and delta the failure probability.
// ageEvery is the number of increments between agings; 0 selects the TinyLFU
// default of 10 increments per counter of one row (10 * width).
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
	if ageEvery == 0 {
		ageEvery = 10 * uint64(width)
	}
	return &CountMinSketch{
		depth:   depth,
		width:   width,
		rows:    rows,
		ageEvry: ageEvery,
	}
}

// Depth reports d (the number of hash functions, k_h in the paper).
func (c *CountMinSketch) Depth() int { return c.depth }

// Width reports w (counters per row).
func (c *CountMinSketch) Width() int { return c.width }

// mix is a 64-bit finaliser (MurmurHash3 fmix64). Row positions are derived
// from one mixed fingerprint by double hashing, g_i(x) = h1 + i*h2 mod w
// (Kirsch and Mitzenmacher), which keeps the Count-Min guarantees while
// hashing the key only once per operation.
func mix(h uint64) uint64 {
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

// Increment records one access to the key fingerprint and ages the sketch
// when the increment budget is exhausted.
func (c *CountMinSketch) Increment(fp uint64) {
	h := mix(fp)
	h1, h2 := h&0xffffffff, (h>>32)|1
	w := uint64(c.width)
	for i := 0; i < c.depth; i++ {
		j := (h1 + uint64(i)*h2) % w
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
	h := mix(fp)
	h1, h2 := h&0xffffffff, (h>>32)|1
	w := uint64(c.width)
	for i := 0; i < c.depth; i++ {
		j := (h1 + uint64(i)*h2) % w
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
	c.resets++
}

// Resets reports how many times the sketch has been aged.
func (c *CountMinSketch) Resets() uint64 { return c.resets }

// AgeEvery reports the aging period in increments.
func (c *CountMinSketch) AgeEvery() uint64 { return c.ageEvry }
