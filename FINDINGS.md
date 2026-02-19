# FINDINGS.md -- go-webview

## 2026-02-19: Split from core/go (Virgil)

### Origin

Extracted from `forge.lthn.ai/core/go` `pkg/webview/` on 19 Feb 2026.

### Architecture

- Chrome DevTools Protocol (CDP) client over WebSocket
- Connects to Chrome's remote debugging port (default 9222)
- High-level API: `Navigate`, `Click`, `Type`, `QuerySelector`, `Evaluate`, `Screenshot`
- Console capture via `Runtime.consoleAPICalled` CDP events
- Multi-tab support via `Target.createTarget` / `Target.closeTarget`
- Angular-specific helpers for SPA testing workflows

### Dependencies

- `github.com/gorilla/websocket` -- WebSocket client for CDP connection

### Notes

- Requires a running Chrome instance with `--remote-debugging-port=9222`
- No headless Chrome launcher included -- the caller must start Chrome separately
