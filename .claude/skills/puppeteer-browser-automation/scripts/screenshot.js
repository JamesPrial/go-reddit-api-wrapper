#!/usr/bin/env node
/**
 * Screenshot Script - Capture webpage screenshots with Puppeteer
 * Usage: node screenshot.js <url> [output_path] [options]
 * 
 * Options:
 *   --fullpage - Capture full scrollable page
 *   --width=<number> - Viewport width (default: 1920)
 *   --height=<number> - Viewport height (default: 1080)
 *   --wait=<selector> - Wait for specific element before screenshot
 */

const puppeteer = require('puppeteer');

async function takeScreenshot(url, outputPath = 'screenshot.png', options = {}) {
  const browser = await puppeteer.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });

  try {
    const page = await browser.newPage();
    
    // Set viewport
    await page.setViewport({
      width: options.width || 1920,
      height: options.height || 1080
    });

    console.log(`Navigating to ${url}...`);
    await page.goto(url, { waitUntil: 'networkidle2' });

    // Wait for specific element if requested
    if (options.waitFor) {
      console.log(`Waiting for element: ${options.waitFor}`);
      await page.waitForSelector(options.waitFor, { timeout: 10000 });
    }

    console.log(`Taking screenshot...`);
    await page.screenshot({
      path: outputPath,
      fullPage: options.fullPage || false
    });

    console.log(`✓ Screenshot saved to ${outputPath}`);
  } catch (error) {
    console.error('Error taking screenshot:', error.message);
    throw error;
  } finally {
    await browser.close();
  }
}

// CLI interface
if (require.main === module) {
  const args = process.argv.slice(2);
  const url = args[0];
  const outputPath = args[1] || 'screenshot.png';
  
  if (!url) {
    console.error('Usage: node screenshot.js <url> [output_path] [--fullpage] [--width=N] [--height=N] [--wait=selector]');
    process.exit(1);
  }

  const options = {
    fullPage: args.includes('--fullpage'),
    width: parseInt(args.find(arg => arg.startsWith('--width='))?.split('=')[1]) || 1920,
    height: parseInt(args.find(arg => arg.startsWith('--height='))?.split('=')[1]) || 1080,
    waitFor: args.find(arg => arg.startsWith('--wait='))?.split('=')[1]
  };

  takeScreenshot(url, outputPath, options)
    .then(() => process.exit(0))
    .catch(() => process.exit(1));
}

module.exports = takeScreenshot;
