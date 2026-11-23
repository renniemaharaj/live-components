package main

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/renniemaharaj/live-components/livews"
)

func counterSocket(markup string) http.Handler {
	upgrader := websocket.Upgrader{
		HandshakeTimeout: 5 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		WriteBufferPool:  &sync.Pool{},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		initial := r.URL.Query().Get("count")
		if initial == "" {
			initial = "0"
		}

		count, err := strconv.ParseInt(initial, 10, 32)
		if err != nil {
			http.Error(w, "Invalid count.", http.StatusBadRequest)
			return
		}

		// One connection owns one tree, reused for all of its actions.
		page, err := renderCounter(markup, count)
		if err != nil {
			http.Error(w, "Could not render counter.", http.StatusInternalServerError)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.SetReadLimit(1024)
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			return
		}
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		})

		done := make(chan struct{})
		defer close(done)
		go counterHeartbeat(conn, done)

		value := page.GetElementById("count")
		state := page.GetElementById("counter-state")
		hidden := page.GetElementById("current-count")
		sendCount := func() error {
			text := strconv.FormatInt(count, 10)
			value.SetTextContent(text)
			hidden.SetAttribute("value", text)

			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				return err
			}

			return livews.WriteUpdates(conn, value, state)
		}

		// Push the initial state without waiting for a browser action.
		if err := sendCount(); err != nil {
			return
		}

		for {
			var action struct {
				Action string `json:"action"`
			}
			if err := conn.ReadJSON(&action); err != nil {
				return
			}

			next, actionErr := nextCount(count, action.Action)
			if actionErr != nil {
				if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
					return
				}

				if err := conn.WriteJSON(map[string]string{"error": actionErr.Error()}); err != nil {
					return
				}
				continue
			}

			count = next
			if err := sendCount(); err != nil {
				return
			}
		}
	})
}

func counterHeartbeat(conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			deadline := time.Now().Add(10 * time.Second)
			if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				conn.Close()
				return
			}
		}
	}
}
