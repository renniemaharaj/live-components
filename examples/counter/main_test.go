package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	live "github.com/renniemaharaj/live-component"
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
