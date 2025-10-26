package websocket

import (
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// WriteWait is the time allowed to write a message to the peer.
	WriteWait = 10 * time.Second

	// PongWait is the time allowed to read the next pong message from the peer.
	PongWait = 60 * time.Second

	// PingPeriod is the interval at which ping messages are sent to the peer.
	// Must be less than PongWait to allow for network latency.
	PingPeriod = (PongWait * 9) / 10 // 54 seconds

	// MaxMessageSize is the maximum message size allowed from peer.
	MaxMessageSize = 512 * 1024 // 512 KB
)

// Client represents a single WebSocket connection from a client.
// Each Client runs two goroutines: readPump reads from the WebSocket connection,
// and writePump writes messages from the send channel to the WebSocket.
type Client struct {
	// hub is the central message hub this client belongs to
	hub *Hub

	// conn is the WebSocket connection
	conn *websocket.Conn

	// send is a buffered channel of outbound messages
	send chan []byte

	// id uniquely identifies this client for logging
	id string

	// logger for structured logging
	logger *slog.Logger
}

// newClient creates a new Client instance.
// The client must be registered with the hub before use.
func newClient(hub *Hub, conn *websocket.Conn, id string, logger *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256), // Buffered to prevent blocking
		id:     id,
		logger: logger,
	}
}

// readPump reads messages from the WebSocket connection.
// It runs in a separate goroutine per client and pumps messages from the
// WebSocket connection to the hub. The application ensures there is at most
// one reader per connection by executing all reads from this goroutine.
//
// When the connection is closed or an error occurs, the client is unregistered
// from the hub and the connection is closed.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// Configure connection limits
	c.conn.SetReadLimit(MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	// Read messages in a loop
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				if c.logger != nil {
					c.logger.LogAttrs(nil, slog.LevelWarn, "websocket connection error",
						slog.String("client_id", c.id),
						slog.String("error", err.Error()),
					)
				}
			}
			break
		}

		// For now, we just log received messages
		// Future enhancement: handle client-to-server messages
		if c.logger != nil && c.logger.Enabled(nil, slog.LevelDebug) {
			c.logger.LogAttrs(nil, slog.LevelDebug, "received websocket message",
				slog.String("client_id", c.id),
				slog.Int("bytes", len(message)),
			)
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
// It runs in a separate goroutine per client. The application ensures there is
// at most one writer per connection by executing all writes from this goroutine.
//
// A ticker is used to send ping messages periodically. When the connection
// cannot be written to or the send channel is closed, the goroutine exits
// and the connection is closed.
func (c *Client) writePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			// Set write deadline
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))

			if !ok {
				// The hub closed the channel, send close message
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Get a writer for the next message
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Coalesce queued messages into the current write
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'}) // Newline separator for multiple messages
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// safeSend attempts to send a message to the client's send channel.
// If the channel is full, it drops the message to prevent blocking the hub.
// This is crucial for maintaining non-blocking broadcast behavior.
func (c *Client) safeSend(message []byte) {
	select {
	case c.send <- message:
		// Message sent successfully
	default:
		// Channel is full, drop the message
		if c.logger != nil {
			c.logger.LogAttrs(nil, slog.LevelWarn, "dropped message for slow client",
				slog.String("client_id", c.id),
				slog.Int("buffer_size", len(c.send)),
			)
		}
	}
}
