package websocket

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// upgrader configures the WebSocket upgrader with buffer sizes and origin checking.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin allows all connections by default.
	// In production, this should be configured to check against allowed origins.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// SetCheckOrigin allows customization of the origin checking function.
// This should be called during application initialization to configure
// which origins are allowed to connect.
//
// Example:
//
//	websocket.SetCheckOrigin(func(r *http.Request) bool {
//	    origin := r.Header.Get("Origin")
//	    return origin == "https://example.com"
//	})
func SetCheckOrigin(fn func(r *http.Request) bool) {
	upgrader.CheckOrigin = fn
}

// SetBufferSizes allows customization of the read and write buffer sizes.
// This should be called during application initialization if the default
// 1KB buffers are not suitable for your use case.
func SetBufferSizes(readSize, writeSize int) {
	if readSize > 0 {
		upgrader.ReadBufferSize = readSize
	}
	if writeSize > 0 {
		upgrader.WriteBufferSize = writeSize
	}
}

// ServeWs handles WebSocket upgrade requests and registers the client with the hub.
// This function should be used as an HTTP handler.
//
// Example:
//
//	hub := websocket.NewHub(logger)
//	go hub.Run()
//	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
//	    websocket.ServeWs(hub, w, r)
//	})
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if hub.logger != nil {
			hub.logger.LogAttrs(r.Context(), slog.LevelError, "failed to upgrade websocket",
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	// Generate a unique ID for this client
	clientID := uuid.New().String()

	// Create and register the client
	client := newClient(hub, conn, clientID, hub.logger)

	if hub.logger != nil {
		hub.logger.LogAttrs(r.Context(), slog.LevelInfo, "websocket connection established",
			slog.String("client_id", clientID),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)
	}

	// Register the client and start its pumps
	hub.RegisterClient(client)
}

// Handler returns an http.HandlerFunc that serves WebSocket connections.
// This is a convenience wrapper around ServeWs for use with routers that
// expect http.HandlerFunc.
//
// Example:
//
//	hub := websocket.NewHub(logger)
//	go hub.Run()
//	mux := http.NewServeMux()
//	mux.HandleFunc("/ws", websocket.Handler(hub))
func Handler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}
}
