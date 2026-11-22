package main

import (
	"context"
	"strings"
	"testing"
	"time"

	live "github.com/renniemaharaj/live-components"
)

func smallConfig(id string) config {
	return config{
		Suite:       id,
		Components:  4,
		Iterations:  3,
		Concurrency: 2,
		Operation:   "mixed",
		Scenario:    "independent",
		Shape:       "wide",
		Timeout:     5,
	}
}

func TestWorkloadsVerifyRealDOMAndOutputs(t *testing.T) {
	for _, s := range suites {
		t.Run(s.ID, func(t *testing.T) {
			c := smallConfig(s.ID)
			work, err := prepareWorkload(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			if work.close != nil {
				defer work.close()
			}
			if work.instances != c.Components || work.nodes <= c.Components || work.unit == "" {
				t.Fatalf("invalid prepared dataset: %+v", work)
			}
			for component := 0; component < c.Components; component++ {
				for iteration := 0; iteration < c.Iterations; iteration++ {
					value, err := work.step(context.Background(), component, iteration)
					if err != nil {
						t.Fatal(err)
					}
					if value.latency <= 0 {
						t.Fatal("workload did not measure latency")
					}
					if s.ID == "render" && (value.renders != 1 || value.requests != 0 || value.domCalls != 0 || value.outputBytes == 0) {
						t.Fatal("isolated rendering mixed unrelated operation counters")
					}
					if s.ID == "concurrent" && (value.requests != 1 || value.renders != 1 || value.domCalls != 9) {
						t.Fatalf("HTTP operation counters are incorrect: %+v", value)
					}
					if s.ID == "dom" && value.domCalls != 19 {
						t.Fatalf("mixed DOM call accounting: %+v", value)
					}
				}
			}
		})
	}
}

func TestIndividualDOMOperations(t *testing.T) {
	for _, operation := range operations {
		if operation == "mixed" {
			continue
		}
		t.Run(operation, func(t *testing.T) {
			c := smallConfig("dom")
			c.Operation = operation
			work, err := prepareWorkload(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			for iteration := 0; iteration < 3; iteration++ {
				value, err := work.step(context.Background(), 0, iteration)
				if err != nil {
					t.Fatal(err)
				}
				if value.domCalls != 1 || value.latency <= 0 || work.unit != "DOM operations" {
					t.Fatalf("isolated DOM operation metrics: %+v", value)
				}
			}
		})
	}
}

func TestTreeShapesAndSharedReadRequests(t *testing.T) {
	for _, shape := range []string{"small", "medium", "wide", "deep", "mutated"} {
		for _, id := range []string{"render", "selectors", "composition"} {
			t.Run(id+"/"+shape, func(t *testing.T) {
				c := smallConfig(id)
				c.Shape = shape
				work, err := prepareWorkload(context.Background(), c)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := work.step(context.Background(), 3, 1); err != nil {
					t.Fatal(err)
				}
			})
		}
	}

	c := smallConfig("concurrent")
	c.Scenario = "shared-read"
	work, err := prepareWorkload(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer work.close()
	value, err := work.step(context.Background(), 0, 0)
	if err != nil || value.domCalls != 0 || value.requests != 1 || value.renders != 1 {
		t.Fatalf("immutable shared scenario performed mutations or failed: %+v, %v", value, err)
	}
}

func TestCorrectnessFailuresAreDetected(t *testing.T) {
	root, err := templateElement("card", "article")
	if err != nil {
		t.Fatal(err)
	}
	root.GetElementById("label").Remove()
	if _, err := mutateComponent(root, "broken"); err == nil {
		t.Fatal("missing template structure was accepted")
	}

	work := &workload{step: func(context.Context, int, int) (sample, error) {
		panic("intentional workload failure")
	}}
	if _, err := safeStep(context.Background(), work, 0, 0); err == nil || !strings.Contains(err.Error(), "intentional workload failure") {
		t.Fatal("worker panic was not reported as a failed assertion")
	}
}

func TestHistogramBoundsAndFixedMemory(t *testing.T) {
	var values histogram
	for i := 1; i <= 10000; i++ {
		values.add(time.Duration(i) * time.Microsecond)
	}
	for _, test := range []struct {
		quantile float64
		expected float64
	}{
		{
			quantile: .50,
			expected: 5,
		},
		{
			quantile: .95,
			expected: 9.5,
		},
		{
			quantile: .99,
			expected: 9.9,
		},
	} {
		actual := values.percentile(test.quantile)
		if actual < test.expected || actual > test.expected*1.03125 {
			t.Fatalf("percentile %v was not within its documented upper bound: %v", test.quantile, actual)
		}
	}
	if len(values.buckets) != 2048 || values.count != 10000 {
		t.Fatal("latency storage must remain bounded independently of sample count")
	}
}

func BenchmarkMixedDOM(b *testing.B) {
	root, err := templateElement("card", "article")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := mutateComponent(root, "benchmark"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRender(b *testing.B) {
	pool, _, err := componentPool(context.Background(), 100)
	if err != nil {
		b.Fatal(err)
	}
	root, err := composedTree(context.Background(), pool, "wide")
	if err != nil {
		b.Fatal(err)
	}
	// Validate once before measuring the unchanged rendering operation.
	if _, err := renderTree(root, len(pool)); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var output string
	for index := 0; index < b.N; index++ {
		output = root.Render()
	}
	b.StopTimer()
	page, err := live.Parse(output)
	if err != nil || len(page.QuerySelectorAll("article")) != len(pool) {
		b.Fatal("last measured render failed its structure check")
	}
}

func TestComposedTemplatesHaveUniqueIDs(t *testing.T) {
	pool, _, err := componentPool(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	root, err := composedTree(context.Background(), pool, "mutated")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, element := range root.QuerySelectorAll("[id]") {
		id, _ := element.GetAttribute("id")
		if seen[id] {
			t.Fatalf("composition duplicated ID %q", id)
		}
		seen[id] = true
	}
}

func TestComposedWorkRejectsExcessiveConcurrentGraphSize(t *testing.T) {
	c := smallConfig("render")
	c.Components = 1000
	c.Concurrency = 1000
	if _, err := prepareWorkload(context.Background(), c); err == nil || !strings.Contains(err.Error(), "in-flight node budget") {
		t.Fatal("render preparation must reject excessive simultaneous graph copies")
	}
}
