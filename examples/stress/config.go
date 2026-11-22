package main

import (
	"fmt"
	"slices"
)

type suite struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Unit        string `json:"unit"`
}

var suites = []suite{
	{
		ID:          "dom",
		Name:        "DOM Mutation",
		Description: "Append, remove, replace, attributes, and classList",
		Unit:        "transactions",
	},
	{
		ID:          "composition",
		Name:        "Component Composition",
		Description: "Nested trees, cloning, and state isolation",
		Unit:        "compositions",
	},
	{
		ID:          "concurrent",
		Name:        "Concurrent Requests",
		Description: "Parallel HTTP rendering and independent state",
		Unit:        "requests",
	},
	{
		ID:          "memory",
		Name:        "Memory Endurance",
		Description: "Repeated creation and disposal cycles",
		Unit:        "cycles",
	},
	{
		ID:          "selectors",
		Name:        "Selector Performance",
		Description: "CSS selectors and literal element lookups",
		Unit:        "query batches",
	},
	{
		ID:          "render",
		Name:        "Large Render",
		Description: "Serialize small, wide, deep, and mutated trees",
		Unit:        "renders",
	},
}

var operations = []string{
	"mixed", "query-selector", "query-selector-all", "get-element-by-id", "get-elements-by-tag-name",
	"append-child", "insert-before", "remove-child", "replace-child", "remove", "clone-node",
	"text-content", "inner-html", "set-attribute", "remove-attribute", "class-list", "class-name",
}

type config struct {
	Suite       string `json:"suite"`
	Components  int    `json:"components"`
	Iterations  int    `json:"iterations"`
	Concurrency int    `json:"concurrency"`
	Operation   string `json:"operation"`
	Scenario    string `json:"scenario"`
	Shape       string `json:"shape"`
	Timeout     int    `json:"timeout_seconds"`
}

func (c *config) validate() error {
	if c.Operation == "" {
		c.Operation = "mixed"
	}
	if c.Scenario == "" {
		c.Scenario = "independent"
	}
	if c.Shape == "" {
		c.Shape = "wide"
	}
	if c.Timeout == 0 {
		c.Timeout = 30
	}

	if c.Suite != "all" && !slices.ContainsFunc(suites, func(s suite) bool { return s.ID == c.Suite }) {
		return fmt.Errorf("choose an available test suite")
	}
	if c.Components < 1 || c.Components > 10000 {
		return fmt.Errorf("components must be between 1 and 10,000")
	}
	if c.Iterations < 1 || c.Iterations > 10000 || c.Components*c.Iterations > 2000000 {
		return fmt.Errorf("iterations must be between 1 and 10,000; components × iterations cannot exceed 2,000,000 per suite")
	}
	if c.Concurrency < 1 || c.Concurrency > 1000 {
		return fmt.Errorf("concurrency must be between 1 and 1,000")
	}
	if !slices.Contains(operations, c.Operation) {
		return fmt.Errorf("unknown DOM operation")
	}
	if c.Scenario == "shared-mutation" {
		return fmt.Errorf("shared DOM mutation is unsupported: use independent documents or immutable shared reads; see the separate race probe")
	}
	if c.Scenario != "independent" && c.Scenario != "shared-read" {
		return fmt.Errorf("unknown concurrency scenario")
	}
	if c.Shape != "wide" && c.Shape != "deep" && c.Shape != "small" && c.Shape != "medium" && c.Shape != "mutated" {
		return fmt.Errorf("unknown component shape")
	}
	if c.Timeout < 1 || c.Timeout > 120 {
		return fmt.Errorf("timeout must be between 1 and 120 seconds per suite")
	}

	return nil
}

func suiteByID(id string) suite {
	for _, candidate := range suites {
		if candidate.ID == id {
			return candidate
		}
	}

	return suite{}
}
