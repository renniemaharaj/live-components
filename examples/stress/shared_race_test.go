//go:build unsafe_shared

package main

import (
	"strconv"
	"sync"
	"testing"

	live "github.com/renniemaharaj/live-components"
)

// This opt-in probe intentionally violates the framework's ownership contract.
func TestUnsafeSharedMutation(t *testing.T) {
	shared := live.CreateElement("div")
	shared.SetAttribute("data-counter", "0")
	var workers sync.WaitGroup
	for worker := 0; worker < 10; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for iteration := 0; iteration < 1000; iteration++ {
				shared.SetAttribute("data-counter", strconv.Itoa(worker+iteration))
			}
		}(worker)
	}
	workers.Wait()
}
