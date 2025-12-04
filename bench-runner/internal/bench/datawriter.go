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
}

func NewDataWriter() (*DataWriter, error) {
	ensureDir("/app/results")

	latencyFile, _ := os.Create("/app/results/latency.csv")
	throughputFile, _ := os.Create("/app/results/throughput.csv")
	hitratioFile, _ := os.Create("/app/results/hitratio.csv")
	evictionsFile, _ := os.Create("/app/results/evictions.csv")
	memoryFile, _ := os.Create("/app/results/memory.csv")

	w := &DataWriter{
		latencyFile:    csv.NewWriter(latencyFile),
		throughputFile: csv.NewWriter(throughputFile),
		hitratioFile:   csv.NewWriter(hitratioFile),
		evictionsFile:  csv.NewWriter(evictionsFile),
		memoryFile:     csv.NewWriter(memoryFile),
	}

	// Write headers
	w.latencyFile.Write([]string{"cache", "p50_us", "p95_us", "p99_us"})
	w.throughputFile.Write([]string{"cache", "ops_per_sec"})
	w.hitratioFile.Write([]string{"cache", "hit_ratio"})
	w.evictionsFile.Write([]string{"cache", "evictions"})
	w.memoryFile.Write([]string{"cache", "memory_mb"})

	return w, nil
}

func (w *DataWriter) WriteLatency(cache string, p50, p95, p99 float64) {
	w.latencyFile.Write([]string{
		cache,
		strconv.FormatFloat(p50, 'f', 2, 64),
		strconv.FormatFloat(p95, 'f', 2, 64),
		strconv.FormatFloat(p99, 'f', 2, 64),
	})
}

func (w *DataWriter) WriteThroughput(cache string, tps float64) {
	w.throughputFile.Write([]string{cache, strconv.FormatFloat(tps, 'f', 2, 64)})
}

func (w *DataWriter) WriteHitRatio(cache string, hitRatio float64) {
	w.hitratioFile.Write([]string{cache, strconv.FormatFloat(hitRatio, 'f', 4, 64)})
}

func (w *DataWriter) WriteEvictions(cache string, evictions int64) {
	w.evictionsFile.Write([]string{cache, strconv.FormatInt(evictions, 10)})
}

func (w *DataWriter) WriteMemory(cache string, memoryMB float64) {
	w.memoryFile.Write([]string{cache, strconv.FormatFloat(memoryMB, 'f', 2, 64)})
}

func (w *DataWriter) Flush() {
	w.latencyFile.Flush()
	w.throughputFile.Flush()
	w.hitratioFile.Flush()
	w.evictionsFile.Flush()
	w.memoryFile.Flush()
}

func ensureDir(path string) {
	os.MkdirAll(path, 0755)
}
