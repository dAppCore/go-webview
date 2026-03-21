[![Go Reference](https://pkg.go.dev/badge/dappco.re/go/core/webview.svg)](https://pkg.go.dev/dappco.re/go/core/webview)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](LICENSE.md)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)](go.mod)

# go-webview

Chrome DevTools Protocol (CDP) client for browser automation, testing, and scraping. Connects to an externally managed Chrome or Chromium instance running with `--remote-debugging-port=9222`, providing navigation, DOM queries, click and type actions, console capture, JavaScript evaluation, screenshots, multi-tab support, viewport emulation, and a fluent `ActionSequence` builder. Includes Angular-specific helpers for Zone.js stability, router navigation, component introspection, and ngModel access.

**Module**: `dappco.re/go/core/webview`
**Licence**: EUPL-1.2
**Language**: Go 1.25

## Quick Start

```go
import "dappco.re/go/core/webview"

wv, err := webview.New(webview.WithDebugURL("http://localhost:9222"))
defer wv.Close()

wv.Navigate("https://example.com")
wv.Click("#submit")
wv.Type("#input", "search term")
png, err := wv.Screenshot()

// Fluent action sequence
err = webview.NewActionSequence().
    Navigate("https://example.com").
    WaitForSelector("#login-form").
    Type("#email", "user@example.com").
    Click("#submit").
    Execute(ctx, wv)
```

## Documentation

- [Architecture](docs/architecture.md) — CDP connection, DOM queries, console capture, Angular helpers, action system
- [Development Guide](docs/development.md) — prerequisites, build, test patterns, adding actions
- [Project History](docs/history.md) — completed phases, known limitations, future considerations

## Build & Test

```bash
go test ./...
go vet ./...
go build ./...
```

## Licence

European Union Public Licence 1.2 — see [LICENCE](LICENCE) for details.
