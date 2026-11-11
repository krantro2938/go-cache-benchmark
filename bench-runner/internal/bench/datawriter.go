package bench

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
)

// DataWriter writes one row per (configuration, cache) to results.csv.
type DataWriter struct {
	f *os.File
	w *csv.Writer
}

var header = []string{
	"seed", "series", "config_id", "cache", "ops", "hits", "misses", "hit_ratio", "byte_hit_ratio",
	"evictions", "ops_per_sec", "p50_us", "p95_us", "p99_us", "memory_mb", "live_mb",
	"alloc_rate_mb_per_sec", "alloc_bytes_per_op", "allocs_per_op", "gc_cycles_per_sec", "gc_pause_us",
	"duration_s",
}

// NewDataWriter creates dir/results.csv.
func NewDataWriter(dir string) (*DataWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, "results.csv"))
	if err != nil {
		return nil, err
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	return &DataWriter{f: f, w: w}, nil
}

// Write appends one result row and flushes it, so partial runs are kept.
func (d *DataWriter) Write(seed int64, series, configID string, r *BenchmarkResult) error {
	ff := func(v float64, p int) string { return strconv.FormatFloat(v, 'f', p, 64) }
	hr := 0.0
	if r.Hits+r.Misses > 0 {
		hr = float64(r.Hits) / float64(r.Hits+r.Misses)
	}
	bhr := 0.0
	if r.ReqBytes > 0 {
		bhr = float64(r.HitBytes) / float64(r.ReqBytes)
	}
	row := []string{
		strconv.FormatInt(seed, 10), series, configID, r.CacheName,
		strconv.Itoa(r.TotalOps), strconv.FormatInt(r.Hits, 10), strconv.FormatInt(r.Misses, 10),
		ff(hr, 6), ff(bhr, 6), strconv.FormatInt(r.Evictions, 10),
		ff(float64(r.TotalOps)/r.Duration.Seconds(), 1),
		ff(r.P50, 3), ff(r.P95, 3), ff(r.P99, 3), ff(r.MemoryMB, 3), ff(r.LiveMB, 3),
		ff(r.AllocRate, 3), ff(r.AllocPerOp, 1), ff(r.AllocsPerOp, 4), ff(r.GCCycles, 4), ff(r.GCPause, 3),
		ff(r.Duration.Seconds(), 3),
	}
	if err := d.w.Write(row); err != nil {
		return err
	}
	d.w.Flush()
	return d.w.Error()
}

// Close flushes and closes the file.
func (d *DataWriter) Close() error {
	d.w.Flush()
	return d.f.Close()
}
