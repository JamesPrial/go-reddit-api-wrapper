# MCP Tools Reference

## Core Automation Tools

### browser_navigate
Navigate to a URL
- Parameters:
  - `url` (string): The URL to navigate to

### browser_click
Perform click on element
- Parameters:
  - `element` (string): Human-readable element description
  - `ref` (string): Exact element reference from page snapshot
  - `doubleClick` (boolean, optional): Perform double click
  - `button` (string, optional): 'left', 'right', or 'middle' (default: 'left')
  - `modifiers` (array, optional): Modifier keys to press

### browser_type
Type text into editable element
- Parameters:
  - `element` (string): Human-readable element description
  - `ref` (string): Exact element reference from page snapshot
  - `text` (string): Text to type
  - `submit` (boolean, optional): Press Enter after typing
  - `slowly` (boolean, optional): Type one character at a time

### browser_fill_form
Fill multiple form fields at once
- Parameters:
  - `fields` (array): Array of field objects with element, ref, and text

### browser_select_option
Select option in dropdown
- Parameters:
  - `element` (string): Human-readable element description
  - `ref` (string): Exact element reference
  - `values` (array): Values to select (single or multiple)

### browser_hover
Hover over element
- Parameters:
  - `element` (string): Human-readable element description
  - `ref` (string): Exact element reference

### browser_drag
Perform drag and drop
- Parameters:
  - `startElement` (string): Source element description
  - `startRef` (string): Source element reference
  - `endElement` (string): Target element description
  - `endRef` (string): Target element reference

### browser_press_key
Press a keyboard key
- Parameters:
  - `key` (string): Key name (e.g., 'ArrowLeft', 'Enter', 'a')

### browser_file_upload
Upload files
- Parameters:
  - `paths` (array, optional): Absolute file paths (omit to cancel chooser)

### browser_handle_dialog
Handle browser dialog
- Parameters:
  - `accept` (boolean): Whether to accept dialog
  - `promptText` (string, optional): Text for prompt dialog

### browser_wait_for
Wait for condition or time
- Parameters:
  - `time` (number, optional): Seconds to wait
  - `text` (string, optional): Text to wait to appear
  - `textGone` (string, optional): Text to wait to disappear

## Navigation Tools

### browser_navigate_back
Go back to previous page
- Parameters: None

### browser_resize
Resize browser window
- Parameters:
  - `width` (number): Window width
  - `height` (number): Window height

## Information Tools

### browser_snapshot
Capture accessibility snapshot of current page
- Parameters: None
- Returns: Structured page snapshot for interaction

### browser_take_screenshot
Take screenshot of page or element
- Parameters:
  - `type` (string, optional): 'png' or 'jpeg' (default: 'png')
  - `filename` (string, optional): Save filename
  - `element` (string, optional): Element description for element screenshot
  - `ref` (string, optional): Element reference for element screenshot
  - `fullPage` (boolean, optional): Capture full scrollable page

### browser_console_messages
Get console messages
- Parameters: None
- Returns: All console messages since page load

### browser_network_requests
List network requests
- Parameters: None
- Returns: All network requests since page load

## JavaScript Execution

### browser_evaluate
Evaluate JavaScript in page context
- Parameters:
  - `function` (string): JavaScript function code
  - `element` (string, optional): Element description
  - `ref` (string, optional): Element reference
- Examples:
  ```javascript
  // Page context
  "() => { return document.title; }"
  
  // Element context
  "(element) => { return element.textContent; }"
  ```

## Tab Management (requires --caps=tabs)

### browser_tabs
Manage browser tabs
- Parameters:
  - `action` (string): 'list', 'create', 'close', or 'select'
  - `index` (number, optional): Tab index for close/select

## Browser Lifecycle

### browser_close
Close the browser page
- Parameters: None

### browser_install (requires --caps=install)
Install browser specified in config
- Parameters: None
- Use when: Browser not installed error occurs

## PDF Generation (requires --caps=pdf)

### browser_pdf_save
Save page as PDF
- Parameters:
  - `filename` (string, optional): PDF filename (default: page-{timestamp}.pdf)

## Coordinate-Based Tools (requires --caps=vision)

### browser_mouse_click_xy
Click at specific coordinates
- Parameters:
  - `element` (string): Element description for permission
  - `x` (number): X coordinate
  - `y` (number): Y coordinate

### browser_mouse_move_xy
Move mouse to coordinates
- Parameters:
  - `element` (string): Element description
  - `x` (number): X coordinate
  - `y` (number): Y coordinate

### browser_mouse_drag_xy
Drag mouse between coordinates
- Parameters:
  - `element` (string): Element description
  - `startX` (number): Start X coordinate
  - `startY` (number): Start Y coordinate
  - `endX` (number): End X coordinate
  - `endY` (number): End Y coordinate

## Tracing Tools (requires --caps=tracing)

### browser_start_tracing
Start Playwright trace recording
- Parameters: None

### browser_stop_tracing
Stop trace recording
- Parameters: None

## Usage Notes

### Element References
- Most tools require both `element` (description) and `ref` (exact reference)
- Get references from `browser_snapshot` output
- Descriptions are for permission/clarity, refs ensure exact targeting

### Accessibility-First
- Playwright MCP uses accessibility tree, not visual analysis
- No screenshots needed for automation
- Fast, deterministic, LLM-friendly

### Read-Only vs Modifying
Some tools are read-only (safe, no state change):
- browser_snapshot
- browser_console_messages
- browser_network_requests
- browser_take_screenshot
- browser_hover
- browser_resize
- browser_navigate_back
- browser_wait_for
- browser_close

Others modify page state:
- browser_navigate
- browser_click
- browser_type
- browser_fill_form
- browser_select_option
- browser_drag
- browser_press_key
- browser_file_upload
- browser_handle_dialog
- browser_evaluate

## Example Workflow

```javascript
// 1. Navigate
await call('browser_navigate', { url: 'https://example.com' });

// 2. Get page snapshot
const snapshot = await call('browser_snapshot');

// 3. Find element ref from snapshot
const inputRef = findElementRef(snapshot, 'search input');

// 4. Type into element
await call('browser_type', {
  element: 'search input',
  ref: inputRef,
  text: 'playwright',
  submit: true
});

// 5. Wait for results
await call('browser_wait_for', { text: 'results' });

// 6. Take screenshot
await call('browser_take_screenshot', {
  filename: 'results.png'
});
```

## References

- Full API: https://github.com/microsoft/playwright-mcp
- Playwright docs: https://playwright.dev/docs/api/class-page
