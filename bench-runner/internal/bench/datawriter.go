package bench

import (
	"encoding/csv"
	"os"
	"strconv"
)

type DataWriter struct {
	latencyFile    *csv.Writer
	throughputFile *csv.Writer
	hitratioFile   *csv.Writer
	evictionsFile  *csv.Writer
	memoryFile     *csv.Writer
	gcFile         *csv.Writer
}

func NewDataWriter() (*DataWriter, error) {
	ensureDir("/app/results")

	latencyFile, _ := os.Create("/app/results/latency.csv")
	throughputFile, _ := os.Create("/app/results/throughput.csv")
	hitratioFile, _ := os.Create("/app/results/hitratio.csv")
	evictionsFile, _ := os.Create("/app/results/evictions.csv")
	memoryFile, _ := os.Create("/app/results/memory.csv")
	gcFile, _ := os.Create("/app/results/gc.csv")

	w := &DataWriter{
		latencyFile:    csv.NewWriter(latencyFile),
		throughputFile: csv.NewWriter(throughputFile),
		hitratioFile:   csv.NewWriter(hitratioFile),
		evictionsFile:  csv.NewWriter(evictionsFile),
		memoryFile:     csv.NewWriter(memoryFile),
		gcFile:         csv.NewWriter(gcFile),
	}

	// Write headers
	w.latencyFile.Write([]string{"config_id", "cache", "p50_us", "p95_us", "p99_us"})
	w.throughputFile.Write([]string{"config_id", "cache", "ops_per_sec"})
	w.hitratioFile.Write([]string{"config_id", "cache", "hit_ratio"})
	w.evictionsFile.Write([]string{"config_id", "cache", "evictions"})
	w.memoryFile.Write([]string{"config_id", "cache", "memory_mb"})
	w.gcFile.Write([]string{"config_id", "cache", "allocs_per_op_k", "gc_pause_us", "gc_cycles_per_sec", "alloc_rate_mb_per_sec"})

	return w, nil
}

func (w *DataWriter) WriteLatency(configID, cache string, p50, p95, p99 float64) {
	w.latencyFile.Write([]string{configID, cache,
		strconv.FormatFloat(p50, 'f', 2, 64),
		strconv.FormatFloat(p95, 'f', 2, 64),
		strconv.FormatFloat(p99, 'f', 2, 64)})
}

func (w *DataWriter) WriteThroughput(configID, cache string, tps float64) {
	w.throughputFile.Write([]string{configID, cache, strconv.FormatFloat(tps, 'f', 2, 64)})
}

func (w *DataWriter) WriteHitRatio(configID, cache string, hitRatio float64) {
	w.hitratioFile.Write([]string{configID, cache, strconv.FormatFloat(hitRatio, 'f', 4, 64)})
}

func (w *DataWriter) WriteEvictions(configID, cache string, evictions int64) {
	w.evictionsFile.Write([]string{configID, cache, strconv.FormatInt(evictions, 10)})
}

func (w *DataWriter) WriteMemory(configID, cache string, memoryMB float64) {
	w.memoryFile.Write([]string{configID, cache, strconv.FormatFloat(memoryMB, 'f', 2, 64)})
}

func (w *DataWriter) WriteGC(configID, cache string, allocsPerOp, gcPause, gcCycles, allocRate float64) {
	w.gcFile.Write([]string{
		configID,
		cache,
		strconv.FormatFloat(allocsPerOp, 'f', 4, 64),
		strconv.FormatFloat(gcPause, 'f', 2, 64),
		strconv.FormatFloat(gcCycles, 'f', 4, 64),
		strconv.FormatFloat(allocRate, 'f', 2, 64),
	})
}

func (w *DataWriter) Flush() {
	w.latencyFile.Flush()
	w.throughputFile.Flush()
	w.hitratioFile.Flush()
	w.evictionsFile.Flush()
	w.memoryFile.Flush()
	w.gcFile.Flush()
}

func ensureDir(path string) {
	os.MkdirAll(path, 0755)
}
