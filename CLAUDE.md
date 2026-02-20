# CLAUDE.md

Module: `forge.lthn.ai/core/go-webview` — Chrome DevTools Protocol client for browser automation.

## Commands

```bash
go test ./...          # Run all tests (must pass before commit)
go test -v -run Name   # Run a single test
gofmt -w .             # Format code
```

## Coding Standards

- UK English in all comments, docs, and commit messages
- EUPL-1.2 licence header (`// SPDX-License-Identifier: EUPL-1.2`) in every Go file
- Conventional commits: `type(scope): description`
- Co-author trailer on every commit: `Co-Authored-By: Virgil <virgil@lethean.io>`
- Test naming: `_Good` (happy path), `_Bad` (expected errors), `_Ugly` (panics/edge cases)

## Key API

```go
wv, err := webview.New(webview.WithDebugURL("http://localhost:9222"))
defer wv.Close()
wv.Navigate("https://example.com")
wv.Click("#submit")
wv.Type("#input", "text")
screenshot, _ := wv.Screenshot()
```

## Docs

- `docs/architecture.md` — CDP connection, DOM queries, console capture, Angular helpers
- `docs/development.md` — prerequisites, build/test, coding standards, adding actions
- `docs/history.md` — completed phases, known limitations, future considerations

## Forge Push

```bash
git push ssh://git@forge.lthn.ai:2223/core/go-webview.git HEAD:main
```
