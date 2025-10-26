# Reddit Frontend

A simple Svelte-based frontend for Reddit authentication with a Go backend proxy.

## Architecture

This application consists of two parts:

1. **Backend Server** (`./server/`) - Go HTTP server that handles Reddit OAuth authentication
2. **Frontend Web App** (`./web/`) - Svelte application with login UI

The backend acts as a secure proxy, keeping Reddit credentials server-side and providing a JWT-based session system.

## Prerequisites

- Go 1.22 or higher
- Node.js 18 or higher
- Reddit API credentials (Client ID and Client Secret)

### Getting Reddit API Credentials

1. Go to [Reddit App Preferences](https://www.reddit.com/prefs/apps)
2. Click "Create App" or "Create Another App"
3. Choose **"script"** as the app type
4. Fill in the required fields:
   - **name**: Your app name (e.g., "My Reddit App")
   - **redirect uri**: `http://localhost` (required but not used for script type)
5. Click "Create app"
6. Note your **client ID** (under the app name) and **client secret**

## Setup

### 1. Backend Server Setup

```bash
cd frontend/server

# Install Go dependencies
go mod download

# Generate a secure JWT secret
export JWT_SECRET_KEY="$(openssl rand -base64 32)"

# Set Reddit API credentials
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"

# Start the backend server
go run .
```

The backend server will start on `http://localhost:8080`.

**Environment Variables:**
- `JWT_SECRET_KEY` - Secret key for JWT token signing (min 32 characters)
- `REDDIT_CLIENT_ID` - Your Reddit app client ID
- `REDDIT_CLIENT_SECRET` - Your Reddit app client secret

### 2. Frontend Web App Setup

In a new terminal:

```bash
cd frontend/web

# Install npm dependencies
npm install

# Start the development server
npm run dev
```

The frontend will start on `http://localhost:5173` and proxy API requests to the backend.

## Usage

1. Open your browser to `http://localhost:5173`
2. Enter your Reddit username and password
3. Click "Sign In"
4. After successful authentication, you'll see a dashboard with your Reddit karma stats

## Features

### Backend (`server/`)
- ✅ JWT-based session management
- ✅ Reddit OAuth2 password grant authentication
- ✅ Rate limiting (5 requests/second on login)
- ✅ Request body size limits (1MB max)
- ✅ Session cleanup (24-hour expiry)
- ✅ CORS enabled for local development
- ✅ Structured logging
- ✅ Graceful shutdown

**API Endpoints:**
- `POST /api/auth/login` - Authenticate with Reddit credentials
- `GET /api/auth/status` - Check authentication status
- `POST /api/auth/logout` - Invalidate session
- `GET /health` - Health check

### Frontend (`web/`)
- ✅ Responsive login form
- ✅ Form validation (client-side)
- ✅ Loading states during authentication
- ✅ Error handling and display
- ✅ Simple dashboard with user karma stats
- ✅ Logout functionality

## Project Structure

```
frontend/
├── server/              # Go backend server
│   ├── main.go          # HTTP server setup
│   ├── handlers.go      # Auth endpoint handlers
│   ├── session.go       # Session management & JWT
│   └── go.mod           # Go module definition
│
└── web/                 # Svelte frontend
    ├── src/
    │   ├── App.svelte   # Main app component
    │   ├── Login.svelte # Login form component
    │   ├── api.js       # Backend API client
    │   └── main.js      # Entry point
    ├── public/
    ├── package.json
    └── vite.config.js   # Vite config with backend proxy
```

## Development

### Backend Development

```bash
cd frontend/server

# Run with auto-reload (requires air)
air

# Run tests
go test ./...

# Build binary
go build -o reddit-server .
```

### Frontend Development

```bash
cd frontend/web

# Start dev server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

## Security Considerations

### Current Implementation (Development)
- Uses OAuth2 **password grant flow** (user credentials sent directly)
- JWT secret must be set via environment variable
- Sessions stored in-memory (lost on server restart)
- Rate limiting prevents brute-force attacks
- CORS restricted to localhost origins

### Production Recommendations
- Migrate to **OAuth2 Authorization Code flow** (more secure)
- Use Redis or database for session storage
- Implement per-IP rate limiting
- Use HTTPS everywhere
- Store JWT secret in secure secret manager (HashiCorp Vault, AWS Secrets Manager)
- Add session refresh tokens
- Implement CSRF protection
- Add comprehensive logging and monitoring

## Troubleshooting

### Backend won't start
- Ensure `JWT_SECRET_KEY` is at least 32 characters
- Verify Reddit API credentials are correct
- Check port 8080 is not already in use

### Frontend can't connect to backend
- Ensure backend server is running on port 8080
- Check browser console for CORS errors
- Verify Vite proxy configuration in `vite.config.js`

### Login fails with "Invalid credentials"
- Verify your Reddit username and password are correct
- Check backend logs for Reddit API errors
- Ensure your Reddit account is not locked or suspended

### Rate limit errors
- Backend enforces 5 login attempts per second globally
- Wait a few seconds and try again
- For production, implement per-IP rate limiting

## Next Steps

Possible enhancements for this application:

1. **Reddit API Integration**
   - Browse subreddits
   - View posts and comments
   - Search functionality
   - User profile management

2. **Authentication Improvements**
   - Remember me / persistent login
   - OAuth2 Authorization Code flow
   - Session refresh tokens
   - Two-factor authentication support

3. **UI/UX Enhancements**
   - Dark mode
   - Improved dashboard with charts
   - Real-time updates
   - Mobile app

4. **Infrastructure**
   - Docker containers
   - Production deployment guide
   - CI/CD pipeline
   - Monitoring and metrics

## License

This project is part of the go-reddit-api-wrapper repository. See the main repository for license information.

## Related

- Main repository: [go-reddit-api-wrapper](../../README.md)
- Reddit API documentation: [https://www.reddit.com/dev/api/](https://www.reddit.com/dev/api/)
- OAuth2 documentation: [https://github.com/reddit-archive/reddit/wiki/OAuth2](https://github.com/reddit-archive/reddit/wiki/OAuth2)
