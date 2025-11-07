# Puppeteer Patterns and Best Practices

## Common Patterns

### Basic Browser Lifecycle

```javascript
const puppeteer = require('puppeteer');

// Launch browser
const browser = await puppeteer.launch({
  headless: true, // false to see browser
  args: ['--no-sandbox', '--disable-setuid-sandbox']
});

// Create page
const page = await browser.newPage();

// Set viewport
await page.setViewport({ width: 1920, height: 1080 });

// Navigate
await page.goto('https://example.com', { 
  waitUntil: 'networkidle2' // or 'load', 'domcontentloaded', 'networkidle0'
});

// Always close browser
await browser.close();
```

### Waiting Strategies

```javascript
// Wait for selector
await page.waitForSelector('#element', { timeout: 5000 });

// Wait for navigation
await page.waitForNavigation({ waitUntil: 'networkidle2' });

// Wait for timeout
await page.waitForTimeout(2000);

// Wait for function
await page.waitForFunction('window.dataLoaded === true');

// Wait for XPath
await page.waitForXPath('//button[contains(text(), "Submit")]');
```

### Element Interaction

```javascript
// Click element
await page.click('#button');

// Type text
await page.type('#input', 'Hello World');

// Select dropdown
await page.select('select#country', 'US');

// Check checkbox
await page.click('input[type="checkbox"]');

// Hover over element
await page.hover('#menu-item');

// Focus element
await page.focus('#input');

// Clear input
await page.evaluate(() => {
  document.querySelector('#input').value = '';
});
```

### Data Extraction

```javascript
// Extract text content
const text = await page.$eval('#element', el => el.textContent);

// Extract attribute
const href = await page.$eval('a', el => el.href);

// Extract multiple elements
const items = await page.$$eval('.item', elements => 
  elements.map(el => ({
    title: el.querySelector('.title')?.textContent,
    link: el.querySelector('a')?.href
  }))
);

// Complex extraction with page.evaluate
const data = await page.evaluate(() => {
  const results = [];
  document.querySelectorAll('.product').forEach(product => {
    results.push({
      name: product.querySelector('.name')?.textContent,
      price: product.querySelector('.price')?.textContent,
      image: product.querySelector('img')?.src
    });
  });
  return results;
});
```

### Screenshots and PDFs

```javascript
// Full page screenshot
await page.screenshot({ 
  path: 'screenshot.png', 
  fullPage: true 
});

// Element screenshot
const element = await page.$('#specific-element');
await element.screenshot({ path: 'element.png' });

// Generate PDF
await page.pdf({
  path: 'page.pdf',
  format: 'A4',
  printBackground: true,
  margin: { top: '1cm', right: '1cm', bottom: '1cm', left: '1cm' }
});
```

### Request Interception

```javascript
// Enable request interception
await page.setRequestInterception(true);

page.on('request', request => {
  // Block images
  if (request.resourceType() === 'image') {
    request.abort();
  }
  // Block specific domains
  else if (request.url().includes('ads.example.com')) {
    request.abort();
  }
  // Modify requests
  else if (request.url().includes('api.example.com')) {
    request.continue({
      headers: {
        ...request.headers(),
        'Authorization': 'Bearer token'
      }
    });
  }
  else {
    request.continue();
  }
});
```

### Handling Authentication

```javascript
// HTTP Basic Auth
await page.authenticate({
  username: 'user',
  password: 'pass'
});

// Cookie-based auth
await page.setCookie({
  name: 'session',
  value: 'abc123',
  domain: 'example.com'
});

// Login form automation
await page.type('#username', 'user@example.com');
await page.type('#password', 'password');
await page.click('#login-button');
await page.waitForNavigation();
```

### Error Handling

```javascript
try {
  await page.goto('https://example.com', { 
    timeout: 30000,
    waitUntil: 'networkidle2' 
  });
} catch (error) {
  if (error.name === 'TimeoutError') {
    console.error('Page load timeout');
  } else {
    console.error('Navigation failed:', error);
  }
}

// Handle page crashes
page.on('error', error => {
  console.error('Page crashed:', error);
});

// Handle console messages
page.on('console', msg => {
  console.log('Browser console:', msg.text());
});
```

## Best Practices

### Performance Optimization

1. **Disable unnecessary resources**
```javascript
await page.setRequestInterception(true);
page.on('request', request => {
  if (['image', 'stylesheet', 'font'].includes(request.resourceType())) {
    request.abort();
  } else {
    request.continue();
  }
});
```

2. **Use appropriate wait strategies**
```javascript
// ✅ Good - specific wait
await page.waitForSelector('#data-loaded');

// ❌ Bad - arbitrary timeout
await page.waitForTimeout(5000);
```

3. **Reuse browser instances**
```javascript
// Create one browser, multiple pages
const browser = await puppeteer.launch();
const page1 = await browser.newPage();
const page2 = await browser.newPage();
// Process multiple URLs in parallel
```

### Reliability Patterns

1. **Retry logic**
```javascript
async function retryOperation(operation, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await operation();
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      await new Promise(resolve => setTimeout(resolve, 1000 * (i + 1)));
    }
  }
}
```

2. **Graceful error handling**
```javascript
const browser = await puppeteer.launch();
try {
  const page = await browser.newPage();
  // operations
} catch (error) {
  console.error('Error:', error);
} finally {
  await browser.close(); // Always close
}
```

### Security Considerations

1. **Sanitize user input**
```javascript
// ❌ Bad - XSS vulnerability
await page.evaluate((userInput) => {
  document.body.innerHTML = userInput;
}, userProvidedHTML);

// ✅ Good - safe text insertion
await page.evaluate((userInput) => {
  document.querySelector('#target').textContent = userInput;
}, userProvidedText);
```

2. **Use sandbox mode**
```javascript
const browser = await puppeteer.launch({
  args: ['--no-sandbox', '--disable-setuid-sandbox']
});
```

## Common Issues and Solutions

### Issue: Page won't load
```javascript
// Solution: Increase timeout and try different wait strategies
await page.goto(url, { 
  timeout: 60000,
  waitUntil: ['load', 'domcontentloaded', 'networkidle0']
});
```

### Issue: Element not found
```javascript
// Solution: Wait for element with retry
async function waitAndClick(selector) {
  await page.waitForSelector(selector, { visible: true, timeout: 10000 });
  await page.click(selector);
}
```

### Issue: Memory leaks
```javascript
// Solution: Always close pages and browsers
const page = await browser.newPage();
try {
  // operations
} finally {
  await page.close();
}
```

### Issue: Bot detection
```javascript
// Solution: Use stealth mode
await page.evaluateOnNewDocument(() => {
  Object.defineProperty(navigator, 'webdriver', {
    get: () => false,
  });
});

await page.setUserAgent('Mozilla/5.0...');
await page.setViewport({ width: 1920, height: 1080 });
```

## Advanced Patterns

### Parallel Processing
```javascript
const urls = ['url1', 'url2', 'url3'];
const browser = await puppeteer.launch();

const results = await Promise.all(
  urls.map(async (url) => {
    const page = await browser.newPage();
    try {
      await page.goto(url);
      return await page.title();
    } finally {
      await page.close();
    }
  })
);

await browser.close();
```

### Dynamic Content Handling
```javascript
// Infinite scroll
await page.evaluate(async () => {
  await new Promise((resolve) => {
    let totalHeight = 0;
    const distance = 100;
    const timer = setInterval(() => {
      window.scrollBy(0, distance);
      totalHeight += distance;
      if (totalHeight >= document.body.scrollHeight) {
        clearInterval(timer);
        resolve();
      }
    }, 100);
  });
});
```

### Custom Events
```javascript
// Expose function to page context
await page.exposeFunction('notifyChange', (data) => {
  console.log('Data changed:', data);
});

await page.evaluate(() => {
  // Call from page context
  window.notifyChange({ status: 'complete' });
});
```
