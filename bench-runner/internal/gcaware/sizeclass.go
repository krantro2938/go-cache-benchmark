// Package gcaware implements an adaptive, GC-aware local cache for Go.
//
// This file models the runtime memory overhead m_gc(x) used by the cost
// function C(x) = s(x) + m_gc(x). The estimate is derived from the Go
// allocator's size-class behaviour: every allocation request is rounded up
// to the nearest size class, so the difference between the requested size
// s(x) and the actually reserved span is real, reproducible internal
// fragmentation. We add a small fixed per-object overhead to account for the
// map slot and the heap pointer the runtime must scan during the GC mark
// phase.
package gcaware

// classToSize mirrors runtime/sizeclasses.go (Go 1.22). Each value is the
// number of bytes a span of that class actually reserves. A request of n
// bytes is served by the smallest class whose size is >= n.
var classToSize = [...]uintptr{
	0, 8, 16, 24, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176, 192,
	208, 224, 240, 256, 288, 320, 352, 384, 416, 448, 480, 512, 576,
	640, 704, 768, 896, 1024, 1152, 1280, 1408, 1536, 1792, 2048, 2304,
	2688, 3072, 3200, 3456, 4096, 4864, 5376, 6144, 6528, 6784, 6912,
	8192, 9472, 9728, 10240, 10880, 12288, 13568, 14336, 16384, 18432,
	19072, 20480, 21760, 24576, 27264, 28672, 32768,
}

const (
	// maxSmallSize is the largest object served by a size class. Larger
	// objects are allocated as "large" spans rounded up to the page size.
	maxSmallSize = 32768
	// pageSize is the Go runtime page size used for large allocations.
	pageSize = 8192
	// perObjectOverhead approximates the fixed runtime cost of holding an
	// object in the cache that is not part of the payload: the map bucket
	// slot plus the pointer word the GC must scan. It is a constant so the
	// estimate stays fully reproducible.
	perObjectOverhead = 16
)

// roundupSize returns the number of bytes the Go allocator actually reserves
// for a request of size bytes, following the size-class table for small
// objects and page rounding for large ones.
func roundupSize(size uintptr) uintptr {
	if size == 0 {
		return 0
	}
	if size <= maxSmallSize {
		for _, c := range classToSize {
			if c >= size {
				return c
			}
		}
	}
	// Large object: round up to a whole number of pages.
	pages := (size + pageSize - 1) / pageSize
	return pages * pageSize
}

// mGC estimates the runtime overhead m_gc(x) for a payload of s bytes:
// allocator size-class fragmentation plus the fixed per-object cost.
func mGC(s uintptr) uintptr {
	return (roundupSize(s) - s) + perObjectOverhead
}

// cost returns C(x) = s(x) + m_gc(x): the total reserved footprint of an
// object whose payload is s bytes.
func cost(s uintptr) uintptr {
	return s + mGC(s)
}
