---
name: browser-automating
description: "Use when needing browser automation, fetching web page content, extracting structured data from URLs, or encountering PMS/Jira/GitHub issue links. Triggers on BUG IDs, URLs requiring login, web scraping, or browser-based tasks."
---

# Browser Automating

Universal browser automation. Auto-detects backend, provides reusable extraction patterns.

## Backend Selection

1. **Has `browser_*` tools?** → Use opencode-browser (zero config, auto login reuse)
2. **No `browser_*` tools?** → Run `scripts/start-cdp-chrome.sh` to launch Chrome CDP
3. **Neither available?** → Recommend `bunx @different-ai/opencode-browser@latest install`

| Backend | Login Reuse | Setup |
|---------|-------------|-------|
| opencode-browser (`browser_*`) | Auto | None |
| Chrome CDP (Playwright) | Yes | `scripts/start-cdp-chrome.sh` |
| Playwright standalone | No | `npm i playwright` |

## Pattern: Extract Structured Data from Any Page

Works for PMS bugs, Jira tickets, GitHub issues, any structured web page.

### 1. Identify Target

| Input | Regex | Action |
|-------|-------|--------|
| PMS URL | `https://pms.uniontech.com/bug-view-(\d+)` | Navigate directly |
| BUG ID | `(?:BUG\|bug\|#)\s*(\d+)` | Build `https://pms.uniontech.com/bug-view-{ID}.html` |
| Jira URL | `https://jira\..*?/browse/([A-Z]+-\d+)` | Navigate directly |
| Any URL | URL match | Navigate directly |

### 2. Open & Wait

```
browser_open_tab(url=target_url)
browser_wait(ms=3000)
```

### 3. Check Login

```
browser_query(selector="input[type='password']", mode="exists")
```

If `true`: **tell user to login in the browser, then retry**. Do NOT attempt login automation.

### 4. Extract Content

```
browser_query(mode="page_text")
```

Or use `browser_snapshot()` for accessibility tree. Then extract key fields.

**Filtering rule**: Keep actionable content, discard noise.

| Keep | Discard |
|------|---------|
| Title, status, priority, severity | Operation history |
| Preconditions, steps to reproduce | Unrelated attachments |
| Actual result (the bug/error) | Chat comments |
| Expected result | Related task links |
| Screenshots of the issue | Statistics, footer |

### 5. Screenshots (if needed)

```
browser_screenshot()
```

### 6. Output Template

```markdown
## [Bug/Issue/Ticket] #ID

**Title**: [title]
**Severity/Priority**: [level]
**Status**: [status]

### Preconditions
[context needed]

### Steps to Reproduce
1. [step]
2. [step]

### Actual Result
[what happened]

### Expected Result
[what should happen]

### Screenshots
[if any]
```

## CDP Backend

For detailed CDP launch, connection, troubleshooting, and security: **See [reference/cdp.md](reference/cdp.md)**

Quick start:
```bash
./scripts/start-cdp-chrome.sh [url]
```

Connect via Playwright:
```typescript
import { chromium } from 'playwright';
const browser = await chromium.connectOverCDP('http://localhost:9223');
const context = browser.contexts()[0] || await browser.newContext();
const page = context.pages()[0] || await context.newPage();
```

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Hardcoding selectors from one site version | Use `page_text`/`snapshot` first, adapt to actual page |
| Trying to auto-login | Prompt user to login manually, then retry |
| Closing CDP browser after use | Keep it alive for session reuse |
| Using Playwright standalone when CDP available | Prefer CDP to reuse login state |
| Skipping backend detection under time pressure | Always detect first — wrong backend wastes more time |
| Blindly trying webfetch on auth-required pages | Auth pages require browser with login state |
