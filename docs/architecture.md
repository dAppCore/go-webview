---
title: Architecture
description: Internals of go-webview -- CDP connection, message protocol, DOM queries, console capture, action system, and Angular helpers.
---

# Architecture

This document describes how `go-webview` works internally. It covers the CDP connection lifecycle, message protocol, DOM query mechanics, input simulation, console capture, the action system, Angular helpers, and thread safety.

## High-Level Data Flow

```
Application Code
      |
      v
  Webview  (high-level API: Navigate, Click, Type, Screenshot, ...)
      |
      v
  CDPClient  (WebSocket transport, message framing, event dispatch)
      |
      v
  Chrome / Chromium  (running with --remote-debugging-port=9222)
```

The application interacts with `Webview` methods. Each method constructs a CDP command, passes it to `CDPClient.Call()`, which serialises it as JSON over a WebSocket connection to Chrome. Chrome processes the command and returns a JSON response. Events (console messages, exceptions, navigation state changes) flow in the opposite direction: Chrome pushes them over the WebSocket, the `CDPClient` read loop dispatches them to registered handlers.

## CDP Connection

### Initialisation

`NewCDPClient(debugURL string)` connects to Chrome's HTTP endpoint in four steps:

1. Issues `GET {debugURL}/json` to retrieve the list of available targets (tabs/pages).
2. Selects the first target with `type == "page"` that has a `webSocketDebuggerUrl`.
3. If no page target exists, calls `GET {debugURL}/json/new` to create one.
4. Upgrades the connection to WebSocket using `github.com/gorilla/websocket` and starts a background `readLoop` goroutine.

### Message Protocol

CDP uses JSON-framed messages over WebSocket. The client distinguishes two message kinds:

- **Commands** -- sent by the client with an integer `id`. Chrome responds with a matching `id` and a `result` or `error` field.
- **Events** -- sent by Chrome without an `id`. They carry a `method` name and a `params` map.

The `CDPClient` maintains a `pending` map of `id -> chan *cdpResponse`. When `Call()` sends a command it registers a channel, then blocks on that channel until the matching response arrives or the context expires.

Events are dispatched to zero or more registered handlers via `OnEvent(method, handler)`. Each handler is called in its own goroutine so it cannot block the read loop.

### Connection Lifecycle

```
New(WithDebugURL(...))
  +-- NewCDPClient(url)
        |-- HTTP GET /json          (target discovery)
        |-- websocket.Dial(wsURL)   (WebSocket upgrade)
        +-- go readLoop()           (background goroutine)

wv.Close()
  +-- cancel()                     (signals readLoop to stop)
      +-- CDPClient.Close()
            |-- <-done              (waits for readLoop to finish)
            +-- conn.Close()        (closes WebSocket)
```

## Key Types

### CDPClient

```go
type CDPClient struct {
    conn     *websocket.Conn
    debugURL string
    wsURL    string
    msgID    atomic.Int64                    // monotonic command ID
    pending  map[int64]chan *cdpResponse      // awaiting responses
    handlers map[string][]func(map[string]any) // event subscribers
    ctx      context.Context
    cancel   context.CancelFunc
    done     chan struct{}
}
```

The core transport layer. All WebSocket reads happen in the `readLoop` goroutine. All writes are serialised through a `sync.RWMutex`. The `pending` map and `handlers` map each have their own dedicated mutexes.

### Webview

```go
type Webview struct {
    client       *CDPClient
    ctx          context.Context
    cancel       context.CancelFunc
    timeout      time.Duration      // default 30s
    consoleLogs  []ConsoleMessage
    consoleLimit int                // default 1000
}
```

The high-level API surface. Constructed via `New()` with functional options. On construction, it enables three CDP domains -- `Runtime`, `Page`, and `DOM` -- and registers a handler for `Runtime.consoleAPICalled` events so console capture begins immediately.

### ConsoleMessage

```go
type ConsoleMessage struct {
    Type      string    // log, warn, error, info, debug
    Text      string    // message text
    Timestamp time.Time
    URL       string    // source URL
    Line      int       // source line number
    Column    int       // source column number
}
```

### ElementInfo

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

### BoundingBox

```go
type BoundingBox struct {
    X      float64
    Y      float64
    Width  float64
    Height float64
}
```

## Navigation

`Navigate(url string) error` calls `Page.navigate` then polls `document.readyState` via `Runtime.evaluate` at 100 ms intervals until the value is `"complete"` or the context deadline is exceeded.

`Reload()`, `GoBack()`, and `GoForward()` follow the same pattern: issue a CDP command then call `waitForLoad`.

`waitForSelector(ctx, selector)` polls `document.querySelector(selector)` at 100 ms intervals until the element exists or the context expires.

## DOM Queries

DOM queries follow a two-step pattern:

1. Call `DOM.getDocument` to obtain the root node ID.
2. Call `DOM.querySelector` or `DOM.querySelectorAll` with that node ID and the CSS selector string.

For each matching node, `getElementInfo` calls:
- `DOM.describeNode` -- tag name and attribute list (flat alternating key/value array)
- `DOM.getBoxModel` -- bounding rectangle from the `content` quad

`QuerySelectorAllAll(selector)` returns an `iter.Seq[*ElementInfo]` iterator for lazy consumption of results.

## Click and Type

### Click

`click(ctx, selector)` resolves the element's bounding box, computes the centre point, then dispatches `Input.dispatchMouseEvent` for `mousePressed` then `mouseReleased`. If the element has no bounding box (e.g. a hidden element), it falls back to evaluating `document.querySelector(selector)?.click()`.

### Type

`typeText(ctx, selector, text)` first focuses the element via JavaScript, then dispatches `Input.dispatchKeyEvent` with `type: "keyDown"` and `type: "keyUp"` for each character in the string individually.

`PressKeyAction` handles named keys (Enter, Tab, Escape, Backspace, Delete, arrow keys, Home, End, Page Up, Page Down) by mapping them to their CDP virtual key codes and code strings.

## Console Capture

Console capture is enabled in `New()` by subscribing to `Runtime.consoleAPICalled` events.

### Basic Capture (Webview)

The `Webview` itself accumulates messages in a slice guarded by `sync.RWMutex`. When the buffer reaches `consoleLimit`, the oldest 100 messages are dropped.

```go
msgs := wv.GetConsole()       // returns a collected slice
wv.ClearConsole()

// Or iterate lazily
for msg := range wv.GetConsoleAll() {
    fmt.Println(msg.Text)
}
```

### ConsoleWatcher

`ConsoleWatcher` (constructed via `NewConsoleWatcher(wv)`) registers its own handler on the same `Runtime.consoleAPICalled` event. It adds filtering and reactive capabilities:

- `AddFilter(ConsoleFilter)` -- filter by message type and/or text pattern (substring match)
- `AddHandler(ConsoleHandler)` -- callback invoked for each incoming message (outside the write lock)
- `WaitForMessage(ctx, filter)` -- blocks until a matching message arrives
- `WaitForError(ctx)` -- convenience wrapper for `type == "error"`
- `Errors()`, `Warnings()`, `HasErrors()`, `ErrorCount()`
- `FilteredMessages()` / `FilteredMessagesAll()` -- returns messages matching all active filters

### ExceptionWatcher

`ExceptionWatcher` subscribes to `Runtime.exceptionThrown` events and captures unhandled JavaScript exceptions with full stack traces:

```go
type ExceptionInfo struct {
    Text         string
    LineNumber   int
    ColumnNumber int
    URL          string
    StackTrace   string
    Timestamp    time.Time
}
```

It exposes the same reactive pattern as `ConsoleWatcher`: `AddHandler`, `WaitForException`, `HasExceptions`, `Count`.

### FormatConsoleOutput

The package-level `FormatConsoleOutput(messages)` function formats a slice of `ConsoleMessage` into human-readable lines with timestamp, level prefix (`[ERROR]`, `[WARN]`, `[INFO]`, `[DEBUG]`, `[LOG]`), and message text.

## Screenshots

`Screenshot()` calls `Page.captureScreenshot` with `format: "png"`. Chrome returns the image as a base64-encoded string in the `data` field of the response. The method decodes this and returns raw PNG bytes.

## JavaScript Evaluation

`evaluate(ctx, script)` calls `Runtime.evaluate` with `returnByValue: true`. The result is extracted from `result.result.value`. If `result.exceptionDetails` is present, the error description is returned as a Go error.

`Evaluate(script string) (any, error)` is the public wrapper that applies the default timeout.

Convenience wrappers:

| Method | JavaScript evaluated |
|--------|---------------------|
| `GetURL()` | `window.location.href` |
| `GetTitle()` | `document.title` |
| `GetHTML(selector)` | `document.querySelector(selector)?.outerHTML` (or `document.documentElement.outerHTML` when selector is empty) |

## Action System

The `Action` interface has a single method:

```go
type Action interface {
    Execute(ctx context.Context, wv *Webview) error
}
```

### Concrete Action Types

| Type | Description |
|------|-------------|
| `ClickAction` | Click an element by CSS selector |
| `TypeAction` | Type text into a focused element |
| `NavigateAction` | Navigate to a URL and wait for load |
| `WaitAction` | Wait for a fixed duration |
| `WaitForSelectorAction` | Wait for an element to appear |
| `ScrollAction` | Scroll to absolute coordinates |
| `ScrollIntoViewAction` | Scroll an element into view smoothly |
| `FocusAction` | Focus an element |
| `BlurAction` | Remove focus from an element |
| `ClearAction` | Clear an input's value, firing `input` and `change` events |
| `SelectAction` | Select a value in a `<select>` element |
| `CheckAction` | Check or uncheck a checkbox |
| `HoverAction` | Hover over an element |
| `DoubleClickAction` | Double-click an element |
| `RightClickAction` | Right-click (context menu) an element |
| `PressKeyAction` | Press a named key (Enter, Tab, Escape, etc.) |
| `SetAttributeAction` | Set an HTML attribute on an element |
| `RemoveAttributeAction` | Remove an HTML attribute from an element |
| `SetValueAction` | Set an input's value, firing `input` and `change` events |

### ActionSequence

`ActionSequence` provides a fluent builder. Actions are executed sequentially; the first failure halts the sequence and returns the action index with the error.

```go
err := webview.NewActionSequence().
    Navigate("https://example.com").
    WaitForSelector("#login-form").
    Type("#email", "user@example.com").
    Type("#password", "secret").
    Click("#submit").
    Execute(ctx, wv)
```

### File Upload and Drag-and-Drop

These are methods on `Webview` rather than action types:

- `UploadFile(selector, filePaths)` -- uses `DOM.setFileInputFiles` on the resolved file input node
- `DragAndDrop(sourceSelector, targetSelector)` -- dispatches `mousePressed`, `mouseMoved`, and `mouseReleased` events between the centre points of two elements

## Angular Helpers

`AngularHelper` (constructed via `NewAngularHelper(wv)`) provides SPA-specific utilities for Angular 2+ applications. All methods use the helper's configurable timeout (default 30 seconds).

### Application Detection

`isAngularApp` checks for Angular by probing:
- `window.getAllAngularRootElements` (Angular 2+)
- The `[ng-version]` attribute on DOM elements
- `window.ng.probe` (Angular debug utilities)
- `window.angular.element` (AngularJS 1.x)

### Zone.js Stability

`WaitForAngular()` waits for Zone.js to report stability by checking `zone.isStable` and subscribing to `zone.onStable`. If the injector-based approach fails (production builds without debug info), it falls back to polling `window.Zone.current._inner._hasPendingMicrotasks` and `_hasPendingMacrotasks` at 50 ms intervals.

### Router Integration

- `NavigateByRouter(path)` -- obtains the `Router` service from the Angular injector, calls `router.navigateByUrl(path)`, then waits for Zone.js stability
- `GetRouterState()` -- returns an `AngularRouterState` with the current URL, fragment, route params, and query params

### Component Introspection

These methods require the Angular application to be running in debug mode (`window.ng.probe` must be available):

- `GetComponentProperty(selector, property)` -- reads a property from a component instance
- `SetComponentProperty(selector, property, value)` -- writes a property and triggers `ApplicationRef.tick()`
- `CallComponentMethod(selector, method, args...)` -- invokes a method and triggers change detection
- `GetService(name)` -- retrieves a named service from the root injector, returned as a JSON-serialisable value

### ngModel Access

- `GetNgModel(selector)` -- reads the current value of an ngModel-bound input
- `SetNgModel(selector, value)` -- writes the value, fires `input` and `change` events, and triggers `ApplicationRef.tick()`

### Other Helpers

- `TriggerChangeDetection()` -- manually triggers `ApplicationRef.tick()` across all root elements
- `WaitForComponent(selector)` -- polls until a component instance exists on the matched element
- `DispatchEvent(selector, eventName, detail)` -- dispatches a `CustomEvent` on an element

## Multi-Tab Support

`CDPClient.NewTab(url)` calls `GET {debugURL}/json/new?{url}` and returns a new `CDPClient` connected to the WebSocket of the newly created tab. Each tab has its own independent read loop and event handler registry, so console events and other notifications are tab-scoped.

`ListTargets(debugURL)` and `ListTargetsAll(debugURL)` are package-level utilities that query the HTTP endpoint without requiring an active WebSocket connection. `ListTargetsAll` returns an `iter.Seq[targetInfo]` iterator.

`GetVersion(debugURL)` returns Chrome version information as a string map.

## Emulation

- `SetViewport(width, height int)` -- calls `Emulation.setDeviceMetricsOverride` with `deviceScaleFactor: 1` and `mobile: false`
- `SetUserAgent(ua string)` -- calls `Emulation.setUserAgentOverride`

## Thread Safety

- **CDPClient** uses `sync.RWMutex` for WebSocket writes and `sync.Mutex` for the pending-response map. Event handler registration uses a separate `sync.RWMutex`.
- **Webview** uses `sync.RWMutex` for its console log slice.
- **ConsoleWatcher** and **ExceptionWatcher** use `sync.RWMutex` for their message and handler slices. Handlers are copied before being called so they execute outside the write lock.
- Event handlers registered via `OnEvent` are dispatched in separate goroutines so they cannot block the WebSocket read loop.
