package websocket

import (
	"context"
	"log/slog"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to them.
// It uses a channel-based architecture where all operations happen in a single
// goroutine (Run method), eliminating the need for locks and ensuring thread safety.
//
// The Hub follows the pattern from gorilla/websocket's chat example:
// - Single event loop processes all operations sequentially
// - No locks needed since only one goroutine accesses the clients map
// - Non-blocking sends prevent slow clients from affecting others
type Hub struct {
	// clients is the set of registered clients
	// Only accessed from the Run goroutine, no lock needed
	clients map[*Client]bool

	// broadcast is the channel for messages to send to all clients
	broadcast chan []byte

	// register is the channel for new client registrations
	register chan *Client

	// unregister is the channel for client disconnections
	unregister chan *Client

	// done signals the hub to shut down
	done chan struct{}

	// wg tracks the Run goroutine for graceful shutdown
	wg sync.WaitGroup

	// logger for structured logging
	logger *slog.Logger
}

// NewHub creates a new Hub instance.
// The Hub must be started with Run() before use.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256), // Buffered to prevent blocking senders
		register:   make(chan *Client, 16), // Buffered for burst registrations
		unregister: make(chan *Client, 16), // Buffered for burst disconnections
		done:       make(chan struct{}),
		logger:     logger,
	}
}

// Run starts the hub's event loop.
// This method blocks and should be run in a separate goroutine.
// It processes client registrations, unregistrations, and broadcasts
// all in a single goroutine, ensuring thread-safe access to the clients map.
//
// Example:
//
//	hub := NewHub(logger)
//	go hub.Run()
func (h *Hub) Run() {
	h.wg.Add(1)
	defer h.wg.Done()

	if h.logger != nil {
		h.logger.LogAttrs(nil, slog.LevelInfo, "websocket hub starting")
	}

	for {
		select {
		case client := <-h.register:
			// Register new client
			h.clients[client] = true
			if h.logger != nil {
				h.logger.LogAttrs(nil, slog.LevelInfo, "client connected",
					slog.String("client_id", client.id),
					slog.Int("total_clients", len(h.clients)),
				)
			}

		case client := <-h.unregister:
			// Unregister client and clean up
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				if h.logger != nil {
					h.logger.LogAttrs(nil, slog.LevelInfo, "client disconnected",
						slog.String("client_id", client.id),
						slog.Int("total_clients", len(h.clients)),
					)
				}
			}

		case message := <-h.broadcast:
			// Broadcast message to all clients
			// Use non-blocking sends to prevent slow clients from blocking others
			for client := range h.clients {
				client.safeSend(message)
			}

			if h.logger != nil && h.logger.Enabled(nil, slog.LevelDebug) {
				h.logger.LogAttrs(nil, slog.LevelDebug, "broadcast message",
					slog.Int("bytes", len(message)),
					slog.Int("recipients", len(h.clients)),
				)
			}

		case <-h.done:
			// Shutdown: close all client connections
			if h.logger != nil {
				h.logger.LogAttrs(nil, slog.LevelInfo, "websocket hub shutting down",
					slog.Int("active_clients", len(h.clients)),
				)
			}

			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			return
		}
	}
}

// Broadcast sends a message to all connected clients.
// This is a non-blocking operation - if the broadcast channel is full,
// the message is dropped to prevent blocking the caller.
//
// This method is safe to call from multiple goroutines.
func (h *Hub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
		// Message queued successfully
	default:
		// Broadcast channel is full, drop the message
		if h.logger != nil {
			h.logger.LogAttrs(nil, slog.LevelWarn, "dropped broadcast message, channel full",
				slog.Int("channel_size", cap(h.broadcast)),
			)
		}
	}
}

// BroadcastMessage creates a typed Message and broadcasts it to all clients.
// This is a convenience method that combines NewMessage and Broadcast.
// Returns an error if the message cannot be marshaled.
//
// This method is safe to call from multiple goroutines.
func (h *Hub) BroadcastMessage(msgType MessageType, data interface{}) error {
	msgBytes, err := MarshalMessage(msgType, data)
	if err != nil {
		return err
	}
	h.Broadcast(msgBytes)
	return nil
}

// RegisterClient adds a new client to the hub and starts its read/write pumps.
// This method should be called after successfully upgrading an HTTP connection
// to a WebSocket connection.
//
// This method is safe to call from multiple goroutines (typically HTTP handlers).
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
	// Start the client's read and write pumps in separate goroutines
	go client.writePump()
	go client.readPump()
}

// Shutdown gracefully shuts down the hub.
// It signals the Run goroutine to stop and waits for it to complete.
// All client connections will be closed.
//
// This method blocks until the hub has fully shut down.
func (h *Hub) Shutdown(ctx context.Context) error {
	if h.logger != nil {
		h.logger.LogAttrs(ctx, slog.LevelInfo, "initiating hub shutdown")
	}

	// Signal shutdown
	close(h.done)

	// Wait for Run goroutine to complete with context timeout
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if h.logger != nil {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "hub shutdown complete")
		}
		return nil
	case <-ctx.Done():
		if h.logger != nil {
			h.logger.LogAttrs(ctx, slog.LevelWarn, "hub shutdown timeout",
				slog.String("error", ctx.Err().Error()),
			)
		}
		return ctx.Err()
	}
}

// ClientCount returns the current number of connected clients.
// Note: This is not thread-safe and should only be used for monitoring/debugging.
// For accurate counts, consider adding a clientCount channel to the Hub.
func (h *Hub) ClientCount() int {
	// This is racy but acceptable for monitoring purposes
	return len(h.clients)
}
