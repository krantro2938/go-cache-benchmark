package gcaware

import (
	"fmt"
	"testing"
)

func TestBudgetNeverExceeded(t *testing.T) {
	c := New(DefaultConfig(64 << 10))
	sizes := []int{100, 1000, 5000, 20000}
	for i := 0; i < 20000; i++ {
		k := fmt.Sprintf("k%d", i%3000)
		if _, ok := c.Lookup(k); !ok {
			c.Set(k, make([]byte, sizes[i%len(sizes)]))
		}
		if c.curByte > c.cfg.MaxBytes {
			t.Fatalf("budget exceeded: %d > %d", c.curByte, c.cfg.MaxBytes)
		}
	}
	if len(c.keys) != len(c.data) || len(c.keyIdx) != len(c.data) {
		t.Fatalf("index mismatch: keys=%d data=%d idx=%d", len(c.keys), len(c.data), len(c.keyIdx))
	}
	var sum uintptr
	for _, e := range c.data {
		sum += e.c
	}
	if sum != c.curByte {
		t.Fatalf("byte accounting mismatch: %d vs %d", sum, c.curByte)
	}
}

// A rejected admission must leave the cache unchanged (no partial eviction).
func TestRejectedAdmissionLeavesCacheUnchanged(t *testing.T) {
	cfg := DefaultConfig(8 * 1040) // room for exactly 8 objects of 1 KB
	c := New(cfg)
	for i := 0; i < 8; i++ {
		k := fmt.Sprintf("hot%d", i)
		for j := 0; j < 50; j++ {
			c.Lookup(k) // make residents frequent
		}
		c.Set(k, make([]byte, 1024))
	}
	before := len(c.data)
	ev := c.evictions
	c.Lookup("cold")
	c.Set("cold", make([]byte, 4*1024)) // needs several victims, all more valuable
	if len(c.data) != before || c.evictions != ev {
		t.Fatalf("rejected admission modified cache: entries %d->%d, evictions %d->%d",
			before, len(c.data), ev, c.evictions)
	}
	if _, ok := c.data["cold"]; ok {
		t.Fatal("cold object admitted")
	}
}

func TestLambdaZeroIsHonoured(t *testing.T) {
	cfg := DefaultConfig(1 << 20)
	cfg.Lambda = 0
	c := New(cfg)
	if c.alpha != 0 || c.beta != 1 {
		t.Fatalf("lambda=0 not honoured: alpha=%v beta=%v", c.alpha, c.beta)
	}
}

func TestCostModel(t *testing.T) {
	cases := map[uintptr]uintptr{1000: 1024 + 16, 1024: 1024 + 16, 65536: 65536 + 16, 40000: 40960 + 16}
	for s, want := range cases {
		if got := cost(s); got != want {
			t.Errorf("cost(%d) = %d, want %d", s, got, want)
		}
	}
}

func TestReplaceExistingKey(t *testing.T) {
	c := New(DefaultConfig(1 << 20))
	c.Set("a", make([]byte, 100))
	c.Set("a", make([]byte, 500))
	if len(c.keys) != 1 || c.curByte != cost(500) {
		t.Fatalf("replace failed: keys=%d bytes=%d", len(c.keys), c.curByte)
	}
}
