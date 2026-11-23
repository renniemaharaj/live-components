package livehttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	live "github.com/renniemaharaj/live-components"
	"github.com/renniemaharaj/live-components/livehttp"
)

func TestWriteUpdatesRendersOnlySelectedContents(t *testing.T) {
	page, err := live.Parse(`<section id="panel"><span id="value:1">Old</span><p id="clear">Remove me</p><input id="untouched" value="keep"></section>`)
	if err != nil {
		t.Fatal(err)
	}

	value := page.GetElementById("value:1")
	value.SetTextContent(`<script>text & more</script>`)
	clear := page.GetElementById("clear")
	clear.SetTextContent("")

	response := httptest.NewRecorder()
	if err := livehttp.WriteUpdates(response, value, clear); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Updates []livehttp.Update `json:"updates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}

	if len(payload.Updates) != 2 ||
		payload.Updates[0].ID != "value:1" ||
		payload.Updates[0].HTML != "&lt;script&gt;text &amp; more&lt;/script&gt;" ||
		payload.Updates[1].ID != "clear" || payload.Updates[1].HTML != "" {
		t.Fatalf("unexpected updates: %s", response.Body.String())
	}

	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
		response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected headers: %v", response.Header())
	}
}

func TestWriteUpdatesValidatesBeforeWriting(t *testing.T) {
	valid := live.CreateElement("span")
	valid.SetAttribute("id", "value")
	duplicate := live.CreateElement("div")
	duplicate.SetAttribute("id", "value")

	for _, test := range []struct {
		name   string
		target *live.Element
	}{
		{name: "nil"},
		{
			name:   "zero",
			target: &live.Element{},
		},
		{
			name:   "text",
			target: live.CreateTextNode("text"),
		},
		{
			name:   "missing id",
			target: live.CreateElement("span"),
		},
		{
			name:   "duplicate id",
			target: duplicate,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			if err := livehttp.WriteUpdates(response, valid, test.target); err == nil {
				t.Fatal("invalid target was accepted")
			}

			if response.Body.Len() != 0 || len(response.Header()) != 0 {
				t.Fatal("validation wrote a partial response")
			}
		})
	}
}

func TestWriteUpdatesWithoutTargets(t *testing.T) {
	response := httptest.NewRecorder()
	if err := livehttp.WriteUpdates(response); err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(response.Body.String()) != `{"updates":[]}` {
		t.Fatalf("empty update batch: %s", response.Body.String())
	}
}

func TestRuntimeHandler(t *testing.T) {
	handler := livehttp.RuntimeHandler()
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/live.js", nil))
	if get.Code != http.StatusOK ||
		get.Header().Get("Content-Type") != "text/javascript; charset=utf-8" ||
		!strings.Contains(get.Body.String(), "export function applyUpdates") {
		t.Fatalf("runtime GET: %d %s", get.Code, get.Body.String())
	}

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/live.js", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 ||
		head.Header().Get("Content-Length") != get.Header().Get("Content-Length") {
		t.Fatalf("runtime HEAD: %d %v", head.Code, head.Header())
	}

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/live.js", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("runtime POST: %d %v", post.Code, post.Header())
	}
}
