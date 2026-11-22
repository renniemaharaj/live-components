package main

import (
	"math"
	"math/bits"
	"runtime"
	"sync"
	"time"
)

// Thirty-two buckets per power of two bound percentile error to 3.125%.
type histogram struct {
	buckets [2048]uint64
	count   uint64
	total   time.Duration
}

func (h *histogram) add(duration time.Duration) {
	nanoseconds := uint64(max(duration, 1))
	exponent := bits.Len64(nanoseconds) - 1
	base := uint64(1) << exponent
	fraction := (nanoseconds - base) * 32 / base
	h.buckets[exponent*32+int(fraction)]++
	h.count++
	h.total += duration
}

func (h *histogram) percentile(percent float64) float64 {
	if h.count == 0 {
		return 0
	}

	target := uint64(math.Ceil(float64(h.count) * percent))
	var cumulative uint64
	for index, count := range h.buckets {
		cumulative += count
		if cumulative >= target {
			base := float64(uint64(1) << (index / 32))
			upper := base * (1 + float64(index%32+1)/32)
			return upper / float64(time.Millisecond)
		}
	}

	return 0
}

type sample struct {
	latency       time.Duration
	renderLatency time.Duration
	domCalls      uint64
	renders       uint64
	requests      uint64
	outputBytes   uint64
}

type measurement struct {
	Total          uint64  `json:"total_operations"`
	Successful     uint64  `json:"successful_operations"`
	Failed         uint64  `json:"failed_operations"`
	OpsPerSecond   float64 `json:"ops_per_second"`
	RequestsPerSec float64 `json:"requests_per_second"`
	Duration       float64 `json:"duration_seconds"`
	AverageMS      float64 `json:"average_ms"`
	P50MS          float64 `json:"p50_ms"`
	P95MS          float64 `json:"p95_ms"`
	P99MS          float64 `json:"p99_ms"`
	RenderP95MS    float64 `json:"render_p95_ms"`
	DOMCalls       uint64  `json:"dom_calls"`
	Renders        uint64  `json:"renders"`
	Requests       uint64  `json:"requests"`
	OutputBytes    uint64  `json:"output_bytes"`
	HeapBytes      uint64  `json:"heap_bytes"`
	TotalAlloc     uint64  `json:"total_alloc_bytes"`
	Allocations    uint64  `json:"allocations"`
	BytesPerOp     float64 `json:"bytes_per_op"`
	AllocsPerOp    float64 `json:"allocs_per_op"`
	GCs            uint32  `json:"gc_cycles"`
	GCPauseMS      float64 `json:"gc_pause_ms"`
	Goroutines     int     `json:"goroutines"`
	AfterGCHeap    uint64  `json:"after_gc_heap_bytes,omitempty"`
}

type measurements struct {
	mu       sync.Mutex
	started  time.Time
	finished time.Time
	baseline runtime.MemStats
	latency  histogram
	render   histogram
	values   measurement
}

func newMeasurements() *measurements {
	m := &measurements{started: time.Now()}
	runtime.ReadMemStats(&m.baseline)
	return m
}

func (m *measurements) record(value sample, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.values.Total++
	m.values.DOMCalls += value.domCalls
	m.values.Renders += value.renders
	m.values.Requests += value.requests
	m.values.OutputBytes += value.outputBytes
	if err != nil {
		m.values.Failed++
		return
	}

	m.values.Successful++
	m.latency.add(value.latency)
	if value.renders > 0 {
		m.render.add(value.renderLatency)
	}
}

func (m *measurements) finish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finished = time.Now()
}

func (m *measurements) snapshot() measurement {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)

	m.mu.Lock()
	defer m.mu.Unlock()

	values := m.values
	end := m.finished
	if end.IsZero() {
		end = time.Now()
	}
	values.Duration = end.Sub(m.started).Seconds()
	if values.Duration > 0 {
		values.OpsPerSecond = float64(values.Successful) / values.Duration
		values.RequestsPerSec = float64(values.Requests) / values.Duration
	}
	if m.latency.count > 0 {
		values.AverageMS = float64(m.latency.total) / float64(m.latency.count) / float64(time.Millisecond)
	}
	values.P50MS = m.latency.percentile(0.50)
	values.P95MS = m.latency.percentile(0.95)
	values.P99MS = m.latency.percentile(0.99)
	values.RenderP95MS = m.render.percentile(0.95)
	values.HeapBytes = memory.HeapAlloc
	values.TotalAlloc = memory.TotalAlloc - m.baseline.TotalAlloc
	values.Allocations = memory.Mallocs - m.baseline.Mallocs
	values.GCs = memory.NumGC - m.baseline.NumGC
	values.GCPauseMS = float64(memory.PauseTotalNs-m.baseline.PauseTotalNs) / float64(time.Millisecond)
	values.Goroutines = runtime.NumGoroutine()
	if values.Successful > 0 {
		values.BytesPerOp = float64(values.TotalAlloc) / float64(values.Successful)
		values.AllocsPerOp = float64(values.Allocations) / float64(values.Successful)
	}

	return values
}
