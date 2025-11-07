# Reddit API Browser - Static Frontend

A modern, interactive web frontend for the Reddit API HTTP server built with Alpine.js, Water.css, and vanilla JavaScript.

## Features

### API Key Management
- Secure API key input with password field masking
- Automatic localStorage persistence for session continuity
- Authentication status indicator with live status badge
- Logout functionality to clear credentials

### Posts Browser
- Browse hot or new posts from any subreddit
- Real-time sort toggle (Hot/New)
- Post statistics display (score, comment count)
- Support for link posts and self-text posts
- Pagination with forward/backward navigation

### Comments Viewer
- Threaded comment display with indentation
- Author and score information for each comment
- Post preview showing original post title and stats
- Easy navigation back to posts list
- Foundation for loading more comments (extensible)

## Architecture

### Technology Stack
- **HTML5**: Semantic markup with accessibility attributes
- **Alpine.js (v3)**: Lightweight reactive framework via CDN
- **Water.css (v2)**: Classless CSS baseline styling via CDN
- **Custom CSS**: Enhanced theming with CSS variables
- **Vanilla JavaScript**: No build step required

### File Structure
```
static/
├── index.html        # Main HTML markup with Alpine.js templates
├── app.js           # Application logic and API integration
└── style.css        # Custom styling and theming
```

## Application State

The application uses Alpine.js's reactive data binding with a single root component:

```javascript
{
    // Authentication
    apiKey: '',
    authenticated: false,

    // Navigation
    view: 'posts' | 'comments',

    // Posts browsing
    subreddit: 'golang',
    sortBy: 'hot' | 'new',
    posts: [],
    pagination: { after, before, history }

    // Comments viewing
    comments: [],
    currentPost: null,

    // UI state
    loading: false,
    error: ''
}
```

## API Integration

### Authentication
The frontend passes API keys via the `X-API-Key` header:
```javascript
const headers = {
    'X-API-Key': this.apiKey,
    'Content-Type': 'application/json'
};
```

### Endpoints Used
- `GET /api/v1/health` - Health check (authentication verification)
- `GET /api/v1/posts/hot` - Get hot posts
- `GET /api/v1/posts/new` - Get new posts
- `GET /api/v1/posts/{subreddit}/{postID}/comments` - Get post comments

### Request Format
All requests include pagination parameters:
```
?subreddit=golang&limit=25&after=t3_abc123
```

### Response Handling
The frontend automatically extracts posts and comments from various response formats:
- Direct arrays: `[{ kind: 't3', data: {...} }]`
- Nested structure: `{ posts: [...] }`
- Reddit format: `{ data: { children: [...] } }`

## Styling

### Design Philosophy
- **Water.css baseline**: Provides clean typography and form styling
- **Custom variables**: Override Water.css with project-specific colors
- **Dark mode**: Automatic support via `prefers-color-scheme` media query
- **Accessibility**: WCAG 2.1 compliant with proper contrast and focus indicators

### CSS Variables
```css
--primary-color: #1a73e8
--primary-hover: #1557b0
--text-primary: #202124
--text-secondary: #5f6368
--border-color: #dadce0
--bg-secondary: #f8f9fa
--radius: 8px
```

### Responsive Design
- Mobile-first approach with breakpoint at 768px
- Full-width buttons on mobile devices
- Collapsible controls group on smaller screens
- Touch-friendly button sizing (min 44x44px)

## Usage

### Deployment
The server serves the static frontend at the root path:
```bash
cd cmd/reddit-server
go build -o reddit-server
./reddit-server
# Access at http://localhost:8080/
```

### For Development
The HTML file can be opened directly in a browser, but requires the API server running:
```bash
# Terminal 1: Start the API server
cd cmd/reddit-server
go run .

# Terminal 2: Serve static files (or open index.html in browser)
cd cmd/reddit-server/static
python3 -m http.server 3000
```

## Features in Detail

### API Key Management
- Password input masking for security
- Validation prevents empty keys
- Auto-focus on startup
- Clear feedback on authentication success/failure

### Post Display
- Title with proper text truncation
- Author attribution
- Score and comment count
- Content preview (first 150 chars for self-text)
- Link preview for link posts
- Hover effects with elevation

### Comment Threading
- Visual indentation based on nesting depth
- Border-left accent highlighting
- Flat list with depth metadata (can be extended for collapsing)
- Author and score on every comment
- Markdown formatting preserved (from API)

### Error Handling
- User-friendly error messages
- Auto-dismissing error banners (5 seconds)
- Manual close button for errors
- Loading state management on all operations

### Pagination
- History-based navigation (not cursor-based to simplify state)
- Disabled buttons when at bounds
- Previous/Next controls
- Automatic status updates

## Extensibility

### Adding Features
1. **New API endpoints**: Add methods to `appState()` object
2. **New views**: Add template sections with `x-if` directives
3. **New styles**: Add CSS rules or modify variables
4. **Offline support**: Extend localStorage persistence

### Loading More Comments
Currently shows a placeholder. To implement:
1. Add `loadMoreComments()` method that calls `/api/v1/posts/{linkID}/more-comments`
2. Update comment items to show collapse/expand
3. Add tree structure management for nested replies

## Accessibility Features

- Semantic HTML (header, main, section, article, footer)
- ARIA labels on form inputs and buttons
- `aria-describedby` for input help text
- `aria-pressed` for toggle buttons
- Proper heading hierarchy
- Focus indicators on interactive elements
- Reduced motion support for animations

## Browser Support

- Modern browsers with ES6 support
- Alpine.js requires ES6 (Chrome 51+, Firefox 54+, Safari 10+)
- Water.css supports all modern browsers
- No IE11 support

## Performance

- No build step or compilation
- CDN delivery for Alpine.js and Water.css
- Minimal JavaScript (no frameworks overhead)
- Efficient DOM updates via Alpine.js reactivity
- Buffer pooling and pagination for Reddit API responses

## Security

- API keys stored in localStorage (user's responsibility to use HTTPS)
- No sensitive data logged to console
- CORS headers handled by backend server
- Password field masking for API key input
- Sanitization of user input before API calls

## Future Enhancements

- Advanced search and filtering
- Comment collapse/expand with "load more" support
- User profile information
- Saved posts and comments
- Search across subreddits
- Keyboard navigation improvements
- Service worker for offline viewing
