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
	// cost. Alpha = K*Lambda, Beta = K*(1-Lambda).
	Lambda float64
	// K scales the value function.
	K float64
	// SampleSize k_s: number of random resident entries inspected when
	// choosing an eviction victim.
	SampleSize int
	// CMSEpsilon / CMSDelta size the Count-Min Sketch (k_h = ceil(ln(1/delta))).
	CMSEpsilon float64
	CMSDelta   float64
	// CMSAgeEvery halves the sketch after this many increments.
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
		CMSAgeEvery:    0,     // set from workload size by the caller if desired
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

	data    map[string]*entry
	keys    []string // parallel slice for O(1) random sampling
	keyIdx  map[string]int
	curByte uintptr

	cms     *CountMinSketch
	maxFreq uint32 // running max for frequency normalisation

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
	if cfg.Lambda == 0 {
		cfg.Lambda = d.Lambda
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
		data:        make(map[string]*entry),
		keyIdx:      make(map[string]int),
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
	var h maphash.Hash
	h.SetSeed(c.hseed)
	_, _ = h.WriteString(key)
	return h.Sum64()
}

// normFreq returns f(x) in [0,1]: the estimated frequency divided by the
// largest frequency observed so far.
func (c *Cache) normFreq(fp uint64) float64 {
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
	if e, ok := c.data[key]; ok {
		c.hits++
		c.maybeAdapt()
		return e.value, true
	}
	c.misses++
	c.maybeAdapt()
	return nil, false
}

// admit applies the selective admission policy from the paper.
func (c *Cache) admit(key string, fp uint64, data []byte) {
	s := uintptr(len(data))
	cNew := cost(s)
	if cNew > c.cfg.MaxBytes {
		return // object alone exceeds the budget; never admit
	}

	// If there is room, insert directly.
	if c.curByteAfterInsert(cNew) {
		c.insert(key, fp, data, cNew)
		return
	}

	// Otherwise compare against a sampled victim and evict to make room.
	vNew := c.value(fp, cNew)
	for c.curByte+cNew > c.cfg.MaxBytes {
		victim := c.selectVictim()
		if victim == nil {
			return
		}
		vVictim := c.value(victim.fp, victim.c)
		if vNew <= vVictim {
			return // incoming object is not worth more than the victim
		}
		c.remove(victim.key)
		c.evictions++
	}
	c.insert(key, fp, data, cNew)
}

func (c *Cache) curByteAfterInsert(cNew uintptr) bool {
	return c.curByte+cNew <= c.cfg.MaxBytes
}

// selectVictim samples k_s resident entries and returns the one with the
// lowest value V, skipping objects still inside the protected window region.
func (c *Cache) selectVictim() *entry {
	n := len(c.keys)
	if n == 0 {
		return nil
	}
	windowEdge := c.windowEdge()
	var victim *entry
	var vMin float64
	tries := c.cfg.SampleSize
	for i := 0; i < tries; i++ {
		e := c.data[c.keys[c.rng.Intn(n)]]
		if e == nil {
			continue
		}
		// Protect recently admitted objects (the adaptive window).
		if e.seq >= windowEdge {
			continue
		}
		v := c.value(e.fp, e.c)
		if victim == nil || v < vMin {
			victim, vMin = e, v
		}
	}
	if victim == nil {
		// All sampled entries were protected; fall back to any sample so the
		// cache can still make progress under a large window.
		e := c.data[c.keys[c.rng.Intn(n)]]
		return e
	}
	return victim
}

// windowEdge returns the smallest admission sequence number that is still
// considered "recent" given the current window size in bytes.
func (c *Cache) windowEdge() uint64 {
	if c.curByte == 0 {
		return 0
	}
	// Approximate: protect the most recent fraction windowBytes/curByte of
	// the admission sequence space.
	frac := float64(c.windowBytes) / float64(c.curByte)
	if frac > 1 {
		frac = 1
	}
	span := float64(c.seq) * frac
	if float64(c.seq) < span {
		return 0
	}
	return c.seq - uint64(span)
}

func (c *Cache) insert(key string, fp uint64, data []byte, cNew uintptr) {
	c.seq++
	e := &entry{key: key, fp: fp, value: data, c: cNew, seq: c.seq}
	c.data[key] = e
	c.keyIdx[key] = len(c.keys)
	c.keys = append(c.keys, key)
	c.curByte += cNew
}

func (c *Cache) remove(key string) {
	e, ok := c.data[key]
	if !ok {
		return
	}
	c.curByte -= e.c
	delete(c.data, key)
	// O(1) swap-remove from the keys slice.
	idx := c.keyIdx[key]
	last := len(c.keys) - 1
	c.keys[idx] = c.keys[last]
	c.keyIdx[c.keys[idx]] = idx
	c.keys = c.keys[:last]
	delete(c.keyIdx, key)
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
		Entries:     len(c.data),
		Bytes:       c.curByte,
		WindowBytes: c.windowBytes,
		HashFuncs:   c.cms.Depth(),
		SampleSize:  c.cfg.SampleSize,
		CMSWidth:    c.cms.Width(),
	}
}

// ensure math is referenced even if future edits drop its only use.
var _ = math.MaxUint16
