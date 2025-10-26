# Reddit Tracker Frontend

A real-time SvelteKit frontend for the Reddit tracker application with WebSocket support.

## Features

- Real-time post updates via WebSocket
- Modern, responsive UI with TailwindCSS v4
- Subreddit management (add/remove tracked subreddits)
- Live post feed with filtering and sorting
- Connection status indicator
- TypeScript for type safety

## Tech Stack

- **SvelteKit** - Web framework (Svelte 5)
- **TailwindCSS v4** - Styling
- **TypeScript** - Type safety
- **WebSocket** - Real-time updates
- **adapter-node** - Node.js production deployment

## Prerequisites

- Node.js 18+ and npm
- Running Reddit Tracker backend (see `/cmd/examples/monitor`)

## Installation

```sh
npm install
```

## Configuration

Copy the example environment file:

```sh
cp .env.example .env
```

Edit `.env` to configure API endpoints:

```env
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
```

## Development

Start the development server:

```sh
npm run dev

# or open in browser automatically
npm run dev -- --open
```

The app will be available at `http://localhost:5173/`

## Building for Production

Create a production build:

```sh
npm run build
```

Preview the production build:

```sh
npm run preview
```

The built application uses `@sveltejs/adapter-node` and can be deployed to any Node.js environment.

## Project Structure

```
frontend/
├── src/
│   ├── lib/
│   │   ├── api/
│   │   │   ├── client.ts          # REST API client
│   │   │   └── types.ts           # TypeScript types
│   │   ├── stores/
│   │   │   ├── websocket.ts       # WebSocket store
│   │   │   └── realtimeData.ts    # Application state
│   │   └── components/
│   │       ├── ConnectionIndicator.svelte
│   │       ├── SubredditManager.svelte
│   │       ├── PostList.svelte
│   │       └── CommentList.svelte
│   ├── routes/
│   │   ├── +layout.svelte         # App layout
│   │   ├── +page.svelte           # Dashboard
│   │   └── subreddits/
│   │       └── +page.svelte       # Subreddit management
│   └── app.css                    # Tailwind imports
├── .env.example                   # Environment template
└── svelte.config.js               # SvelteKit config
```

## Usage

### Dashboard (`/`)

- View real-time posts from all tracked subreddits
- Filter by subreddit
- Sort by score, time, or comments
- Auto-updates via WebSocket

### Manage Subreddits (`/subreddits`)

- Add new subreddits to track
- Remove existing subreddits
- View subreddit stats

## Integration with Backend

The frontend expects the following backend endpoints:

**REST API:**
- `GET /api/subreddits` - List tracked subreddits
- `POST /api/subreddits` - Add subreddit
- `DELETE /api/subreddits/:name` - Remove subreddit
- `GET /api/posts` - Get all posts
- `GET /api/subreddits/:name/posts` - Get posts by subreddit
- `GET /api/posts/:fullname/comments` - Get post comments

**WebSocket:**
- `GET /ws` - WebSocket connection for real-time updates

Message types:
- `new_post` - New post published
- `post_update` - Existing post updated
- `posts_batch` - Batch of posts
- `benchmark` - Benchmark result

## Deployment

The app is built with `@sveltejs/adapter-node` for Node.js deployment.

Deploy the `build/` directory to your Node.js hosting:

```sh
npm run build
node build
```

For Docker deployment, see the main project README.
