package live_test

import (
	"testing"

	live "github.com/renniemaharaj/live-components"
)

func TestUpdatesSnapshotReusableTree(t *testing.T) {
	value := live.CreateElement("span")
	value.SetAttribute("id", "value")
	value.SetTextContent("First")

	first, err := live.NewUpdates(value)
	if err != nil {
		t.Fatal(err)
	}

	value.SetTextContent("Second")
	second, err := live.NewUpdates(value)
	if err != nil {
		t.Fatal(err)
	}

	if first.Updates[0].HTML != "First" || second.Updates[0].HTML != "Second" {
		t.Fatalf("updates did not snapshot the tree: %v, %v", first, second)
	}
}
