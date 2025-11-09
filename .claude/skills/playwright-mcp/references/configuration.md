# Configuration Reference

## Connection Configuration

```javascript
import { createConnection } from '@playwright/mcp';

const connection = await createConnection({
  browser: {
    // Browser type: 'chromium', 'firefox', or 'webkit'
    browserName?: 'chromium' | 'firefox' | 'webkit',
    
    // Keep profile in memory (don't persist)
    isolated?: boolean,
    
    // Path to user data directory for persistence
    userDataDir?: string,
    
    // Launch options
    launchOptions?: {
      channel?: string,          // e.g., 'chrome', 'msedge'
      headless?: boolean,         // Run headless (default: true)
      executablePath?: string,    // Custom browser path
      // See: https://playwright.dev/docs/api/class-browsertype#browser-type-launch
    },
    
    // Context options
    contextOptions?: {
      viewport?: { width: number, height: number },
      // See: https://playwright.dev/docs/api/class-browser#browser-new-context
    },
    
    // Connect to existing browser
    cdpEndpoint?: string,
    remoteEndpoint?: string,
  },
  
  server?: {
    port?: number,              // Server port
    host?: string,              // Bind host (default: 'localhost')
  },
  
  // Additional capabilities: 'tabs', 'install', 'pdf', 'vision'
  capabilities?: Array<string>,
  
  // Output directory for files
  outputDir?: string,
  
  // Network configuration
  network?: {
    allowedOrigins?: string[],
    blockedOrigins?: string[],
  },
  
  // Image response handling: 'allow' | 'omit'
  imageResponses?: string,
});
```

## Command-Line Arguments

When using CLI mode (npx @playwright/mcp@latest):

```bash
--browser <browser>           # chrome, firefox, webkit, msedge
--headless                    # Run headless
--isolated                    # Memory-only profile
--user-data-dir <path>        # Profile directory
--storage-state <path>        # Initial storage state
--device <device>             # Emulate device (e.g., "iPhone 15")
--viewport-size <size>        # e.g., "1280x720"
--caps <caps>                 # vision, pdf, tabs, install
--port <port>                 # HTTP server port
--host <host>                 # Bind host
--timeout-action <ms>         # Action timeout (default: 5000ms)
--timeout-navigation <ms>     # Navigation timeout (default: 60000ms)
--proxy-server <proxy>        # Proxy configuration
--user-agent <ua>             # Custom user agent
--grant-permissions <perms>   # e.g., "geolocation,clipboard-read"
--save-trace                  # Save Playwright trace
--save-video <size>           # Save video (e.g., "800x600")
--output-dir <path>           # Output directory
```

## JSON Configuration File

```json
{
  "browser": {
    "browserName": "chromium",
    "isolated": false,
    "userDataDir": "/path/to/profile",
    "launchOptions": {
      "channel": "chrome",
      "headless": true
    },
    "contextOptions": {
      "viewport": {
        "width": 1920,
        "height": 1080
      }
    }
  },
  "server": {
    "port": 8931,
    "host": "localhost"
  },
  "capabilities": ["tabs", "pdf"],
  "outputDir": "./output",
  "network": {
    "allowedOrigins": ["https://example.com"],
    "blockedOrigins": ["https://ads.example.com"]
  }
}
```

Load with: `npx @playwright/mcp@latest --config path/to/config.json`

## Profile Modes

### Persistent Profile (Default)
- Stores login state, cookies, cache
- Location:
  - Windows: `%USERPROFILE%\AppData\Local\ms-playwright\mcp-{channel}-profile`
  - macOS: `~/Library/Caches/ms-playwright/mcp-{channel}-profile`
  - Linux: `~/.cache/ms-playwright/mcp-{channel}-profile`

### Isolated Mode
- Each session uses fresh profile
- State lost when browser closes
- Use `--isolated` flag or `isolated: true` in config

### Browser Extension Mode
- Connect to existing browser tabs
- Uses Chrome extension
- See: https://github.com/microsoft/playwright-mcp/tree/main/extension

## Transport Modes

### SSE (Server-Sent Events)
Default for programmatic usage:
```javascript
import { SSEServerTransport } from '@modelcontextprotocol/sdk/server/sse.js';
const transport = new SSEServerTransport('/messages', res);
await connection.connect(transport);
```

### HTTP
For headless systems or worker processes:
```bash
npx @playwright/mcp@latest --port 8931
```
Client config:
```json
{
  "mcpServers": {
    "playwright": {
      "url": "http://localhost:8931/mcp"
    }
  }
}
```

## Docker

```bash
docker run -i --rm --init \
  mcr.microsoft.com/playwright/mcp
```

Long-lived service:
```bash
docker run -d -i --rm --init \
  --entrypoint node \
  --name playwright \
  -p 8931:8931 \
  mcr.microsoft.com/playwright/mcp \
  cli.js --headless --browser chromium --no-sandbox --port 8931
```

## References

- Full launch options: https://playwright.dev/docs/api/class-browsertype#browser-type-launch
- Context options: https://playwright.dev/docs/api/class-browser#browser-new-context
- Storage state: https://playwright.dev/docs/auth
