package websocket

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message being sent.
type MessageType string

const (
	// MessageTypePostUpdate indicates a Reddit post has been updated
	MessageTypePostUpdate MessageType = "post_update"
	// MessageTypeCommentUpdate indicates a comment has been updated
	MessageTypeCommentUpdate MessageType = "comment_update"
	// MessageTypeSubredditUpdate indicates subreddit information has changed
	MessageTypeSubredditUpdate MessageType = "subreddit_update"
	// MessageTypeError indicates an error occurred
	MessageTypeError MessageType = "error"
	// MessageTypePing is a heartbeat ping from server to client
	MessageTypePing MessageType = "ping"
	// MessageTypePong is a heartbeat response from client to server
	MessageTypePong MessageType = "pong"
)

// Message is the envelope for all WebSocket messages sent to clients.
// It provides a consistent structure with type identification and timestamp.
type Message struct {
	// Type identifies what kind of data is in the Data field
	Type MessageType `json:"type"`
	// Data contains the type-specific payload as raw JSON
	Data json.RawMessage `json:"data,omitempty"`
	// Timestamp indicates when this message was created
	Timestamp time.Time `json:"timestamp"`
}

// PostUpdateData contains information about a Reddit post update.
type PostUpdateData struct {
	Fullname    string `json:"fullname"`              // Reddit fullname (e.g., "t3_abc123")
	Subreddit   string `json:"subreddit"`             // Subreddit name
	Title       string `json:"title"`                 // Post title
	Author      string `json:"author"`                // Author username
	Score       int    `json:"score"`                 // Current score/upvotes
	NumComments int    `json:"num_comments"`          // Number of comments
	URL         string `json:"url,omitempty"`         // External URL if link post
	Permalink   string `json:"permalink,omitempty"`   // Reddit permalink
	CreatedUTC  int64  `json:"created_utc,omitempty"` // Unix timestamp
}

// CommentUpdateData contains information about a comment update.
type CommentUpdateData struct {
	Fullname   string `json:"fullname"`              // Reddit fullname (e.g., "t1_xyz789")
	PostID     string `json:"post_id"`               // Parent post fullname
	ParentID   string `json:"parent_id"`             // Immediate parent fullname
	Author     string `json:"author"`                // Comment author
	Body       string `json:"body,omitempty"`        // Comment text
	Score      int    `json:"score"`                 // Current score
	Permalink  string `json:"permalink,omitempty"`   // Reddit permalink
	CreatedUTC int64  `json:"created_utc,omitempty"` // Unix timestamp
}

// SubredditUpdateData contains information about a subreddit update.
type SubredditUpdateData struct {
	Name        string `json:"name"`                         // Subreddit name (without r/ prefix)
	DisplayName string `json:"display_name"`                 // Display name with r/ prefix
	Subscribers int    `json:"subscribers"`                  // Current subscriber count
	ActiveUsers int    `json:"active_users,omitempty"`       // Currently active users
	Description string `json:"description,omitempty"`        // Subreddit description
	PublicDesc  string `json:"public_description,omitempty"` // Short public description
}

// ErrorData contains error information to send to clients.
type ErrorData struct {
	Code    string `json:"code,omitempty"`    // Error code
	Message string `json:"message"`           // Human-readable error message
	Details string `json:"details,omitempty"` // Additional error details
}

// NewMessage creates a new Message with the given type and marshals the data.
// Returns an error if the data cannot be marshaled to JSON.
func NewMessage(msgType MessageType, data interface{}) (*Message, error) {
	var rawData json.RawMessage
	if data != nil {
		marshaled, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = marshaled
	}

	return &Message{
		Type:      msgType,
		Data:      rawData,
		Timestamp: time.Now(),
	}, nil
}

// MarshalMessage creates a Message and marshals it to JSON bytes in one operation.
// This is a convenience function for the common case of creating and immediately
// serializing a message for transmission.
func MarshalMessage(msgType MessageType, data interface{}) ([]byte, error) {
	msg, err := NewMessage(msgType, data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(msg)
}

// UnmarshalData unmarshals the raw Data field into the provided value.
// The value parameter should be a pointer to the appropriate struct type
// (e.g., *PostUpdateData for MessageTypePostUpdate).
func (m *Message) UnmarshalData(v interface{}) error {
	if len(m.Data) == 0 {
		return nil
	}
	return json.Unmarshal(m.Data, v)
}
