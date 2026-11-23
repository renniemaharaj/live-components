package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	live "github.com/renniemaharaj/live-components"
	"github.com/renniemaharaj/live-components/livehttp"
)

func TestCounterFormFlow(t *testing.T) {
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	for _, step := range []struct {
		count  string
		action string
		want   string
	}{
		{count: "0", action: "increment", want: "1"},
		{count: "1", action: "increment", want: "2"},
		{count: "0", action: "decrement", want: "-1"},
		{count: "42", action: "reset", want: "0"},
	} {
		t.Run(step.count+"/"+step.action, func(t *testing.T) {
			response, err := server.Client().PostForm(server.URL+"/counter", url.Values{
				"count":  {step.count},
				"action": {step.action},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}

			page, err := live.Parse(string(body))
			if err != nil {
				t.Fatal(err)
			}

			hiddenCount, _ := page.GetElementById("current-count").GetAttribute("value")
			if response.StatusCode != http.StatusOK ||
				response.Request.Method != http.MethodGet ||
				response.Request.URL.Query().Get("count") != step.want ||
				page.GetElementById("count").TextContent() != step.want || hiddenCount != step.want {
				t.Fatalf("form did not redirect and render count %s: %s", step.want, body)
			}
		})
	}

	// Earlier submissions must not change a new visitor's initial count.
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	page, err := live.Parse(response.Body.String())
	if err != nil {
		t.Fatal(err)
	}

	if page.GetElementById("count").TextContent() != "0" {
		t.Fatal("a new visitor inherited another request's counter")
	}
}

func TestCounterRejectsInvalidForms(t *testing.T) {
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{
		"count=hello&action=increment",
		"count=0&action=unknown",
		"count=2147483647&action=increment",
		"count=-2147483648&action=decrement",
		"count=0&action=increment&extra=" + strings.Repeat("x", 1024),
	} {
		request := httptest.NewRequest(http.MethodPost, "/counter", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid form returned %d", response.Code)
		}
	}
}

func TestCounterFragmentFlow(t *testing.T) {
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}

	for _, step := range []struct {
		count  string
		action string
		want   string
	}{
		{count: "0", action: "increment", want: "1"},
		{count: "1", action: "increment", want: "2"},
		{count: "0", action: "decrement", want: "-1"},
		{count: "42", action: "reset", want: "0"},
	} {
		t.Run(step.count+"/"+step.action, func(t *testing.T) {
			body := url.Values{
				"count":  {step.count},
				"action": {step.action},
			}
			request := httptest.NewRequest(http.MethodPost, "/counter", strings.NewReader(body.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Accept", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK || response.Header().Get("Location") != "" ||
				response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("fragment response: %d %s", response.Code, response.Body.String())
			}

			var payload struct {
				Updates []livehttp.Update `json:"updates"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}

			if len(payload.Updates) != 2 ||
				payload.Updates[0].ID != "count" || payload.Updates[0].HTML != step.want ||
				payload.Updates[1].ID != "counter-state" {
				t.Fatalf("unexpected fragment targets: %s", response.Body.String())
			}

			state, err := live.Parse(payload.Updates[1].HTML)
			if err != nil {
				t.Fatal(err)
			}

			value, _ := state.GetElementById("current-count").GetAttribute("value")
			if value != step.want {
				t.Fatalf("hidden count = %q, want %q", value, step.want)
			}
		})
	}

	for _, path := range []string{"/live.js", "/static/app.js"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("missing browser asset %s: %d", path, response.Code)
		}
	}
}
