package live

import "fmt"

// Update replaces the inner HTML of the browser element with the given ID.
type Update struct {
	ID   string `json:"id"`
	HTML string `json:"html"`
}

// UpdateBatch is the shared payload for HTTP and WebSocket updates.
type UpdateBatch struct {
	Updates []Update `json:"updates"`
}

// NewUpdates snapshots the inner HTML of explicitly selected elements.
// Every target needs a unique, nonempty ID. No targets produces an empty array.
func NewUpdates(targets ...*Element) (UpdateBatch, error) {
	batch := UpdateBatch{Updates: make([]Update, 0, len(targets))}
	seen := make(map[string]bool, len(targets))

	for _, target := range targets {
		id, _ := target.GetAttribute("id")
		if id == "" {
			return UpdateBatch{}, fmt.Errorf("live: update target needs a nonempty id")
		}

		if seen[id] {
			return UpdateBatch{}, fmt.Errorf("live: duplicate update target %q", id)
		}

		seen[id] = true
		batch.Updates = append(batch.Updates, Update{
			ID:   id,
			HTML: target.InnerHTML(),
		})
	}

	return batch, nil
}
