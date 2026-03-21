# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Module: `dappco.re/go/core/webview` — Chrome DevTools Protocol client for browser automation.

## Commands

```bash
go test ./...                          # Run all tests (must pass before commit)
go test -v -run TestActionSequence_Good ./...  # Run a single test
gofmt -w .                             # Format code
go vet ./...                           # Static analysis
```

## Prerequisites

Tests and usage require a running Chrome/Chromium with `--remote-debugging-port=9222`. The package does not launch Chrome itself. Verify with `curl http://localhost:9222/json`.

```bash
# macOS
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222

# Headless (CI)
google-chrome --headless=new --remote-debugging-port=9222 --no-sandbox --disable-gpu
```

## Architecture

```
Application Code → Webview (high-level API) → CDPClient (WebSocket transport) → Chrome
```

Single flat package (`package webview`). No sub-packages.

| File | Layer | Purpose |
|------|-------|---------|
| `cdp.go` | Transport | WebSocket connection, CDP message framing, event dispatch. No browser-level logic. |
| `webview.go` | High-level API | Navigate, Click, Type, Screenshot, DOM queries, console capture. Application-facing. |
| `actions.go` | Action system | `Action` interface + concrete types (ClickAction, TypeAction, etc.) + fluent `ActionSequence` builder. |
| `console.go` | Diagnostics | `ConsoleWatcher` (filtered console capture) and `ExceptionWatcher` (JS exception tracking). |
| `angular.go` | SPA helpers | Angular-specific: Zone.js stability, router navigation, component introspection, ngModel access. |

Key patterns:
- `CDPClient.Call(ctx, method, params)` sends a CDP command and blocks for the response via a pending-channel map.
- Events flow from Chrome → `readLoop` goroutine → registered handlers (each dispatched in its own goroutine).
- `Webview` enables `Runtime`, `Page`, and `DOM` CDP domains on construction; console capture starts immediately.
- New action types: add struct + `Execute` method in `actions.go`, builder method on `ActionSequence`, `_Good` test.
- New SPA framework helpers: create `<framework>.go` following the `angular.go` pattern.

## Coding Standards

- UK English in all comments, docs, and commit messages (behaviour, colour, initialise)
- EUPL-1.2 licence header (`// SPDX-License-Identifier: EUPL-1.2`) in every Go file
- Conventional commits: `type(scope): description` (scopes: cdp, angular, console, actions)
- Co-author trailer on every commit: `Co-Authored-By: Virgil <virgil@lethean.io>`
- Test naming: `_Good` (happy path), `_Bad` (expected errors), `_Ugly` (panics/edge cases)
- Standard `testing.T` only — no test frameworks
- Wrap errors with `coreerr.E("Scope.Method", "description", err)` from `go-log`, never `fmt.Errorf`
- Protect shared state with `sync.RWMutex`; copy handler slices before calling outside lock

## Docs

- `docs/architecture.md` — CDP connection, DOM queries, console capture, Angular helpers
- `docs/development.md` — prerequisites, build/test, coding standards, adding actions
- `docs/history.md` — completed phases, known limitations, future considerations

## Forge Push

```bash
git push ssh://git@forge.lthn.ai:2223/core/go-webview.git HEAD:main
```
