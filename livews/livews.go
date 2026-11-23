// Package livews sends targeted HTML updates over Gorilla WebSocket connections.
package livews

import (
	"github.com/gorilla/websocket"
	live "github.com/renniemaharaj/live-components"
)

// WriteUpdates sends selected elements as one JSON message.
// Call from the connection's writer goroutine, or serialize writes with a mutex.
func WriteUpdates(conn *websocket.Conn, targets ...*live.Element) error {
	batch, err := live.NewUpdates(targets...)
	if err != nil {
		return err
	}

	return conn.WriteJSON(batch)
}
