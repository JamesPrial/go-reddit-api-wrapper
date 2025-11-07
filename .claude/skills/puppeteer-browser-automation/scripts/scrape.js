#!/usr/bin/env node
/**
 * Web Scraper - Extract data from webpages using Puppeteer
 * Usage: node scrape.js <url> <selector> [options]
 * 
 * Options:
 *   --multiple - Extract all matching elements (default: first only)
 *   --attr=<attribute> - Extract attribute value instead of text
 *   --json - Output as JSON
 *   --wait=<ms> - Wait time in milliseconds after page load
 */

const puppeteer = require('puppeteer');

async function scrapeData(url, selector, options = {}) {
  const browser = await puppeteer.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });

  try {
    const page = await browser.newPage();

    console.log(`Navigating to ${url}...`);
    await page.goto(url, { waitUntil: 'networkidle2' });

    // Optional wait time
    if (options.wait) {
      await page.waitForTimeout(options.wait);
    }

    // Wait for selector
    await page.waitForSelector(selector, { timeout: 10000 });

    console.log(`Extracting data from selector: ${selector}`);
    
    const data = await page.evaluate((sel, opts) => {
      const elements = document.querySelectorAll(sel);
      
      const extractValue = (el) => {
        if (opts.attr) {
          return el.getAttribute(opts.attr);
        }
        return el.textContent.trim();
      };

      if (opts.multiple) {
        return Array.from(elements).map(extractValue);
      } else {
        return elements[0] ? extractValue(elements[0]) : null;
      }
    }, selector, options);

    return data;
  } catch (error) {
    console.error('Error scraping data:', error.message);
    throw error;
  } finally {
    await browser.close();
  }
}

// CLI interface
if (require.main === module) {
  const args = process.argv.slice(2);
  const url = args[0];
  const selector = args[1];
  
  if (!url || !selector) {
    console.error('Usage: node scrape.js <url> <selector> [--multiple] [--attr=name] [--json] [--wait=ms]');
    process.exit(1);
  }

  const options = {
    multiple: args.includes('--multiple'),
    attr: args.find(arg => arg.startsWith('--attr='))?.split('=')[1],
    json: args.includes('--json'),
    wait: parseInt(args.find(arg => arg.startsWith('--wait='))?.split('=')[1]) || 0
  };

  scrapeData(url, selector, options)
    .then(data => {
      if (options.json) {
        console.log(JSON.stringify(data, null, 2));
      } else {
        console.log('\nExtracted data:');
        console.log(data);
      }
      process.exit(0);
    })
    .catch(() => process.exit(1));
}

module.exports = scrapeData;
