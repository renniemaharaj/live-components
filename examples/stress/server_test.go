package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	live "github.com/renniemaharaj/live-component"
)

func TestDashboardIsRenderedThroughFramework(t *testing.T) {
	app := newLab()
	response := httptest.NewRecorder()
	app.routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("dashboard: %d %s", response.Code, response.Body.String())
	}
	page, err := live.Parse(response.Body.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(page.QuerySelectorAll("#suite-list tr")) != 6 || page.GetElementById("preview-title").TextContent() != "Hello Ada" {
		t.Fatal("dashboard did not compose the suite list and preview")
	}
	if page.GetElementById("metric-throughput").TextContent() != "—" || len(app.runner.snapshot().Results) != 0 {
		t.Fatal("initial dashboard must not invent benchmark data")
	}
	if response.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("dashboard response is missing its local asset policy")
	}

	for _, path := range []string{"/static/app.css", "/static/app.js", "/static/favicon.svg", "/api/state", "/api/results"} {
		response := httptest.NewRecorder()
		app.routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("missing endpoint %s: %d", path, response.Code)
		}
	}
}

func TestRunEndpointValidatesAndExecutes(t *testing.T) {
	app := newLab()
	c := smallConfig("dom")
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	app.routes().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(body)))
	if response.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", response.Code, response.Body.String())
	}
	if waitForRunner(t, app.runner).State != "completed" {
		t.Fatal("HTTP-triggered workload did not verify its result")
	}

	for _, invalid := range []string{`{`, `{}`, `{"unknown":1}`, string(body) + "{}", strings.Repeat(" ", 5000) + string(body)} {
		response := httptest.NewRecorder()
		app.routes().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(invalid)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid configuration admitted: %d", response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(body))
	request.Header.Set("Origin", "https://unrelated.example")
	response = httptest.NewRecorder()
	app.routes().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatal("unrelated origins must not start local resource-intensive workloads")
	}

	c.Iterations = 10000
	c.Components = 100
	body, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	app.routes().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(body)))
	if response.Code != http.StatusAccepted {
		t.Fatal("could not start cancellable workload")
	}
	conflict := httptest.NewRecorder()
	app.routes().ServeHTTP(conflict, httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(body)))
	if conflict.Code != http.StatusConflict {
		t.Fatal("HTTP endpoint admitted overlapping workloads")
	}
	stopped := httptest.NewRecorder()
	app.routes().ServeHTTP(stopped, httptest.NewRequest(http.MethodPost, "/api/stop", nil))
	if stopped.Code != http.StatusOK || waitForRunner(t, app.runner).State != "cancelled" {
		t.Fatal("HTTP cancellation did not stop the workload")
	}
}

func TestPreviewMutationsCloningAndEscaping(t *testing.T) {
	for _, template := range []string{"banner", "card", "table"} {
		t.Run(template, func(t *testing.T) {
			values := url.Values{
				"template":  {template},
				"name":      {`<script>text</script>`},
				"children":  {"2"},
				"clones":    {"1"},
				"active":    {"true"},
				"attribute": {"true"},
				"removed":   {"true"},
			}
			output, err := renderPreview(values)
			if err != nil {
				t.Fatal(err)
			}
			page, err := live.Parse(output)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output, "<script>") || len(page.QuerySelectorAll("[data-component]")) != 2 || len(page.QuerySelectorAll(".highlighted")) != 2 {
				t.Fatal("preview failed its text, clone, or class operation")
			}
			if len(page.QuerySelectorAll("[data-version=mutated]")) != 2 || page.GetElementById("preview-button") != nil {
				t.Fatal("preview failed its attribute or child removal operation")
			}
			if !strings.Contains(page.Root().TextContent(), "clone 1") {
				t.Fatal("clone did not receive an independent text mutation")
			}
			seen := make(map[string]bool)
			for _, element := range page.QuerySelectorAll("[id]") {
				id, _ := element.GetAttribute("id")
				if seen[id] {
					t.Fatalf("clone introduced duplicate ID %q", id)
				}
				seen[id] = true
			}
		})
	}

	for _, values := range []url.Values{
		{"template": {"../index"}},
		{"name": {strings.Repeat("x", 81)}},
		{"children": {"999"}},
		{"clones": {"-1"}},
		{"children": {"invalid"}},
	} {
		if _, err := renderPreview(values); err == nil {
			t.Fatal("invalid preview configuration was accepted")
		}
	}
}

func TestSSEStreamsRealSnapshotsAndCloses(t *testing.T) {
	app := newLab()
	server := httptest.NewServer(app.routes())
	defer server.Close()
	defer app.closeStreams()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("progress endpoint is not an event stream")
	}
	reader := bufio.NewReader(response.Body)
	event, err := reader.ReadString('\n')
	if err != nil || event != "event: snapshot\n" {
		t.Fatalf("missing snapshot event: %q %v", event, err)
	}
	data, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var value snapshot
	if err := json.Unmarshal([]byte(strings.TrimPrefix(data, "data: ")), &value); err != nil || value.State != "idle" || len(value.Results) != 0 {
		t.Fatal("initial event did not contain the actual runner state")
	}
	app.closeStreams()
	if _, err := io.ReadAll(reader); err != nil {
		t.Fatal("stream did not close cleanly:", err)
	}
}

func TestPreviewRemovesChildrenAndClonesThroughDOM(t *testing.T) {
	for _, template := range []string{"banner", "card", "table"} {
		for _, action := range []string{"remove-child", "remove-clone"} {
			values := url.Values{
				"template": {template},
				"children": {"1"},
				"clones":   {"0"},
				"action":   {action},
			}
			output, err := renderPreview(values)
			if err != nil {
				t.Fatal(err)
			}
			page, err := live.Parse(output)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.QuerySelectorAll("[data-component]")) != 1 {
				t.Fatal("removed clone remained attached")
			}
			if template == "table" {
				if len(page.QuerySelectorAll("tbody tr")) != 3 {
					t.Fatal("table child mutation did not preserve the correct row count")
				}
			} else if len(page.QuerySelectorAll(".demo-child")) != 1 {
				t.Fatal("child removal did not preserve the requested count")
			}
		}
	}
}
