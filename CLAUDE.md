# CLAUDE.md

## What This Is

Chrome DevTools Protocol (CDP) client for browser automation, testing, and scraping. Module: `forge.lthn.ai/core/go-webview`

## Commands

```bash
go test ./...          # Run all tests
go test -v -run Name   # Run single test
```

## Architecture

- `webview.New(webview.WithDebugURL("http://localhost:9222"))` connects to Chrome
- Navigation, DOM queries, console capture, screenshots, JS evaluation
- Angular-specific helpers for SPA testing

## Key API

```go
wv, _ := webview.New(webview.WithDebugURL("http://localhost:9222"))
defer wv.Close()
wv.Navigate("https://example.com")
wv.Click("#submit")
wv.Type("#input", "text")
screenshot, _ := wv.Screenshot()
```

## Coding Standards

- UK English
- `go test ./...` must pass before commit
- Conventional commits: `type(scope): description`
- Co-Author: `Co-Authored-By: Virgil <virgil@lethean.io>`
