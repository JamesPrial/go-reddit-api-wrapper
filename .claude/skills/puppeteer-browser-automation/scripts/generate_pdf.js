#!/usr/bin/env node
/**
 * PDF Generator - Convert webpages to PDF with Puppeteer
 * Usage: node generate_pdf.js <url> [output_path] [options]
 * 
 * Options:
 *   --format=<A4|Letter|Legal|A3> - Paper format (default: A4)
 *   --landscape - Use landscape orientation
 *   --margin=<number> - Margin in mm for all sides (default: 10)
 *   --header=<text> - Header text
 *   --footer=<text> - Footer text
 */

const puppeteer = require('puppeteer');

async function generatePDF(url, outputPath = 'output.pdf', options = {}) {
  const browser = await puppeteer.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });

  try {
    const page = await browser.newPage();

    console.log(`Navigating to ${url}...`);
    await page.goto(url, { waitUntil: 'networkidle2' });

    const margin = options.margin || 10;
    const pdfOptions = {
      path: outputPath,
      format: options.format || 'A4',
      landscape: options.landscape || false,
      margin: {
        top: `${margin}mm`,
        right: `${margin}mm`,
        bottom: `${margin}mm`,
        left: `${margin}mm`
      },
      printBackground: true
    };

    // Add header/footer if provided
    if (options.header || options.footer) {
      pdfOptions.displayHeaderFooter = true;
      pdfOptions.headerTemplate = options.header ? 
        `<div style="font-size: 10px; width: 100%; text-align: center;">${options.header}</div>` : 
        '<div></div>';
      pdfOptions.footerTemplate = options.footer ? 
        `<div style="font-size: 10px; width: 100%; text-align: center;">${options.footer} - Page <span class="pageNumber"></span> of <span class="totalPages"></span></div>` : 
        '<div></div>';
    }

    console.log(`Generating PDF...`);
    await page.pdf(pdfOptions);

    console.log(`✓ PDF saved to ${outputPath}`);
  } catch (error) {
    console.error('Error generating PDF:', error.message);
    throw error;
  } finally {
    await browser.close();
  }
}

// CLI interface
if (require.main === module) {
  const args = process.argv.slice(2);
  const url = args[0];
  const outputPath = args[1] || 'output.pdf';
  
  if (!url) {
    console.error('Usage: node generate_pdf.js <url> [output_path] [--format=A4] [--landscape] [--margin=N] [--header=text] [--footer=text]');
    process.exit(1);
  }

  const options = {
    format: args.find(arg => arg.startsWith('--format='))?.split('=')[1] || 'A4',
    landscape: args.includes('--landscape'),
    margin: parseInt(args.find(arg => arg.startsWith('--margin='))?.split('=')[1]) || 10,
    header: args.find(arg => arg.startsWith('--header='))?.split('=')[1],
    footer: args.find(arg => arg.startsWith('--footer='))?.split('=')[1]
  };

  generatePDF(url, outputPath, options)
    .then(() => process.exit(0))
    .catch(() => process.exit(1));
}

module.exports = generatePDF;
