# WebSocket Hub Implementation

This package provides a WebSocket hub for broadcasting real-time Reddit updates to connected clients.

## Architecture Overview

The implementation follows the **channel-based pattern** from the [Gorilla WebSocket chat example](https://github.com/gorilla/websocket/tree/master/examples/chat), which provides excellent concurrency guarantees through a single-threaded event loop.

### Core Components

#### Hub (`hub.go`)
The central message broker that manages all client connections and broadcasts.

**Key Features:**
- **Single goroutine event loop** - All operations (register, unregister, broadcast) processed sequentially
- **Lock-free design** - No mutexes needed since only one goroutine accesses the clients map
- **Non-blocking broadcasts** - Uses `select` with default case to prevent slow clients from blocking others
- **Graceful shutdown** - Cleanly closes all client connections on shutdown

**Channels:**
- `register chan *Client` - New client connections (buffered: 16)
- `unregister chan *Client` - Client disconnections (buffered: 16)
- `broadcast chan []byte` - Messages to broadcast to all clients (buffered: 256)
- `done chan struct{}` - Shutdown signal

#### Client (`client.go`)
Represents a single WebSocket connection with separate read/write goroutines.

**Read Pump (`readPump`):**
- Runs in dedicated goroutine per client
- Reads messages from WebSocket connection
- Handles pong messages for heartbeat
- Enforces `MaxMessageSize` limit (512 KB)
- Auto-unregisters client on disconnect

**Write Pump (`writePump`):**
- Runs in dedicated goroutine per client
- Writes messages from `send` channel to WebSocket
- Sends periodic ping messages (every 54 seconds)
- Coalesces queued messages for efficient writes
- Closes connection on channel close

**Configuration Constants:**
- `WriteWait`: 10 seconds - Time allowed to write a message
- `PongWait`: 60 seconds - Time to wait for pong response
- `PingPeriod`: 54 seconds - Interval for ping messages (90% of PongWait)
- `MaxMessageSize`: 512 KB - Maximum message size from client

#### Messages (`messages.go`)
Typed message system with JSON serialization.

**Message Types:**
- `post_update` - Reddit post updates
- `comment_update` - Comment updates
- `subreddit_update` - Subreddit information changes
- `error` - Error messages
- `ping/pong` - Heartbeat messages

**Message Envelope:**
```json
{
  "type": "post_update",
  "data": { ... },
  "timestamp": "2025-10-25T12:00:00Z"
}
```

#### Handler (`handler.go`)
HTTP handler for upgrading connections to WebSocket.

**Features:**
- Upgrades HTTP requests to WebSocket connections
- Generates unique client IDs (UUID)
- Configurable origin checking (allows all origins by default)
- Structured logging for connection events

## Concurrency Guarantees

### Thread Safety
1. **Hub Operations** - All hub state modifications happen in single goroutine (Run method)
2. **Client Map** - No concurrent access, only touched by Run goroutine
3. **Broadcast** - Safe to call from any goroutine, queues to channel
4. **Registration** - Safe to call from any goroutine (HTTP handlers)

### Non-Blocking Behavior
1. **Hub Broadcast** - Uses `select` with default to drop messages if channel full
2. **Client Send** - Uses `select` with default to drop messages for slow clients
3. **No Deadlocks** - Buffered channels and non-blocking sends prevent deadlocks

### Memory Safety
1. **Buffered Channels** - Prevent blocking on burst traffic
   - `broadcast`: 256 messages
   - `register/unregister`: 16 clients
   - `client.send`: 256 messages per client
2. **Message Size Limits** - `MaxMessageSize` prevents memory exhaustion
3. **Clean Disconnection** - Channels closed, connections properly closed

## Usage Example

```go
package main

import (
    "log/slog"
    "net/http"
    "os"

    "github.com/jamesprial/go-reddit-api-wrapper/internal/websocket"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Create and start hub
    hub := websocket.NewHub(logger)
    go hub.Run()

    // Configure WebSocket endpoint
    http.HandleFunc("/ws", websocket.Handler(hub))

    // Broadcast example
    go func() {
        data := websocket.PostUpdateData{
            Fullname:  "t3_abc123",
            Subreddit: "golang",
            Title:     "New post",
            Score:     42,
        }
        hub.BroadcastMessage(websocket.MessageTypePostUpdate, data)
    }()

    // Start server
    http.ListenAndServe(":8080", nil)
}
```

## Production Considerations

### Origin Checking
The default configuration allows all origins. In production, configure strict origin checking:

```go
websocket.SetCheckOrigin(func(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    return origin == "https://yourdomain.com"
})
```

### Buffer Sizing
Adjust buffer sizes based on your traffic patterns:

```go
websocket.SetBufferSizes(4096, 4096) // 4KB read/write buffers
```

### Monitoring
- Monitor `ClientCount()` for active connections
- Watch for "dropped message" log entries indicating slow clients
- Track "channel full" warnings indicating insufficient buffer sizes

### Graceful Shutdown
Always shut down the hub gracefully to close all connections:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
hub.Shutdown(ctx)
```

## Testing

The package follows Go best practices:
- All exported types and functions are documented
- Structured logging throughout
- Clean separation of concerns
- Testable design with interface boundaries

To test the WebSocket server:
```bash
# Install wscat for testing
npm install -g wscat

# Connect to server
wscat -c ws://localhost:8080/ws

# You should receive messages as they're broadcast
```

## Performance Characteristics

- **Lock-free hub** - No mutex contention on broadcast
- **Message coalescing** - Multiple queued messages sent in single write
- **Non-blocking sends** - Slow clients don't affect fast clients
- **Efficient heartbeat** - Minimal overhead from ping/pong
- **Buffer pooling** - Consider adding buffer pools for large message volumes

## Security Considerations

1. **Origin Validation** - Always validate origins in production
2. **Message Size Limits** - Enforced at 512 KB by default
3. **Client Authentication** - Add authentication layer before WebSocket upgrade
4. **Rate Limiting** - Consider per-client rate limiting for broadcasts
5. **TLS** - Always use WSS (WebSocket over TLS) in production
