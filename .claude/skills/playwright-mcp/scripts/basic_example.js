#!/usr/bin/env node

/**
 * Basic example of programmatic Playwright MCP usage
 * 
 * This demonstrates:
 * - Creating a connection with browser configuration
 * - Setting up SSE transport
 * - Using the connection to automate browser tasks
 * 
 * Usage: node basic_example.js
 */

import http from 'http';
import { createConnection } from '@playwright/mcp';
import { SSEServerTransport } from '@modelcontextprotocol/sdk/server/sse.js';

async function startServer() {
  const server = http.createServer(async (req, res) => {
    if (req.url === '/mcp' && req.method === 'GET') {
      // Create headless browser connection
      const connection = await createConnection({
        browser: {
          launchOptions: {
            headless: true,
            channel: 'chrome', // or 'chromium', 'firefox', 'webkit'
          },
          contextOptions: {
            viewport: { width: 1920, height: 1080 }
          }
        },
        capabilities: ['tabs', 'pdf'], // Optional capabilities
        outputDir: './output'
      });

      // Setup SSE transport
      const transport = new SSEServerTransport('/messages', res);
      await connection.connect(transport);

      console.log('MCP connection established');

      // Connection is now ready to receive tool calls
      // The client can now call browser automation tools
    } else if (req.url === '/messages' && req.method === 'POST') {
      // Handle incoming messages from MCP client
      // (transport handles this automatically)
    } else {
      res.writeHead(404);
      res.end('Not found');
    }
  });

  const PORT = process.env.PORT || 8931;
  server.listen(PORT, () => {
    console.log(`Playwright MCP server running on http://localhost:${PORT}/mcp`);
  });
}

startServer().catch(console.error);
