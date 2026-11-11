package gcaware

import (
	"hash/maphash"
	"math"
	"math/rand"
	"sync"
)

// Config parameterises the cache. Defaults (see DefaultConfig) are applied for
// any zero-valued field.
type Config struct {
	// MaxBytes is the memory budget M_limit. Capacity is a BYTE limit
	// (sum of C(x) over resident objects), not an object count.
	MaxBytes uintptr
	// Lambda in [0,1] balances cache utility (frequency) against allocation
	// cost. Alpha = K*Lambda, Beta = K*(1-Lambda). Lambda is used as given
	// (0 is a valid value); start from DefaultConfig to get λ = 0.7.
	// Lambda = 1 disables the cost term (frequency-only Adaptive TinyLFU).
	Lambda float64
	// K scales the value function.
	K float64
	// SampleSize k_s: number of random resident entries inspected when
	// choosing an eviction victim.
	SampleSize int
	// CMSEpsilon / CMSDelta size the Count-Min Sketch (k_h = ceil(ln(1/delta))).
	CMSEpsilon float64
	CMSDelta   float64
	// CMSAgeEvery halves the sketch after this many increments
	// (0 = 10 * sketch width, the TinyLFU default).
	CMSAgeEvery uint64
	// AdaptInterval: AdaptWindow runs once every AdaptInterval operations
	// (not after every admission, to avoid oscillation).
	AdaptInterval uint64
	// Window bounds, expressed as a fraction of MaxBytes.
	WindowMinFrac  float64
	WindowMaxFrac  float64
	WindowInitFrac float64
	// Hit-ratio thresholds controlling window growth/shrink.
	ThresholdHigh float64
	ThresholdLow  float64
}

// DefaultConfig returns the parameters reported in the paper.
func DefaultConfig(maxBytes uintptr) Config {
	return Config{
		MaxBytes:       maxBytes,
		Lambda:         0.7,
		K:              1.0,
		SampleSize:     5,     // k_s
		CMSEpsilon:     0.001, // width ~ 2719 counters/row
		CMSDelta:       0.01,  // depth = 5 => k_h = 5
		CMSAgeEvery:    0,     // 0 = 10 * sketch width (TinyLFU default)
		AdaptInterval:  1000,
		WindowMinFrac:  0.01,
		WindowMaxFrac:  0.40,
		WindowInitFrac: 0.10,
		ThresholdHigh:  0.85,
		ThresholdLow:   0.70,
	}
}

type entry struct {
	key   string
	fp    uint64
	value []byte
	c     uintptr // C(x) = s(x) + m_gc(x)
	seq   uint64  // admission sequence number (for the window region)
}

// Loader fetches a value on a miss (the "FetchFromSource" of the paper).
type Loader func(key string) ([]byte, bool)

// Cache is a concurrency-safe, GC-aware local cache.
type Cache struct {
	mu sync.Mutex

	cfg   Config
	alpha float64
	beta  float64

	// Resident objects are stored by value in a dense slice (no per-object
	// heap allocation for metadata, fewer pointers for the GC to scan) and
	// located through a single index map. The dense slice also gives O(1)
	// uniform sampling of eviction candidates; removal is swap-with-last.
	entries []entry
	index   map[string]int32
	curByte uintptr

	cms       *CountMinSketch
	maxFreq   uint32 // running max for frequency normalisation
	cmsResets uint64 // sketch agings already reflected in maxFreq

	victimBuf []int32 // scratch space for two-stage admission (entry indices)

	seq         uint64
	ops         uint64
	windowBytes uintptr // current adaptive window size, in bytes

	hits      uint64
	misses    uint64
	evictions uint64

	hseed maphash.Seed
	rng   *rand.Rand
}

// New builds a cache from cfg, applying defaults for zero fields.
func New(cfg Config) *Cache {
	d := DefaultConfig(cfg.MaxBytes)
	if cfg.Lambda < 0 || cfg.Lambda > 1 {
		panic("gcaware: Lambda must be in [0, 1]")
	}
	if cfg.K == 0 {
		cfg.K = d.K
	}
	if cfg.SampleSize == 0 {
		cfg.SampleSize = d.SampleSize
	}
	if cfg.CMSEpsilon == 0 {
		cfg.CMSEpsilon = d.CMSEpsilon
	}
	if cfg.CMSDelta == 0 {
		cfg.CMSDelta = d.CMSDelta
	}
	if cfg.AdaptInterval == 0 {
		cfg.AdaptInterval = d.AdaptInterval
	}
	if cfg.WindowMaxFrac == 0 {
		cfg.WindowMinFrac = d.WindowMinFrac
		cfg.WindowMaxFrac = d.WindowMaxFrac
		cfg.WindowInitFrac = d.WindowInitFrac
	}
	if cfg.ThresholdHigh == 0 {
		cfg.ThresholdHigh = d.ThresholdHigh
		cfg.ThresholdLow = d.ThresholdLow
	}

	return &Cache{
		cfg:         cfg,
		alpha:       cfg.K * cfg.Lambda,
		beta:        cfg.K * (1 - cfg.Lambda),
		index:       make(map[string]int32),
		cms:         NewCountMinSketch(cfg.CMSEpsilon, cfg.CMSDelta, cfg.CMSAgeEvery),
		maxFreq:     1,
		windowBytes: uintptr(cfg.WindowInitFrac * float64(cfg.MaxBytes)),
		hseed:       maphash.MakeSeed(),
		rng:         rand.New(rand.NewSource(1)),
	}
}

// HashFuncs reports k_h (the Count-Min Sketch depth).
func (c *Cache) HashFuncs() int { return c.cms.Depth() }

// SampleSize reports k_s.
func (c *Cache) SampleSize() int { return c.cfg.SampleSize }

func (c *Cache) fingerprint(key string) uint64 {
	return maphash.String(c.hseed, key)
}

// normFreq returns f(x) in [0,1]: the estimated frequency divided by the
// largest frequency observed so far.
func (c *Cache) normFreq(fp uint64) float64 {
	// The sketch halves its counters periodically; halve the running maximum
	// accordingly so that normalised frequencies stay in [0, 1] and comparable.
	for c.cmsResets < c.cms.Resets() {
		c.maxFreq /= 2
		if c.maxFreq == 0 {
			c.maxFreq = 1
		}
		c.cmsResets++
	}
	f := c.cms.Estimate(fp)
	if f > c.maxFreq {
		c.maxFreq = f
	}
	if c.maxFreq == 0 {
		return 0
	}
	return float64(f) / float64(c.maxFreq)
}

// normCost returns the normalised cost C(x)/M_limit in [0,1].
func (c *Cache) normCost(cst uintptr) float64 {
	if c.cfg.MaxBytes == 0 {
		return 0
	}
	return float64(cst) / float64(c.cfg.MaxBytes)
}

// value computes V(x) = alpha*f(x) - beta*Ĉ(x) from normalised inputs.
func (c *Cache) value(fp uint64, cst uintptr) float64 {
	return c.alpha*c.normFreq(fp) - c.beta*c.normCost(cst)
}

// Get returns the value for key, fetching and admitting it on a miss
// (read-through usage). The Count-Min Sketch is updated on every access (hit
// and miss), so the frequency estimate reflects the full request stream.
func (c *Cache) Get(key string, load Loader) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if v, hit := c.access(key); hit {
		return v, true
	}
	var data []byte
	ok := true
	if load != nil {
		data, ok = load(key)
	}
	if ok {
		c.admit(key, c.fingerprint(key), data)
	}
	return data, ok
}

// Lookup records an access and returns the cached value WITHOUT loading or
// admitting on a miss. Pair it with Set for look-aside usage:
//
//	if v, ok := c.Lookup(key); ok { use(v) } else { c.Set(key, load(key)) }
func (c *Cache) Lookup(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.access(key)
}

// Set admits value under the GC-aware admission policy (look-aside usage).
// It does not itself record an access; call Lookup first as in a normal
// read path. The supplied value is stored as-is.
func (c *Cache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.admit(key, c.fingerprint(key), value)
}

// access records one access (CMS increment, hit/miss accounting, window
// adaptation) and returns the cached value if present. Callers must hold c.mu.
func (c *Cache) access(key string) ([]byte, bool) {
	fp := c.fingerprint(key)
	c.cms.Increment(fp)
	c.ops++
	if i, ok := c.index[key]; ok {
		c.hits++
		c.maybeAdapt()
		return c.entries[i].value, true
	}
	c.misses++
	c.maybeAdapt()
	return nil, false
}

// admit applies the selective admission policy from the paper (Algorithm 1).
//
// If the new object does not fit, admission is two-stage: first a set E of
// victims whose total cost covers the deficit is selected, and every victim
// is checked against the new object (V(x_new) > V(x_v)); only if all checks
// pass are the victims removed and the new object inserted. If any candidate
// is at least as valuable as the new object, the insertion is rejected and the
// cache is left unchanged.
func (c *Cache) admit(key string, fp uint64, data []byte) {
	s := uintptr(len(data))
	cNew := cost(s)
	if cNew > c.cfg.MaxBytes {
		return // object alone exceeds the budget; never admit
	}

	// Updating a resident key replaces its value in place.
	if i, ok := c.index[key]; ok {
		c.removeAt(i)
	}

	// If there is room, insert directly.
	if c.curByteAfterInsert(cNew) {
		c.insert(key, fp, data, cNew)
		return
	}

	// Stage 1: select and check victims without modifying the cache.
	need := c.curByte + cNew - c.cfg.MaxBytes
	vNew := c.value(fp, cNew)
	chosen := c.victimBuf[:0] // reused buffer: no allocation per admission
	var freed uintptr
	for freed < need {
		vi, vVal := c.selectVictim(chosen)
		if vi < 0 {
			c.victimBuf = chosen[:0]
			return // nothing left to evict; cache unchanged
		}
		if vNew <= vVal {
			c.victimBuf = chosen[:0]
			return // rejected; cache unchanged
		}
		chosen = append(chosen, vi)
		freed += c.entries[vi].c
	}
	c.victimBuf = chosen[:0]

	// Stage 2: commit. Removal by swap-with-last moves entries, so remove
	// in descending index order: a later (smaller) index is never moved.
	sortDesc(chosen)
	for _, vi := range chosen {
		c.removeAt(vi)
		c.evictions++
	}
	c.insert(key, fp, data, cNew)
}

func (c *Cache) curByteAfterInsert(cNew uintptr) bool {
	return c.curByte+cNew <= c.cfg.MaxBytes
}

// selectVictim samples k_s resident entries outside `exclude` and returns the
// index and value V of the one with the lowest V, skipping objects inside the
// protected window. If every sampled entry is protected, the lowest-valued
// sampled entry is returned so that the cache can still make progress.
// It returns -1 if no candidate is available.
func (c *Cache) selectVictim(exclude []int32) (int32, float64) {
	n := len(c.entries)
	if n == 0 || len(exclude) >= n {
		return -1, 0
	}
	windowEdge := c.windowEdge()
	victim, fallback := int32(-1), int32(-1)
	var vMin, fMin float64
	for i := 0; i < c.cfg.SampleSize; i++ {
		idx := int32(c.rng.Intn(n))
		if containsIdx(exclude, idx) {
			continue
		}
		e := &c.entries[idx]
		v := c.value(e.fp, e.c)
		if fallback < 0 || v < fMin {
			fallback, fMin = idx, v
		}
		// Protect recently admitted objects (the adaptive window).
		if e.seq > windowEdge {
			continue
		}
		if victim < 0 || v < vMin {
			victim, vMin = idx, v
		}
	}
	if victim >= 0 {
		return victim, vMin
	}
	if fallback >= 0 {
		return fallback, fMin
	}
	// All samples hit excluded entries: take any non-excluded one.
	for idx := int32(0); idx < int32(n); idx++ {
		if !containsIdx(exclude, idx) {
			e := &c.entries[idx]
			return idx, c.value(e.fp, e.c)
		}
	}
	return -1, 0
}

func containsIdx(set []int32, x int32) bool {
	for _, v := range set {
		if v == x {
			return true
		}
	}
	return false
}

// sortDesc sorts a short slice of indices in descending order (insertion sort;
// the victim set has a handful of elements).
func sortDesc(a []int32) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] > a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// windowEdge returns the largest admission sequence number that is NOT
// protected: the most recently admitted objects, whose share of the resident
// objects equals windowBytes/curByte, have seq > windowEdge.
func (c *Cache) windowEdge() uint64 {
	if c.curByte == 0 {
		return c.seq
	}
	frac := float64(c.windowBytes) / float64(c.curByte)
	if frac > 1 {
		frac = 1
	}
	protected := uint64(float64(len(c.entries)) * frac)
	if protected >= c.seq {
		return 0
	}
	return c.seq - protected
}

func (c *Cache) insert(key string, fp uint64, data []byte, cNew uintptr) {
	c.seq++
	c.index[key] = int32(len(c.entries))
	c.entries = append(c.entries, entry{key: key, fp: fp, value: data, c: cNew, seq: c.seq})
	c.curByte += cNew
}

// removeAt deletes the entry at index i in O(1) by moving the last entry
// into its slot.
func (c *Cache) removeAt(i int32) {
	e := &c.entries[i]
	c.curByte -= e.c
	delete(c.index, e.key)
	last := int32(len(c.entries) - 1)
	if i != last {
		c.entries[i] = c.entries[last]
		c.index[c.entries[i].key] = i
	}
	c.entries[last] = entry{} // drop references so the GC can free the value
	c.entries = c.entries[:last]
}

// maybeAdapt runs AdaptWindow on the configured interval.
func (c *Cache) maybeAdapt() {
	if c.cfg.AdaptInterval == 0 || c.ops%c.cfg.AdaptInterval != 0 {
		return
	}
	c.adaptWindow(c.HitRatio())
}

// adaptWindow grows or shrinks the protected window based on the hit ratio,
// clamped to [WindowMinFrac, WindowMaxFrac] of the budget.
func (c *Cache) adaptWindow(hr float64) {
	switch {
	case hr > c.cfg.ThresholdHigh:
		c.windowBytes = uintptr(float64(c.windowBytes) * 0.95)
	case hr < c.cfg.ThresholdLow:
		c.windowBytes = uintptr(float64(c.windowBytes) * 1.05)
	}
	min := uintptr(c.cfg.WindowMinFrac * float64(c.cfg.MaxBytes))
	max := uintptr(c.cfg.WindowMaxFrac * float64(c.cfg.MaxBytes))
	if c.windowBytes < min {
		c.windowBytes = min
	}
	if c.windowBytes > max {
		c.windowBytes = max
	}
}

// HitRatio is hits / (hits + misses).
func (c *Cache) HitRatio() float64 {
	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

// Stats is a snapshot of cache counters.
type Stats struct {
	Hits, Misses uint64
	Evictions    uint64
	HitRatio     float64
	Entries      int
	Bytes        uintptr
	WindowBytes  uintptr
	HashFuncs    int // k_h
	SampleSize   int // k_s
	CMSWidth     int
}

// Stats returns a snapshot under the lock.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Hits:        c.hits,
		Misses:      c.misses,
		Evictions:   c.evictions,
		HitRatio:    c.HitRatio(),
		Entries:     len(c.entries),
		Bytes:       c.curByte,
		WindowBytes: c.windowBytes,
		HashFuncs:   c.cms.Depth(),
		SampleSize:  c.cfg.SampleSize,
		CMSWidth:    c.cms.Width(),
	}
}

// ensure math is referenced even if future edits drop its only use.
var _ = math.MaxUint16
