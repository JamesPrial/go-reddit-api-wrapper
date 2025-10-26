package websocket_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/websocket"
)

// Example demonstrates basic WebSocket hub usage
func Example() {
	// Create logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create and start the hub
	hub := websocket.NewHub(logger)
	go hub.Run()

	// Set up WebSocket endpoint
	http.HandleFunc("/ws", websocket.Handler(hub))

	// Broadcast a post update
	postData := websocket.PostUpdateData{
		Fullname:    "t3_abc123",
		Subreddit:   "golang",
		Title:       "New Go Release",
		Author:      "gopher",
		Score:       100,
		NumComments: 25,
	}

	if err := hub.BroadcastMessage(websocket.MessageTypePostUpdate, postData); err != nil {
		logger.Error("failed to broadcast", "error", err)
	}

	fmt.Println("WebSocket hub is running on /ws")
	// Output: WebSocket hub is running on /ws
}

// ExampleHub_BroadcastMessage demonstrates broadcasting different message types
func ExampleHub_BroadcastMessage() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := websocket.NewHub(logger)
	go hub.Run()

	// Broadcast a comment update
	commentData := websocket.CommentUpdateData{
		Fullname: "t1_xyz789",
		PostID:   "t3_abc123",
		ParentID: "t3_abc123",
		Author:   "commenter",
		Body:     "Great post!",
		Score:    10,
	}

	err := hub.BroadcastMessage(websocket.MessageTypeCommentUpdate, commentData)
	if err != nil {
		logger.Error("broadcast failed", "error", err)
	}
}

// ExampleSetCheckOrigin demonstrates configuring origin validation
func ExampleSetCheckOrigin() {
	// Configure strict origin checking for production
	websocket.SetCheckOrigin(func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Only allow connections from your domain
		return origin == "https://yourdomain.com"
	})

	// Or allow multiple origins
	websocket.SetCheckOrigin(func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		allowedOrigins := map[string]bool{
			"https://yourdomain.com":     true,
			"https://www.yourdomain.com": true,
		}
		return allowedOrigins[origin]
	})
}

// ExampleNewMessage demonstrates creating typed messages
func ExampleNewMessage() {
	// Create a message with data
	errorData := websocket.ErrorData{
		Code:    "RATE_LIMIT",
		Message: "Rate limit exceeded",
		Details: "Please try again in 60 seconds",
	}

	msg, err := websocket.NewMessage(websocket.MessageTypeError, errorData)
	if err != nil {
		fmt.Printf("Error creating message: %v\n", err)
		return
	}

	fmt.Printf("Message type: %s\n", msg.Type)
	// Output: Message type: error
}
