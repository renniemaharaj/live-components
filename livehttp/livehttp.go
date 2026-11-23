// Package livehttp serves optional browser support for targeted HTML updates.
package livehttp

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	live "github.com/renniemaharaj/live-components"
)

//go:embed runtime.js
var runtime string

// Update is the shared targeted update type, retained here for compatibility.
type Update = live.Update

// WriteUpdates sends the inner HTML of explicitly selected elements as JSON.
// Every target needs a unique, nonempty ID. Validation finishes before writing.
func WriteUpdates(w http.ResponseWriter, targets ...*live.Element) error {
	batch, err := live.NewUpdates(targets...)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	return json.NewEncoder(w).Encode(batch)
}

// RuntimeHandler serves the browser ES module. Mount it at a path such as /live.js.
func RuntimeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		http.ServeContent(w, r, "live.js", time.Time{}, strings.NewReader(runtime))
	})
}
