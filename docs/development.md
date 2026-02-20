# Development Guide

## Prerequisites

### Go

Go 1.25 or later is required. The module path is `forge.lthn.ai/core/go-webview`.

### Chrome or Chromium

A running Chrome or Chromium instance with the remote debugging port enabled is required for any tests or usage that exercises the CDP connection. The package does not launch Chrome itself.

Start Chrome with the remote debugging port:

```bash
# macOS
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
    --remote-debugging-port=9222

# Linux
google-chrome --remote-debugging-port=9222

# Headless (suitable for CI)
google-chrome \
    --headless=new \
    --remote-debugging-port=9222 \
    --no-sandbox \
    --disable-gpu
```

The default debug URL is `http://localhost:9222`. Verify it is reachable by visiting `http://localhost:9222/json` in a browser or with `curl`.

### Dependencies

The only runtime dependency is `github.com/gorilla/websocket v1.5.3`, declared in `go.mod`. Fetch dependencies with:

```bash
go mod download
```

---

## Build and Test

### Running Tests

```bash
go test ./...
```

Tests must pass before committing. There are currently no build tags that gate tests behind a Chrome connection; the integration tests in `webview_test.go` that require a live browser (`TestNew_Bad_InvalidDebugURL`) will fail gracefully because they assert that the error is non-nil when connecting to an unavailable port.

```bash
# Run a specific test
go test -v -run TestActionSequence_Good ./...

# Run all tests with verbose output
go test -v ./...
```

### Test Naming Convention

Tests follow the `_Good`, `_Bad`, `_Ugly` suffix pattern, consistent with the broader Core Go ecosystem:

- `_Good` — happy path, verifies correct behaviour under valid input.
- `_Bad` — expected error conditions, verifies that errors are returned and have the correct shape.
- `_Ugly` — panic/edge cases, unexpected or degenerate inputs.

All test functions use the standard `testing.T` interface; the project does not use a test framework.

### CI Headless Tests

To add tests that exercise the full CDP stack in CI:

1. Start Chrome in headless mode in the CI job before running `go test`.
2. Serve test fixtures using `net/http/httptest` so tests do not depend on external URLs.
3. Use `WithTimeout` to set conservative deadlines appropriate for the CI environment.

---

## Code Organisation

New source files belong in the root package (`package webview`). The package is intentionally a single flat package; do not create sub-packages.

Keep separation between layers:

- **CDP transport** — `cdp.go`. Do not put browser-level logic here.
- **High-level API** — `webview.go`. Methods here should be safe to call from application code without CDP knowledge.
- **Action types** — `actions.go`. Add new action types here; keep each action focused on a single interaction.
- **Diagnostics** — `console.go`. Console and exception capture live here.
- **SPA helpers** — `angular.go`. Framework-specific helpers belong here or in a new file named after the framework (e.g. `react.go`, `vue.go`).

---

## Coding Standards

### Language

UK English throughout all source comments, documentation, commit messages, and identifiers where natural language appears. Use "colour", "organisation", "behaviour", "initialise", not their American equivalents.

### Formatting

Standard `gofmt` formatting is mandatory. Run before committing:

```bash
gofmt -w .
```

### Types and Error Handling

- All exported functions must have Go doc comments.
- Use `fmt.Errorf("context: %w", err)` for error wrapping so callers can use `errors.Is` and `errors.As`.
- Return errors; do not panic in library code.
- Use `context.Context` for all operations that involve I/O or waiting so callers can impose deadlines.

### Concurrency

- Protect shared mutable state with `sync.RWMutex` (read lock for reads, write lock for writes).
- Do not call handlers or callbacks while holding a lock. Copy the slice of handlers, release the lock, then call them.
- CDP WebSocket writes are serialised with a dedicated mutex in `CDPClient`; do not write to `conn` directly from outside `cdp.go`.

### Licence Header

Every Go source file must begin with:

```go
// SPDX-License-Identifier: EUPL-1.2
```

The project is licenced under the European Union Public Licence 1.2 (EUPL-1.2).

---

## Commit Guidelines

Use conventional commits:

```
type(scope): description
```

Common types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`.

Example scopes: `cdp`, `angular`, `console`, `actions`.

All commits must include the co-author trailer:

```
Co-Authored-By: Virgil <virgil@lethean.io>
```

Full example:

```
feat(console): add ExceptionWatcher with stack trace capture

Subscribes to Runtime.exceptionThrown events and exposes a reactive
WaitForException API consistent with ConsoleWatcher.

Co-Authored-By: Virgil <virgil@lethean.io>
```

---

## Adding a New Action Type

1. Define a struct in `actions.go` with exported fields for the action's parameters.
2. Implement `Execute(ctx context.Context, wv *Webview) error` on the struct.
3. Add a builder method on `ActionSequence` that appends the new action.
4. Add a `_Good` test in `webview_test.go` that verifies the struct fields are set correctly.

Example:

```go
// SubmitAction submits a form element.
type SubmitAction struct {
    Selector string
}

func (a SubmitAction) Execute(ctx context.Context, wv *Webview) error {
    script := fmt.Sprintf("document.querySelector(%q)?.submit()", a.Selector)
    _, err := wv.evaluate(ctx, script)
    return err
}

func (s *ActionSequence) Submit(selector string) *ActionSequence {
    return s.Add(SubmitAction{Selector: selector})
}
```

---

## Adding a New Angular Helper

Add methods to `AngularHelper` in `angular.go`. Follow the established pattern:

1. Obtain a context with the helper's timeout using `context.WithTimeout`.
2. Build the JavaScript as a self-invoking function expression `(function() { ... })()`.
3. Call `ah.wv.evaluate(ctx, script)` to execute it.
4. After state-modifying operations, call `TriggerChangeDetection()` or inline `appRef.tick()`.
5. For polling-based waits, use a `time.NewTicker` at 100 ms and select over `ctx.Done()`.

---

## Forge Push

Push to the canonical remote via SSH:

```bash
git push ssh://git@forge.lthn.ai:2223/core/go-webview.git HEAD:main
```

HTTPS authentication is not available on this remote.
