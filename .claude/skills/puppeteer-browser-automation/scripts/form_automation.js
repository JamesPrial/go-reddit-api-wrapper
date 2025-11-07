#!/usr/bin/env node
/**
 * Form Automation - Fill and submit forms using Puppeteer
 * Usage: node form_automation.js <url> <form_data.json>
 * 
 * Form data JSON format:
 * {
 *   "fields": [
 *     {"selector": "#username", "value": "user@example.com", "type": "text"},
 *     {"selector": "#password", "value": "password123", "type": "text"},
 *     {"selector": "#remember", "type": "click"},
 *     {"selector": "select#country", "value": "US", "type": "select"}
 *   ],
 *   "submit": "#submit-button",
 *   "waitAfterSubmit": 2000
 * }
 */

const puppeteer = require('puppeteer');
const fs = require('fs');

async function fillForm(url, formData) {
  const browser = await puppeteer.launch({
    headless: false, // Set to true for production
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });

  try {
    const page = await browser.newPage();

    console.log(`Navigating to ${url}...`);
    await page.goto(url, { waitUntil: 'networkidle2' });

    // Fill form fields
    for (const field of formData.fields) {
      console.log(`Processing field: ${field.selector}`);
      
      await page.waitForSelector(field.selector, { timeout: 5000 });
      
      switch (field.type) {
        case 'text':
        case 'email':
        case 'password':
          await page.type(field.selector, field.value);
          break;
        
        case 'click':
        case 'checkbox':
        case 'radio':
          await page.click(field.selector);
          break;
        
        case 'select':
          await page.select(field.selector, field.value);
          break;
        
        case 'clear':
          await page.evaluate((sel) => {
            document.querySelector(sel).value = '';
          }, field.selector);
          break;
        
        default:
          console.warn(`Unknown field type: ${field.type}`);
      }
      
      // Small delay between fields for natural behavior
      await page.waitForTimeout(100);
    }

    // Submit form if selector provided
    if (formData.submit) {
      console.log(`Submitting form via: ${formData.submit}`);
      await page.click(formData.submit);
      
      // Wait after submission
      const waitTime = formData.waitAfterSubmit || 2000;
      await page.waitForTimeout(waitTime);
    }

    console.log('✓ Form completed successfully');
    
    // Optional: take screenshot of result
    if (formData.screenshot) {
      await page.screenshot({ path: formData.screenshot });
      console.log(`✓ Screenshot saved to ${formData.screenshot}`);
    }

    return { success: true, url: page.url() };
  } catch (error) {
    console.error('Error filling form:', error.message);
    throw error;
  } finally {
    await browser.close();
  }
}

// CLI interface
if (require.main === module) {
  const args = process.argv.slice(2);
  const url = args[0];
  const formDataFile = args[1];
  
  if (!url || !formDataFile) {
    console.error('Usage: node form_automation.js <url> <form_data.json>');
    process.exit(1);
  }

  try {
    const formData = JSON.parse(fs.readFileSync(formDataFile, 'utf8'));
    fillForm(url, formData)
      .then(result => {
        console.log('Result:', result);
        process.exit(0);
      })
      .catch(() => process.exit(1));
  } catch (error) {
    console.error('Error reading form data:', error.message);
    process.exit(1);
  }
}

module.exports = fillForm;
