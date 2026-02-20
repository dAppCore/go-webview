# Architecture

Module: `forge.lthn.ai/core/go-webview`

## Overview

go-webview is a Chrome DevTools Protocol (CDP) client for browser automation, testing, and scraping. It provides a high-level Go API over the low-level CDP WebSocket protocol, connecting to an externally managed Chrome or Chromium instance running with the remote debugging port enabled.

The package does not launch Chrome itself. The caller is responsible for starting a Chrome process with `--remote-debugging-port=9222` before constructing a `Webview`.

---

## Package Structure

| File | Responsibility |
|------|---------------|
| `webview.go` | `Webview` struct, public API, navigation, DOM, screenshot, JS evaluation |
| `cdp.go` | `CDPClient` — WebSocket transport, message framing, event dispatch |
| `actions.go` | `Action` interface, concrete action types, `ActionSequence` builder |
| `console.go` | `ConsoleWatcher`, `ExceptionWatcher`, log formatting |
| `angular.go` | `AngularHelper` — SPA-specific helpers for Angular 2+ and AngularJS 1.x |

---

## CDP Connection

### Initialisation

`NewCDPClient(debugURL string)` connects to Chrome's HTTP endpoint:

1. Issues `GET {debugURL}/json` to retrieve the list of available targets (tabs/pages).
2. Selects the first target with `type == "page"` that has a `webSocketDebuggerUrl`.
3. If no page target exists, calls `GET {debugURL}/json/new` to create one.
4. Upgrades the connection to WebSocket using `github.com/gorilla/websocket`.
5. Starts a background `readLoop` goroutine on the connection.

### Message Protocol

CDP uses JSON-framed messages over WebSocket. The client distinguishes two message kinds:

- **Commands** — sent by the client with an integer `id`. Chrome responds with a matching `id` and a `result` or `error` field.
- **Events** — sent by Chrome without an `id`. They carry a `method` name and a `params` map.

The `CDPClient` maintains a `pending` map of `id -> chan *cdpResponse`. When `Call()` sends a command it registers a channel, then blocks on that channel until the matching response arrives or the context expires.

Events are dispatched to zero or more registered handlers via `OnEvent(method, handler)`. Each handler is called in its own goroutine so it cannot block the read loop.

### Connection Lifecycle

```
New(WithDebugURL(...))
  └── NewCDPClient(url)
        ├── HTTP GET /json          (target discovery)
        ├── websocket.Dial(wsURL)   (WebSocket upgrade)
        └── go readLoop()           (background goroutine)

wv.Close()
  └── cancel()                     (signals readLoop to stop)
      └── CDPClient.Close()
            ├── <-done              (waits for readLoop to finish)
            └── conn.Close()        (closes WebSocket)
```

---

## Webview Struct

```go
type Webview struct {
    mu           sync.RWMutex
    client       *CDPClient
    ctx          context.Context
    cancel       context.CancelFunc
    timeout      time.Duration      // default 30s
    consoleLogs  []ConsoleMessage
    consoleLimit int                // default 1000
}
```

`New()` accepts functional options:

| Option | Effect |
|--------|--------|
| `WithDebugURL(url)` | Required. Connects to Chrome at the given HTTP debug endpoint. |
| `WithTimeout(d)` | Overrides the default 30-second operation timeout. |
| `WithConsoleLimit(n)` | Maximum console messages to retain in memory (default 1000). |

On construction, `New()` enables three CDP domains — `Runtime`, `Page`, and `DOM` — and registers a handler for `Runtime.consoleAPICalled` events to begin console capture immediately.

---

## Navigation

`Navigate(url string) error` calls `Page.navigate` then polls `document.readyState` via `Runtime.evaluate` at 100 ms intervals until the value is `"complete"` or the context deadline is exceeded.

`Reload()`, `GoBack()`, and `GoForward()` follow the same pattern: issue a CDP command then call `waitForLoad`.

`waitForSelector(ctx, selector)` polls `document.querySelector(selector)` at 100 ms intervals.

---

## DOM Queries

DOM queries follow a two-step pattern:

1. Call `DOM.getDocument` to obtain the root node ID.
2. Call `DOM.querySelector` or `DOM.querySelectorAll` with that node ID and the CSS selector string.

For each matching node, `getElementInfo` calls:
- `DOM.describeNode` — tag name and attribute list (flat alternating key/value array)
- `DOM.getBoxModel` — bounding rectangle from the `content` quad

The returned `ElementInfo` carries:

```go
type ElementInfo struct {
    NodeID      int
    TagName     string
    Attributes  map[string]string
    InnerHTML   string
    InnerText   string
    BoundingBox *BoundingBox       // nil if element has no layout box
}
```

---

## Click and Type

### Click

`click(ctx, selector)` resolves the element's bounding box, computes the centre point, then dispatches `Input.dispatchMouseEvent` for `mousePressed` then `mouseReleased`. If the element has no bounding box (e.g. a hidden element), it falls back to evaluating `document.querySelector(selector)?.click()`.

### Type

`typeText(ctx, selector, text)` first focuses the element via JavaScript, then dispatches `Input.dispatchKeyEvent` with `type: "keyDown"` and `type: "keyUp"` for each character in the string individually.

`PressKeyAction` handles named keys (Enter, Tab, Escape, Backspace, Delete, arrow keys, Home, End, Page Up, Page Down) by mapping them to their CDP virtual key codes and code strings.

---

## Console Capture

Console capture is enabled in `New()` by subscribing to `Runtime.consoleAPICalled` events.

### Basic Capture (Webview)

The `Webview` itself accumulates messages in a slice guarded by `sync.RWMutex`. When the buffer reaches `consoleLimit`, the oldest 100 messages are dropped.

```go
msgs := wv.GetConsole()   // returns a copy
wv.ClearConsole()
```

### ConsoleWatcher

`ConsoleWatcher` (constructed via `NewConsoleWatcher(wv)`) registers its own handler on the same `Runtime.consoleAPICalled` event. It adds filtering and reactive capabilities:

- `AddFilter(ConsoleFilter)` — filter by message type and/or text pattern
- `AddHandler(ConsoleHandler)` — callback invoked for each incoming message (outside the write lock)
- `WaitForMessage(ctx, filter)` — blocks until a matching message arrives
- `WaitForError(ctx)` — convenience wrapper for `type == "error"`
- `Errors()`, `Warnings()`, `HasErrors()`, `ErrorCount()`

### ExceptionWatcher

`ExceptionWatcher` subscribes to `Runtime.exceptionThrown` events and captures unhandled JavaScript exceptions with full stack traces. It exposes the same reactive pattern as `ConsoleWatcher`: `AddHandler`, `WaitForException`, `HasExceptions`.

---

## Screenshots

`Screenshot()` calls `Page.captureScreenshot` with `format: "png"`. Chrome returns the image as a base64-encoded string in the `data` field of the response. The method decodes this and returns raw PNG bytes.

---

## JavaScript Evaluation

`evaluate(ctx, script)` calls `Runtime.evaluate` with `returnByValue: true`. The result is extracted from `result.result.value`. If `result.exceptionDetails` is present, the error description is returned as a Go error.

`Evaluate(script string) (any, error)` is the public wrapper that applies the default timeout.

`GetURL()` and `GetTitle()` are thin wrappers that evaluate `window.location.href` and `document.title` respectively.

`GetHTML(selector string)` evaluates `outerHTML` on the matched element, or `document.documentElement.outerHTML` when the selector is empty.

---

## Action System

The `Action` interface has a single method:

```go
type Action interface {
    Execute(ctx context.Context, wv *Webview) error
}
```

Concrete action types cover: `Click`, `Type`, `Navigate`, `Wait`, `WaitForSelector`, `Scroll`, `ScrollIntoView`, `Focus`, `Blur`, `Clear`, `Select`, `Check`, `Hover`, `DoubleClick`, `RightClick`, `PressKey`, `SetAttribute`, `RemoveAttribute`, `SetValue`.

`ActionSequence` provides a fluent builder:

```go
err := NewActionSequence().
    Navigate("https://example.com").
    WaitForSelector("#login-form").
    Type("#email", "user@example.com").
    Type("#password", "secret").
    Click("#submit").
    Execute(ctx, wv)
```

`Execute` runs actions sequentially and returns the index and error of the first failure.

### File Upload and Drag-and-Drop

`UploadFile(selector, filePaths)` uses `DOM.setFileInputFiles` on the node ID of the resolved file input element.

`DragAndDrop(sourceSelector, targetSelector)` dispatches `mousePressed`, `mouseMoved`, and `mouseReleased` events between the centre points of the two elements.

---

## Angular Helpers

`AngularHelper` (constructed via `NewAngularHelper(wv)`) provides SPA-specific utilities. All methods accept the `AngularHelper.timeout` deadline (default 30 s).

### Application Detection

`isAngularApp` checks for Angular 2+ via `window.getAllAngularRootElements`, the `[ng-version]` attribute, or `window.ng.probe`. It also checks for AngularJS 1.x via `window.angular.element`.

### Zone.js Stability

`WaitForAngular()` waits for Zone.js to report stability by checking `zone.isStable` and subscribing to `zone.onStable`. If the injector-based approach fails (production builds without debug info), it falls back to polling `window.Zone.current._inner._hasPendingMicrotasks` and `_hasPendingMacrotasks` at 50 ms intervals.

### Router Integration

`NavigateByRouter(path)` obtains the `Router` service from the Angular injector and calls `router.navigateByUrl(path)`, then waits for Zone.js stability.

`GetRouterState()` returns an `AngularRouterState` with the current URL, fragment, route params, and query params.

### Component Introspection

`GetComponentProperty(selector, property)` and `SetComponentProperty(selector, property, value)` access component instances via `window.ng.probe(element).componentInstance`. After setting a property, `ApplicationRef.tick()` is called to trigger change detection.

`CallComponentMethod(selector, method, args...)` invokes a method on the component instance and triggers change detection.

`GetService(name)` retrieves a named service from the root injector and returns a JSON-serialisable representation.

### ngModel

`GetNgModel(selector)` reads the current value of an ngModel-bound input. `SetNgModel(selector, value)` writes the value, fires `input` and `change` events, and triggers `ApplicationRef.tick()`.

---

## Multi-Tab Support

`CDPClient.NewTab(url)` calls `GET {debugURL}/json/new?{url}` and returns a new `CDPClient` connected to the WebSocket of the newly created tab. Each tab has its own independent read loop and event handler registry, so console events and other notifications are tab-scoped.

`CDPClient.CloseTab()` calls `Browser.close` on the tab's CDP session.

`ListTargets(debugURL)` and `GetVersion(debugURL)` are package-level utilities that query the HTTP endpoint without requiring an active WebSocket connection.

---

## Emulation

`SetViewport(width, height int)` calls `Emulation.setDeviceMetricsOverride` with `deviceScaleFactor: 1` and `mobile: false`.

`SetUserAgent(ua string)` calls `Emulation.setUserAgentOverride`.

---

## Thread Safety

- `CDPClient` uses `sync.RWMutex` for WebSocket writes and `sync.Mutex` for the pending-response map. Event handler registration uses a separate `sync.RWMutex`.
- `Webview` uses `sync.RWMutex` for its console log slice.
- `ConsoleWatcher` and `ExceptionWatcher` use `sync.RWMutex` for their message and handler slices. Handlers are copied before being called so they execute outside the write lock.
