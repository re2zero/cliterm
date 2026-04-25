# CDP Reference

Chrome DevTools Protocol reference for browser automation when opencode-browser is not available.

## Contents

- Launch Chrome with CDP
- Connect via Playwright
- Detect and verify CDP
- Troubleshooting
- Security

## Launch Chrome with CDP

**Recommended**: Use the bundled script:
```bash
./scripts/start-cdp-chrome.sh [url]   # default port 9223
CDP_PORT=9222 ./scripts/start-cdp-chrome.sh
```

**Manual launch**:
```bash
google-chrome \
    --remote-debugging-port=9222 \
    --remote-allow-origins=* \
    --user-data-dir="$HOME/.config/chrome-automation" \
    --no-first-run
```

### Chrome Binary Paths (Deepin/UOS)

| Source | Path |
|--------|------|
| Linglong store | `/opt/apps/cn.google.chrome-pre/files/google/chrome/google-chrome` |
| System apt | `/usr/bin/google-chrome-stable` |
| Fallback | `/usr/bin/google-chrome`, `/usr/bin/chromium` |

### Headless Mode (server)

```bash
google-chrome --headless=new --disable-gpu --no-sandbox \
    --disable-dev-shm-usage --remote-debugging-port=9222 --remote-allow-origins=*
```

## Connect via Playwright

```typescript
import { chromium } from 'playwright';

const browser = await chromium.connectOverCDP('http://localhost:9222', {
    timeout: 30000
});

// Reuse existing context (preserves login state)
const context = browser.contexts()[0] || await browser.newContext();
const page = context.pages()[0] || await context.newPage();

// Navigate
await page.goto(url, { waitUntil: 'networkidle' });

// Extract content
const content = await page.evaluate(() => document.body.innerText);
```

### Check Login Status

```typescript
const needsLogin = await page.evaluate(() => {
    const indicators = [
        'input[type="password"]',
        'input[name*="login"]',
        '.login-form',
        '#login'
    ];
    return indicators.some(sel => document.querySelector(sel));
});
```

## Detect and Verify CDP

```bash
curl -s http://localhost:9222/json/version  # verify running
curl -s http://localhost:9222/json/list       # list open tabs
```

Expected `json/version` output:
```json
{"Browser":"Chrome/xxx","webSocketDebuggerUrl":"ws://localhost:9222/..."}
```

## Troubleshooting

| Error | Cause | Fix |
|-------|-------|-----|
| ECONNREFUSED | CDP not running | Run `scripts/start-cdp-chrome.sh` |
| ETIMEDOUT | Connection timeout | Increase timeout; check firewall |
| WebSocket disconnected | Browser closed | Restart Chrome |
| Data dir locked | Instance conflict | Close other Chrome or use different `--user-data-dir` |
| Port in use | Another CDP instance | `pkill -f 'remote-debugging-port=9222'` or use different port |

## Security

- CDP binds to `127.0.0.1` only by default — keep it that way
- Restrict `--remote-allow-origins` in production (never use `*` on shared networks)
- Never use `--no-sandbox` outside isolated containers
- Use isolated `--user-data-dir` to avoid polluting main Chrome profile
- AI agents are 3x more likely to be targeted — treat CDP access as privileged
