package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func waitForRunner(t *testing.T, r *runner) snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := r.wait(ctx); err != nil {
		t.Fatal(err)
	}
	return r.snapshot()
}

func TestRunnerLifecycleAndDistinctCounters(t *testing.T) {
	r := newRunner()
	c := smallConfig("all")
	if r.snapshot().State != "idle" {
		t.Fatal("runner must start idle")
	}
	if err := r.start(c); err != nil {
		t.Fatal(err)
	}
	state := waitForRunner(t, r)
	if state.State != "completed" || len(state.Results) != len(suites) {
		t.Fatalf("all suites did not complete: %+v", state.Results)
	}
	for _, result := range state.Results {
		if result.Status != "passed" || result.Metrics.Successful != uint64(c.Components*c.Iterations) || result.Metrics.Failed != 0 || result.Metrics.OpsPerSecond <= 0 {
			t.Fatalf("result did not verify its configured work: %+v", result)
		}
		if result.Suite == "memory" && result.Metrics.AfterGCHeap == 0 {
			t.Fatal("endurance must report its post-GC heap sample")
		}
	}
	before := state.Metrics.TotalAlloc
	time.Sleep(time.Millisecond)
	if r.snapshot().Metrics.TotalAlloc != before {
		t.Fatal("completed metrics must not grow as later requests allocate memory")
	}
	state.Statuses["dom"] = "tampered"
	state.Results[0].Name = "tampered"
	if r.snapshot().Statuses["dom"] != "passed" || r.snapshot().Results[0].Name == "tampered" {
		t.Fatal("snapshot callers can mutate runner state")
	}
}

func TestRunnerRejectsOverlappingRunsAndCancels(t *testing.T) {
	r := newRunner()
	c := smallConfig("dom")
	c.Components = 1000
	c.Iterations = 1000
	if err := r.start(c); err != nil {
		t.Fatal(err)
	}
	if err := r.start(c); !errors.Is(err, errBusy) {
		t.Fatalf("overlapping run was admitted: %v", err)
	}
	if !r.stop() {
		t.Fatal("running test could not be stopped")
	}
	state := waitForRunner(t, r)
	if state.State != "cancelled" {
		t.Fatalf("expected cancellation, got %s", state.State)
	}
	if r.stop() {
		t.Fatal("completed cancellation kept an active runner")
	}
	if err := r.start(smallConfig("dom")); err != nil {
		t.Fatal("runner could not be reused after cancellation:", err)
	}
	if waitForRunner(t, r).State != "completed" {
		t.Fatal("second run failed")
	}
}

func TestConfigurationLimitsAndSupportedWorkerCounts(t *testing.T) {
	for _, workers := range []int{1, 10, 100, 500, 1000} {
		c := smallConfig("concurrent")
		c.Concurrency = workers
		if err := c.validate(); err != nil {
			t.Fatalf("worker count %d rejected: %v", workers, err)
		}
	}
	for _, invalid := range []struct {
		name   string
		change func(*config)
	}{
		{
			name:   "suite",
			change: func(c *config) { c.Suite = "missing" },
		},
		{
			name:   "components",
			change: func(c *config) { c.Components = 10001 },
		},
		{
			name:   "zero components",
			change: func(c *config) { c.Components = 0 },
		},
		{
			name:   "iterations",
			change: func(c *config) { c.Iterations = 10001 },
		},
		{name: "work cap", change: func(c *config) {
			c.Components = 10000
			c.Iterations = 1000
		}},
		{
			name:   "workers",
			change: func(c *config) { c.Concurrency = 1001 },
		},
		{
			name:   "timeout",
			change: func(c *config) { c.Timeout = 121 },
		},
		{
			name:   "operation",
			change: func(c *config) { c.Operation = "missing" },
		},
		{
			name:   "scenario",
			change: func(c *config) { c.Scenario = "missing" },
		},
		{
			name:   "shape",
			change: func(c *config) { c.Shape = "missing" },
		},
		{
			name:   "unsafe sharing",
			change: func(c *config) { c.Scenario = "shared-mutation" },
		},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			c := smallConfig("dom")
			invalid.change(&c)
			if err := c.validate(); err == nil {
				t.Fatal("invalid resource configuration was admitted")
			}
		})
	}
}

func TestBoundedHistoryAndFailures(t *testing.T) {
	r := newRunner()
	for index := 0; index < 200; index++ {
		r.log("info", "bounded log")
		r.complete(smallConfig("dom"), measurement{}, errors.New("assertion failed"))
	}
	state := r.snapshot()
	if len(state.Logs) != 120 || len(state.Results) != 60 || state.State != "failed" || !strings.Contains(state.Results[0].Error, "assertion failed") {
		t.Fatal("runner history was not bounded or lost failure status")
	}
}

func TestConcurrentOwnershipScenarios(t *testing.T) {
	for _, scenario := range []string{"independent", "shared-read"} {
		t.Run(scenario, func(t *testing.T) {
			r := newRunner()
			c := smallConfig("concurrent")
			c.Components = 100
			c.Iterations = 2
			c.Concurrency = 100
			c.Scenario = scenario
			c.Timeout = 10
			if err := r.start(c); err != nil {
				t.Fatal(err)
			}
			state := waitForRunner(t, r)
			if state.State != "completed" || state.Metrics.Requests != 200 || state.Metrics.Successful != 200 {
				t.Fatalf("concurrent %s requests did not all verify: %+v", scenario, state.Results)
			}
		})
	}
}

func TestRunnerDeadlineStopsSustainedWork(t *testing.T) {
	r := newRunner()
	c := smallConfig("selectors")
	c.Components = 1000
	c.Iterations = 1000
	c.Timeout = 1
	started := time.Now()
	if err := r.start(c); err != nil {
		t.Fatal(err)
	}
	state := waitForRunner(t, r)
	if state.State != "cancelled" || time.Since(started) > 5*time.Second {
		t.Fatalf("deadline did not bound the workload: %s", state.State)
	}
	if len(state.Results) != 1 || !strings.Contains(state.Results[0].Error, "deadline") {
		t.Fatal("deadline cancellation was not reported explicitly")
	}
}
