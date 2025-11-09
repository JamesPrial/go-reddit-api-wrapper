#!/usr/bin/env node

/**
 * Advanced Playwright MCP examples
 * 
 * Demonstrates various configuration patterns:
 * - Isolated sessions
 * - Custom storage state
 * - Network filtering
 * - Different browser channels
 */

import { createConnection } from '@playwright/mcp';

// Example 1: Isolated session with storage state
async function isolatedWithStorage() {
  const connection = await createConnection({
    browser: {
      isolated: true,
      browserName: 'chromium',
      contextOptions: {
        storageState: './auth-state.json', // Pre-authenticated state
        viewport: { width: 1280, height: 720 }
      }
    }
  });
  return connection;
}

// Example 2: Persistent profile for testing
async function persistentProfile() {
  const connection = await createConnection({
    browser: {
      browserName: 'chromium',
      userDataDir: './test-profile',
      launchOptions: {
        headless: false, // Show browser for debugging
        slowMo: 100 // Slow down by 100ms per action
      }
    }
  });
  return connection;
}

// Example 3: Network filtering for scraping
async function withNetworkFiltering() {
  const connection = await createConnection({
    browser: {
      launchOptions: { headless: true }
    },
    network: {
      blockedOrigins: [
        'https://google-analytics.com',
        'https://facebook.com',
        'https://doubleclick.net'
      ],
      allowedOrigins: [
        'https://example.com',
        'https://api.example.com'
      ]
    }
  });
  return connection;
}

// Example 4: Mobile device emulation
async function mobileEmulation() {
  const connection = await createConnection({
    browser: {
      contextOptions: {
        userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X)',
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true
      }
    }
  });
  return connection;
}

// Example 5: With PDF and screenshot capabilities
async function withCapabilities() {
  const connection = await createConnection({
    browser: {
      launchOptions: { headless: true }
    },
    capabilities: ['pdf', 'tabs', 'vision'],
    outputDir: './screenshots'
  });
  return connection;
}

// Export for use in other modules
export {
  isolatedWithStorage,
  persistentProfile,
  withNetworkFiltering,
  mobileEmulation,
  withCapabilities
};
