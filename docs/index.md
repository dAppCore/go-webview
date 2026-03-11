---
title: go-webview
description: Chrome DevTools Protocol client for browser automation, testing, and scraping in Go.
---

# go-webview

`go-webview` is a Go package that provides browser automation via the Chrome DevTools Protocol (CDP). It connects to an externally managed Chrome or Chromium instance running with `--remote-debugging-port=9222` and exposes a high-level API for navigation, DOM queries, input simulation, screenshot capture, console monitoring, and JavaScript evaluation.

The package does not launch Chrome itself. The caller is responsible for starting the browser process before constructing a `Webview`.

**Module path:** `forge.lthn.ai/core/go-webview`
**Licence:** EUPL-1.2
**Go version:** 1.26+
**Dependencies:** `github.com/gorilla/websocket v1.5.3`

## Quick Start

Start Chrome with the remote debugging port enabled:

```bash
# macOS
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
    --remote-debugging-port=9222

# Linux
google-chrome --remote-debugging-port=9222

# Headless (suitable for CI)
google-chrome --headless=new --remote-debugging-port=9222 --no-sandbox --disable-gpu
```

Then use the package in Go:

```go
import "forge.lthn.ai/core/go-webview"

// Connect to Chrome
wv, err := webview.New(webview.WithDebugURL("http://localhost:9222"))
if err != nil {
    log.Fatal(err)
}
defer wv.Close()

// Navigate and interact
if err := wv.Navigate("https://example.com"); err != nil {
    log.Fatal(err)
}
if err := wv.Click("#submit-button"); err != nil {
    log.Fatal(err)
}
```

### Fluent Action Sequences

Chain multiple browser actions together with `ActionSequence`:

```go
err := webview.NewActionSequence().
    Navigate("https://example.com").
    WaitForSelector("#login-form").
    Type("#email", "user@example.com").
    Type("#password", "secret").
    Click("#submit").
    Execute(ctx, wv)
```

### Console Monitoring

Capture and filter browser console output:

```go
cw := webview.NewConsoleWatcher(wv)
cw.AddFilter(webview.ConsoleFilter{Type: "error"})

// ... perform browser actions ...

if cw.HasErrors() {
    for _, msg := range cw.Errors() {
        log.Printf("JS error: %s at %s:%d", msg.Text, msg.URL, msg.Line)
    }
}
```

### Screenshots

Capture the current page as PNG:

```go
png, err := wv.Screenshot()
if err != nil {
    log.Fatal(err)
}
os.WriteFile("screenshot.png", png, 0644)
```

### Angular Applications

First-class support for Angular single-page applications:

```go
ah := webview.NewAngularHelper(wv)

// Wait for Angular to stabilise
if err := ah.WaitForAngular(); err != nil {
    log.Fatal(err)
}

// Navigate using Angular Router
if err := ah.NavigateByRouter("/dashboard"); err != nil {
    log.Fatal(err)
}

// Inspect component state (debug mode only)
value, err := ah.GetComponentProperty("app-widget", "title")
```

## Package Layout

| File | Responsibility |
|------|----------------|
| `webview.go` | `Webview` struct, public API (navigate, click, type, screenshot, JS evaluation, DOM queries) |
| `cdp.go` | `CDPClient` -- WebSocket transport, CDP message framing, event dispatch, tab management |
| `actions.go` | `Action` interface, 19 concrete action types, `ActionSequence` fluent builder |
| `console.go` | `ConsoleWatcher`, `ExceptionWatcher`, console log formatting |
| `angular.go` | `AngularHelper` -- Zone.js stability, router navigation, component introspection, ngModel |
| `webview_test.go` | Unit tests for structs, options, and action building |

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithDebugURL(url)` | *(required)* | Chrome DevTools HTTP debug endpoint, e.g. `http://localhost:9222` |
| `WithTimeout(d)` | 30 seconds | Default timeout for all browser operations |
| `WithConsoleLimit(n)` | 1000 | Maximum number of console messages retained in memory |

## Further Documentation

- [Architecture](architecture.md) -- internals, data flow, CDP protocol, type reference
- [Development Guide](development.md) -- build, test, contribute, coding standards
- [Project History](history.md) -- extraction origin, completed phases, known limitations
