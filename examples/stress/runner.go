package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

var errBusy = errors.New("a benchmark is already running")

type logEntry struct {
	Sequence uint64    `json:"sequence"`
	Time     time.Time `json:"time"`
	Level    string    `json:"level"`
	Message  string    `json:"message"`
}

type chartPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
	Unit  string    `json:"unit"`
}

type result struct {
	ID         uint64      `json:"id"`
	Suite      string      `json:"suite"`
	Name       string      `json:"name"`
	Unit       string      `json:"unit"`
	Status     string      `json:"status"`
	Config     config      `json:"config"`
	Metrics    measurement `json:"metrics"`
	FinishedAt time.Time   `json:"finished_at"`
	Error      string      `json:"error,omitempty"`
}

type snapshot struct {
	State            string            `json:"state"`
	RunID            uint64            `json:"run_id"`
	Config           config            `json:"config"`
	Suite            string            `json:"suite"`
	Unit             string            `json:"unit"`
	Statuses         map[string]string `json:"statuses"`
	Components       int               `json:"components"`
	Nodes            int               `json:"nodes"`
	Workers          int               `json:"workers"`
	CurrentHeapBytes uint64            `json:"current_heap_bytes"`
	Goroutines       int               `json:"goroutines"`
	Planned          uint64            `json:"planned"`
	Metrics          measurement       `json:"metrics"`
	Results          []result          `json:"results"`
	Logs             []logEntry        `json:"logs"`
	Chart            []chartPoint      `json:"chart"`
}

type runner struct {
	mu       sync.Mutex
	state    snapshot
	current  *measurements
	cancel   context.CancelFunc
	done     chan struct{}
	sequence uint64
}

func newRunner() *runner {
	statuses := make(map[string]string)
	for _, s := range suites {
		statuses[s.ID] = "ready"
	}

	return &runner{state: snapshot{
		State:    "idle",
		Statuses: statuses,
	}}
}

func (r *runner) start(c config) error {
	if err := c.validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return errBusy
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	r.state.RunID++
	r.state.Config = c
	r.state.State = "preparing"
	r.state.Chart = nil
	r.state.Logs = nil
	r.state.Metrics = measurement{}
	r.state.Components = 0
	r.state.Nodes = 0
	r.state.Workers = min(c.Concurrency, c.Components)
	r.state.Planned = uint64(c.Components * c.Iterations)
	for _, s := range suites {
		r.state.Statuses[s.ID] = "ready"
		if c.Suite == s.ID || c.Suite == "all" {
			r.state.Statuses[s.ID] = "queued"
		}
	}
	go r.run(ctx, c, r.done)

	return nil
}

func (r *runner) stop() bool {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel == nil {
		return false
	}

	cancel()
	return true
}

func (r *runner) wait(ctx context.Context) error {
	r.mu.Lock()
	done := r.done
	r.mu.Unlock()
	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *runner) log(level, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sequence++
	r.state.Logs = append(r.state.Logs, logEntry{
		Sequence: r.sequence,
		Time:     time.Now(),
		Level:    level,
		Message:  message,
	})
	if len(r.state.Logs) > 120 {
		r.state.Logs = r.state.Logs[len(r.state.Logs)-120:]
	}
}

func (r *runner) snapshot() snapshot {
	r.mu.Lock()
	value := r.state
	value.Statuses = make(map[string]string, len(r.state.Statuses))
	for id, status := range r.state.Statuses {
		value.Statuses[id] = status
	}
	value.Results = append([]result(nil), r.state.Results...)
	value.Logs = append([]logEntry(nil), r.state.Logs...)
	value.Chart = append([]chartPoint(nil), r.state.Chart...)
	current := r.current
	r.mu.Unlock()

	if current != nil {
		value.Metrics = current.snapshot()
		value.CurrentHeapBytes = value.Metrics.HeapBytes
		value.Goroutines = value.Metrics.Goroutines
	} else {
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		value.CurrentHeapBytes = memory.HeapAlloc
		value.Goroutines = runtime.NumGoroutine()
		if value.Metrics.HeapBytes == 0 {
			value.Metrics.HeapBytes = value.CurrentHeapBytes
			value.Metrics.Goroutines = value.Goroutines
		}
	}

	return value
}

func (r *runner) run(ctx context.Context, c config, done chan struct{}) {
	defer func() {
		r.mu.Lock()
		r.cancel()
		r.cancel = nil
		for id, status := range r.state.Statuses {
			if status == "queued" {
				r.state.Statuses[id] = "cancelled"
			}
		}
		close(done)
		r.mu.Unlock()
	}()

	for _, s := range suites {
		if c.Suite != "all" && c.Suite != s.ID {
			continue
		}
		if ctx.Err() != nil {
			r.mu.Lock()
			r.state.State = "cancelled"
			r.mu.Unlock()
			break
		}

		selected := c
		selected.Suite = s.ID
		r.execute(ctx, selected)
		state := r.snapshot().State
		if state == "failed" || state == "cancelled" {
			break
		}
	}
}

func (r *runner) execute(parent context.Context, c config) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(c.Timeout)*time.Second)
	defer cancel()

	r.mu.Lock()
	r.state.State = "preparing"
	r.state.Suite = c.Suite
	r.state.Statuses[c.Suite] = "preparing"
	r.state.Unit = suiteByID(c.Suite).Unit
	r.current = nil
	r.state.Metrics = measurement{}
	r.state.Chart = nil
	r.state.Components = 0
	r.state.Nodes = 0
	r.mu.Unlock()
	r.log("info", "Preparing "+suiteByID(c.Suite).Name)
	r.log("info", fmt.Sprintf("Components: %d · iterations: %d · workers: %d · timeout: %ds", c.Components, c.Iterations, min(c.Concurrency, c.Components), c.Timeout))
	if c.Suite == "concurrent" {
		r.log("info", "HTTP scenario: "+c.Scenario+". Shared mutation is unsupported.")
	}

	work, err := prepareWorkload(ctx, c)
	if err != nil {
		r.complete(c, measurement{}, err)
		return
	}
	if work.close != nil {
		defer work.close()
	}

	metrics := newMeasurements()
	r.mu.Lock()
	r.current = metrics
	r.state.State = "running"
	r.state.Statuses[c.Suite] = "running"
	r.state.Components = work.instances
	r.state.Nodes = work.nodes
	r.state.Unit = work.unit
	r.mu.Unlock()
	r.log("info", fmt.Sprintf("Running workload · %d prepared DOM nodes · unit: %s", work.nodes, work.unit))

	workerContext, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	var workers sync.WaitGroup
	var firstError error
	var errorOnce sync.Once
	for worker := 0; worker < min(c.Concurrency, c.Components); worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for component := worker; component < c.Components; component += min(c.Concurrency, c.Components) {
				for iteration := 0; iteration < c.Iterations; iteration++ {
					if workerContext.Err() != nil {
						return
					}
					value, err := safeStep(workerContext, work, component, iteration)
					if err != nil && workerContext.Err() != nil {
						return
					}
					metrics.record(value, err)
					if err != nil {
						errorOnce.Do(func() {
							firstError = err
							stopWorkers()
						})
						return
					}
				}
			}
		}(worker)
	}

	finished := make(chan struct{})
	go func() {
		workers.Wait()
		close(finished)
	}()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	previousTime := time.Now()
	var previousCount uint64
	for {
		select {
		case <-finished:
			metrics.finish()
			values := metrics.snapshot()
			if c.Suite == "memory" {
				work.step = nil
				runtime.GC()
				var memory runtime.MemStats
				runtime.ReadMemStats(&memory)
				values.AfterGCHeap = memory.HeapAlloc
			}
			if firstError == nil {
				firstError = ctx.Err()
			}
			r.complete(c, values, firstError)
			return
		case now := <-ticker.C:
			values := metrics.snapshot()
			rate := float64(values.Successful-previousCount) / now.Sub(previousTime).Seconds()
			previousCount = values.Successful
			previousTime = now
			r.mu.Lock()
			r.state.Chart = append(r.state.Chart, chartPoint{
				Time:  now,
				Value: rate,
				Unit:  work.unit,
			})
			if len(r.state.Chart) > 240 {
				r.state.Chart = r.state.Chart[len(r.state.Chart)-240:]
			}
			r.mu.Unlock()
			r.log("metric", fmt.Sprintf("%.0f %s/s · p95 ≤ %s · heap %.1f MiB · %d/%d verified", rate, work.unit, formatLatency(values.P95MS), float64(values.HeapBytes)/(1<<20), values.Successful, c.Components*c.Iterations))
			if values.HeapBytes > 512<<20 {
				errorOnce.Do(func() {
					firstError = fmt.Errorf("512 MiB process heap limit exceeded")
					stopWorkers()
				})
			}
		}
	}
}

func safeStep(ctx context.Context, work *workload, component, iteration int) (value sample, err error) {
	defer func() {
		if problem := recover(); problem != nil {
			err = fmt.Errorf("workload panicked: %v", problem)
		}
	}()

	return work.step(ctx, component, iteration)
}

func (r *runner) complete(c config, values measurement, err error) {
	status := "passed"
	state := "completed"
	message := "Correctness assertions passed"
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		status = "cancelled"
		state = "cancelled"
		message = err.Error()
	} else if err != nil {
		status = "failed"
		state = "failed"
		message = err.Error()
	}
	if status == "passed" && values.Successful != uint64(c.Components*c.Iterations) {
		status = "failed"
		state = "failed"
		message = "completed operation count differs from the configured workload"
	}

	r.mu.Lock()
	completed := result{
		ID:         r.state.RunID,
		Suite:      c.Suite,
		Name:       suiteByID(c.Suite).Name,
		Unit:       r.state.Unit,
		Status:     status,
		Config:     c,
		Metrics:    values,
		FinishedAt: time.Now(),
	}
	if status != "passed" {
		completed.Error = message
	}
	r.state.Results = append(r.state.Results, completed)
	if len(r.state.Results) > 60 {
		r.state.Results = r.state.Results[len(r.state.Results)-60:]
	}
	r.state.State = state
	r.state.Statuses[c.Suite] = status
	r.state.Metrics = values
	r.current = nil
	if len(r.state.Chart) == 0 && values.Total > 0 {
		r.state.Chart = append(r.state.Chart, chartPoint{
			Time:  time.Now(),
			Value: values.OpsPerSecond,
			Unit:  completed.Unit,
		})
	}
	r.mu.Unlock()
	r.log(status, fmt.Sprintf("%s: %s · %d verified · %.0f %s/s · p95 ≤ %s", completed.Name, message, values.Successful, values.OpsPerSecond, completed.Unit, formatLatency(values.P95MS)))
}

func formatLatency(milliseconds float64) string {
	return time.Duration(math.Ceil(milliseconds * float64(time.Millisecond))).String()
}
