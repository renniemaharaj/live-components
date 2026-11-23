package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	live "github.com/renniemaharaj/live-components"
)

func TestCounterWebSocketFlow(t *testing.T) {
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()
	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/counter/live"
	conn := dialCounter(t, address+"?count=40")
	readCounter(t, conn, "40")

	for _, action := range []string{"increment", "increment", "decrement", "reset"} {
		if err := conn.WriteJSON(map[string]string{"action": action}); err != nil {
			t.Fatal(err)
		}
	}

	for _, count := range []string{"41", "42", "41", "0"} {
		readCounter(t, conn, count)
	}

	if err := conn.WriteJSON(map[string]string{"action": "unknown"}); err != nil {
		t.Fatal(err)
	}
	var failure struct {
		Error string `json:"error"`
	}
	if err := conn.ReadJSON(&failure); err != nil || failure.Error == "" {
		t.Fatalf("invalid action did not return an error: %v, %v", failure, err)
	}

	if err := conn.WriteJSON(map[string]string{"action": "increment"}); err != nil {
		t.Fatal(err)
	}
	readCounter(t, conn, "1")

	// A separate connection must not inherit the first client's state.
	other := dialCounter(t, address)
	readCounter(t, other, "0")
}

func TestCounterWebSocketRejectsInvalidInput(t *testing.T) {
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()
	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/counter/live"

	for _, test := range []struct {
		name    string
		query   string
		headers http.Header
		status  int
	}{
		{
			name:   "invalid count",
			query:  "?count=invalid",
			status: http.StatusBadRequest,
		},
		{
			name:    "unrelated origin",
			headers: http.Header{"Origin": {"https://unrelated.example"}},
			status:  http.StatusForbidden,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn, response, err := websocket.DefaultDialer.Dial(address+test.query, test.headers)
			if conn != nil {
				conn.Close()
			}
			if response != nil {
				defer response.Body.Close()
			}

			if err == nil || response == nil || response.StatusCode != test.status {
				t.Fatalf("handshake: %v, %v", response, err)
			}
		})
	}

	conn := dialCounter(t, address+"?count=2147483647")
	readCounter(t, conn, "2147483647")
	if err := conn.WriteJSON(map[string]string{"action": "increment"}); err != nil {
		t.Fatal(err)
	}
	var failure struct {
		Error string `json:"error"`
	}
	if err := conn.ReadJSON(&failure); err != nil || failure.Error == "" {
		t.Fatalf("overflow did not return an error: %v, %v", failure, err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", 1025))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
		t.Fatalf("oversized message did not close the connection: %v", err)
	}
}

func TestCounterWebSocketRetainsAuthentication(t *testing.T) {
	t.Setenv("JWT_ENABLED", "true")
	t.Setenv("JWT_SECRET", strings.Repeat("x", 32))

	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/", "/counter/live"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s bypassed authentication: %d", path, response.Code)
		}
	}
}

func dialCounter(t *testing.T, address string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(address, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}

	return conn
}

func readCounter(t *testing.T, conn *websocket.Conn, count string) {
	t.Helper()
	var batch live.UpdateBatch
	if err := conn.ReadJSON(&batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Updates) != 2 || batch.Updates[0].ID != "count" ||
		batch.Updates[0].HTML != count || batch.Updates[1].ID != "counter-state" {
		t.Fatalf("unexpected count %s: %v", count, batch)
	}

	state, err := live.Parse(batch.Updates[1].HTML)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := state.GetElementById("current-count").GetAttribute("value")
	if value != count {
		t.Fatalf("hidden count = %q, want %q", value, count)
	}
}
