# Docker Deployment Guide

This guide covers deploying the Reddit Tracker application using Docker and Docker Compose.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Configuration](#configuration)
- [Docker Commands](#docker-commands)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)
- [Data Management](#data-management)

## Prerequisites

- **Docker Engine** 20.10 or higher
- **Docker Compose** 2.0 or higher (comes with Docker Desktop)
- **Reddit API credentials** (see [Getting Reddit Credentials](#getting-reddit-credentials))

### Getting Reddit Credentials

1. Go to https://www.reddit.com/prefs/apps
2. Click "create another app..." at the bottom
3. Fill in the form:
   - **name**: reddit-tracker (or any name)
   - **type**: Select "script"
   - **description**: Personal Reddit tracker (optional)
   - **about url**: Leave blank
   - **redirect uri**: http://localhost:8080/callback
4. Click "create app"
5. Your **client ID** is the string under "personal use script"
6. Your **client secret** is labeled as "secret"

## Quick Start

### 1. Clone and Configure

```bash
# Clone the repository (if not already done)
git clone <repository-url>
cd go-reddit-api-wrapper

# Copy the environment template
cp .env.example .env

# Edit .env with your Reddit credentials
nano .env  # or use your preferred editor
```

### 2. Set Required Environment Variables

Edit `.env` and set at minimum:

```bash
REDDIT_CLIENT_ID=your_client_id_here
REDDIT_CLIENT_SECRET=your_client_secret_here
```

### 3. Start the Application

```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f
```

### 4. Access the Application

- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **WebSocket**: ws://localhost:8080/ws
- **Health Check**: http://localhost:8080/health

### 5. Add Subreddits to Track

Use the frontend UI or make API calls:

```bash
curl -X POST http://localhost:8080/api/subreddits \
  -H "Content-Type: application/json" \
  -d '{"name": "golang", "description": "The Go programming language"}'
```

## Architecture

The application consists of two services:

### Backend (`reddit-tracker-backend`)
- **Technology**: Go 1.25, SQLite
- **Port**: 8080
- **Responsibilities**:
  - REST API for subreddit/post/comment management
  - WebSocket server for real-time updates
  - Reddit API polling service
  - SQLite database for data persistence

### Frontend (`reddit-tracker-frontend`)
- **Technology**: SvelteKit, Node.js 20
- **Port**: 3000
- **Responsibilities**:
  - User interface for browsing tracked subreddits
  - Real-time updates via WebSocket
  - API client for backend communication

### Data Persistence

- **Volume**: `reddit-data` stores the SQLite database
- **Location**: `/data/reddit.db` inside the backend container
- **Persistence**: Data survives container restarts/rebuilds

## Configuration

### Environment Variables

#### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `REDDIT_CLIENT_ID` | Reddit OAuth2 client ID | `abc123xyz` |
| `REDDIT_CLIENT_SECRET` | Reddit OAuth2 client secret | `secret_key_here` |

#### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDDIT_USERNAME` | (empty) | Reddit username for user auth |
| `REDDIT_PASSWORD` | (empty) | Reddit password for user auth |
| `POLL_INTERVAL` | `60s` | How often to poll Reddit (min: 10s) |
| `LOG_LEVEL` | `info` | Logging level: debug, info, warn, error |
| `DEBUG` | `false` | Enable verbose debug logging |
| `ENABLE_CORS` | `true` | Enable CORS for API |
| `CORS_ORIGINS` | `*` | Allowed CORS origins |
| `USER_AGENT` | `reddit-tracker/1.0` | User agent for Reddit API |
| `VITE_API_URL` | `http://localhost:8080` | Backend API URL for frontend |
| `VITE_WS_URL` | `ws://localhost:8080` | WebSocket URL for frontend |

### Customizing Ports

Edit `docker-compose.yml` to change exposed ports:

```yaml
services:
  backend:
    ports:
      - "8081:8080"  # Change 8081 to your desired port

  frontend:
    ports:
      - "3001:3000"  # Change 3001 to your desired port
```

## Docker Commands

### Build and Start Services

```bash
# Build images and start containers
docker-compose up -d

# Build without using cache
docker-compose build --no-cache

# Start only backend
docker-compose up -d backend

# Start only frontend
docker-compose up -d frontend
```

### View Logs

```bash
# View all logs
docker-compose logs

# Follow logs in real-time
docker-compose logs -f

# View backend logs only
docker-compose logs -f backend

# View last 100 lines
docker-compose logs --tail=100
```

### Stop and Remove Services

```bash
# Stop containers (data persists)
docker-compose stop

# Stop and remove containers (data persists)
docker-compose down

# Remove containers and volumes (deletes all data)
docker-compose down -v
```

### Restart Services

```bash
# Restart all services
docker-compose restart

# Restart only backend
docker-compose restart backend
```

### Execute Commands in Containers

```bash
# Open shell in backend container
docker-compose exec backend sh

# Open shell in frontend container
docker-compose exec frontend sh

# View SQLite database
docker-compose exec backend sqlite3 /data/reddit.db
```

### Check Service Status

```bash
# View running containers
docker-compose ps

# View resource usage
docker stats
```

## Production Deployment

For production environments, implement these security and performance improvements:

### 1. Use Secrets Management

**DO NOT** use `.env` files in production. Instead:

#### Docker Swarm Secrets

```bash
# Create secrets
echo "your_client_id" | docker secret create reddit_client_id -
echo "your_client_secret" | docker secret create reddit_client_secret -

# Update docker-compose.yml to use secrets
```

#### Kubernetes Secrets

```bash
kubectl create secret generic reddit-credentials \
  --from-literal=client-id=your_client_id \
  --from-literal=client-secret=your_client_secret
```

### 2. Configure Reverse Proxy

Use nginx or Traefik for:
- HTTPS/TLS termination
- Load balancing
- Rate limiting
- Request caching

#### Example nginx configuration

```nginx
server {
    listen 443 ssl http2;
    server_name tracker.example.com;

    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;

    # Frontend
    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Backend API
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # WebSocket
    location /ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

### 3. Update Environment Variables

```bash
# Production .env
POLL_INTERVAL=300s          # 5 minutes (reduce API usage)
LOG_LEVEL=warn              # Less verbose logging
DEBUG=false
CORS_ORIGINS=https://tracker.example.com  # Specific domain
VITE_API_URL=https://tracker.example.com
VITE_WS_URL=wss://tracker.example.com
```

### 4. Enable Resource Limits

Add to `docker-compose.yml`:

```yaml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M

  frontend:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
        reservations:
          cpus: '0.25'
          memory: 128M
```

### 5. Configure Database Backups

```bash
# Create backup script
cat > backup.sh << 'EOF'
#!/bin/bash
BACKUP_DIR=/backups
DATE=$(date +%Y%m%d_%H%M%S)
docker-compose exec -T backend sqlite3 /data/reddit.db ".backup /data/backup_${DATE}.db"
docker cp reddit-tracker-backend:/data/backup_${DATE}.db ${BACKUP_DIR}/
EOF

chmod +x backup.sh

# Run daily via cron
0 2 * * * /path/to/backup.sh
```

### 6. Update Strategy

```bash
# 1. Pull latest changes
git pull

# 2. Rebuild images
docker-compose build --no-cache

# 3. Stop old containers
docker-compose down

# 4. Start new containers
docker-compose up -d

# 5. Verify health
curl http://localhost:8080/health
```

## Troubleshooting

### Backend Fails to Start

**Check logs:**
```bash
docker-compose logs backend
```

**Common issues:**

1. **Missing environment variables**
   ```
   Error: required environment variable REDDIT_CLIENT_ID is not set
   ```
   Solution: Ensure `.env` file has all required variables set

2. **Database permission errors**
   ```
   Error: failed to open database: unable to open database file
   ```
   Solution: Check volume permissions
   ```bash
   docker-compose down -v
   docker-compose up -d
   ```

3. **Reddit API authentication errors**
   ```
   Error: failed to create reddit client: invalid credentials
   ```
   Solution: Verify `REDDIT_CLIENT_ID` and `REDDIT_CLIENT_SECRET`

### Frontend Can't Connect to Backend

**Check network connectivity:**
```bash
# Test from host
curl http://localhost:8080/health

# Test from frontend container
docker-compose exec frontend wget -O- http://backend:8080/health
```

**Update environment variables:**
- For Docker internal networking, use `http://backend:8080`
- For external access, use `http://localhost:8080`

### WebSocket Connection Errors

**Verify WebSocket endpoint:**
```bash
# Test WebSocket connection (requires wscat: npm install -g wscat)
wscat -c ws://localhost:8080/ws
```

**Check CORS settings:**
- Ensure `ENABLE_CORS=true`
- Update `CORS_ORIGINS` to include your frontend URL

### High Memory Usage

**Check resource consumption:**
```bash
docker stats
```

**Solutions:**
- Increase `POLL_INTERVAL` to reduce API calls
- Add memory limits in docker-compose.yml
- Reduce worker count (requires code change)

### Database Corruption

**Recover from backup:**
```bash
# Stop services
docker-compose down

# Restore backup
docker run --rm -v reddit-data:/data -v /path/to/backup:/backup alpine \
  cp /backup/reddit.db /data/reddit.db

# Restart services
docker-compose up -d
```

**Reset database (destroys data):**
```bash
docker-compose down -v
docker-compose up -d
```

## Data Management

### Backup Database

```bash
# Create backup
docker-compose exec backend sqlite3 /data/reddit.db ".backup /data/backup.db"

# Copy to host
docker cp reddit-tracker-backend:/data/backup.db ./reddit_backup_$(date +%Y%m%d).db
```

### Restore Database

```bash
# Stop backend
docker-compose stop backend

# Copy backup to container
docker cp ./reddit_backup.db reddit-tracker-backend:/data/reddit.db

# Start backend
docker-compose start backend
```

### Export Data

```bash
# Export as SQL
docker-compose exec backend sqlite3 /data/reddit.db .dump > export.sql

# Export specific table as CSV
docker-compose exec backend sqlite3 -header -csv /data/reddit.db \
  "SELECT * FROM posts;" > posts.csv
```

### Inspect Database

```bash
# Open SQLite shell
docker-compose exec backend sqlite3 /data/reddit.db

# List tables
.tables

# View schema
.schema posts

# Query data
SELECT * FROM subreddits;

# Exit
.quit
```

### Reset Application State

```bash
# Remove all data and start fresh
docker-compose down -v
docker-compose up -d
```

## Support

For issues and questions:
- Check the [main README](README.md) for project documentation
- Review [troubleshooting](#troubleshooting) section above
- Open an issue on GitHub with logs from `docker-compose logs`

## License

See [LICENSE.md](LICENSE.md) for details.
