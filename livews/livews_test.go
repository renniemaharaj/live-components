package livews_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	live "github.com/renniemaharaj/live-components"
	"github.com/renniemaharaj/live-components/livehttp"
	"github.com/renniemaharaj/live-components/livews"
)

func TestWebSocketUsesHTTPUpdateFormat(t *testing.T) {
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()

		value := live.CreateElement("span")
		value.SetAttribute("id", "value:1")
		value.SetTextContent("Initial push")
		if err := livews.WriteUpdates(conn, value); err != nil {
			result <- err
			return
		}

		var action struct {
			Text string `json:"text"`
		}
		if err := conn.ReadJSON(&action); err != nil {
			result <- err
			return
		}

		value.SetTextContent(action.Text)
		result <- livews.WriteUpdates(conn, value)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}

	var pushed live.UpdateBatch
	if err := conn.ReadJSON(&pushed); err != nil {
		t.Fatal(err)
	}

	value := live.CreateElement("span")
	value.SetAttribute("id", "value:1")
	value.SetTextContent("Initial push")
	response := httptest.NewRecorder()
	if err := livehttp.WriteUpdates(response, value); err != nil {
		t.Fatal(err)
	}

	var requested live.UpdateBatch
	if err := json.Unmarshal(response.Body.Bytes(), &requested); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pushed, requested) {
		t.Fatalf("transport formats differ: %v, %v", pushed, requested)
	}

	if err := conn.WriteJSON(map[string]string{"text": "<New & escaped>"}); err != nil {
		t.Fatal(err)
	}

	var reply live.UpdateBatch
	if err := conn.ReadJSON(&reply); err != nil {
		t.Fatal(err)
	}
	if len(reply.Updates) != 1 || reply.Updates[0].ID != "value:1" ||
		reply.Updates[0].HTML != "&lt;New &amp; escaped&gt;" {
		t.Fatalf("unexpected reply: %v", reply)
	}

	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestWriteUpdatesRejectsTargetsBeforeUsingConnection(t *testing.T) {
	// A nil connection is safe here because validation must fail before writing.
	if err := livews.WriteUpdates(nil, live.CreateElement("span")); err == nil {
		t.Fatal("accepted a target without an ID")
	}
}
