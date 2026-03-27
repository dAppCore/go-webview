# webview, **Import:** `dappco.re/go/core/webview`, **Files:** 5

## Types

### Action
Declaration: `type Action interface`

Browser action contract used by `ActionSequence`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Runs the action against the supplied `Webview` and caller-owned context.

### ActionSequence
Declaration: `type ActionSequence struct`

Represents an ordered list of `Action` values. All fields are unexported.

Methods:
- `Add(action Action) *ActionSequence`: Appends `action` to the sequence and returns the same sequence for chaining.
- `Click(selector string) *ActionSequence`: Appends `ClickAction{Selector: selector}`.
- `Execute(ctx context.Context, wv *Webview) error`: Executes actions in insertion order. The first failing action stops execution and is wrapped as `ActionSequence.Execute` with the zero-based action index.
- `Navigate(url string) *ActionSequence`: Appends `NavigateAction{URL: url}`.
- `Type(selector, text string) *ActionSequence`: Appends `TypeAction{Selector: selector, Text: text}`.
- `Wait(d time.Duration) *ActionSequence`: Appends `WaitAction{Duration: d}`.
- `WaitForSelector(selector string) *ActionSequence`: Appends `WaitForSelectorAction{Selector: selector}`.

### AngularHelper
Declaration: `type AngularHelper struct`

Angular-specific helper bound to a `Webview`. All fields are unexported. The helper stores the target `Webview` and a per-operation timeout, which defaults to 30 seconds in `NewAngularHelper`.

Methods:
- `CallComponentMethod(selector, methodName string, args ...any) (any, error)`: Looks up the Angular component instance for `selector`, verifies that `methodName` is callable, invokes it with JSON-marshalled arguments, ticks `ApplicationRef` when available, and returns the evaluated result.
- `DispatchEvent(selector, eventName string, detail any) error`: Dispatches a bubbling `CustomEvent` on the matched element. `detail` is marshalled into the page script, or `null` when omitted.
- `GetComponentProperty(selector, propertyName string) (any, error)`: Returns `componentInstance[propertyName]` for the Angular component attached to the matched element.
- `GetNgModel(selector string) (any, error)`: Returns `element.value` for `input`, `select`, and `textarea` elements, otherwise `element.value || element.textContent`. Missing elements produce `nil`.
- `GetRouterState() (*AngularRouterState, error)`: Walks Angular root elements, resolves the first available Router, and returns its URL, fragment, root params, and root query params. Only string values are copied into the returned maps.
- `GetService(serviceName string) (any, error)`: Resolves a service from the first Angular root injector and returns `JSON.parse(JSON.stringify(service))`, so only JSON-serialisable data survives.
- `NavigateByRouter(path string) error`: Resolves Angular Router from a root injector, calls `navigateByUrl(path)`, and then waits for Zone.js stability.
- `SetComponentProperty(selector, propertyName string, value any) error`: Sets `componentInstance[propertyName]` and ticks `ApplicationRef` when available.
- `SetNgModel(selector string, value any) error`: Assigns `element.value`, dispatches bubbling `input` and `change` events, and then ticks `ApplicationRef` on the first Angular root that exposes it.
- `SetTimeout(d time.Duration)`: Replaces the helper's default timeout for later Angular operations.
- `TriggerChangeDetection() error`: Tries to tick `ApplicationRef` on the first Angular root. The method only reports script-evaluation failures; a `false` result from the script is ignored.
- `WaitForAngular() error`: Verifies that the page looks like an Angular application, then waits for Zone.js stability. It first tries an async Zone-based script and falls back to polling every 50 ms until the helper timeout expires.
- `WaitForComponent(selector string) error`: Polls every 100 ms until `selector` resolves to an element with an Angular `componentInstance`, or until the helper timeout expires.

### AngularRouterState
Declaration: `type AngularRouterState struct`

Represents the Angular Router state returned by `AngularHelper.GetRouterState`.

Fields:
- `URL string`: Current router URL.
- `Fragment string`: Current URL fragment when Angular reports one.
- `Params map[string]string`: Root route parameters copied from the router state.
- `QueryParams map[string]string`: Root query parameters copied from the router state.

### BlurAction
Declaration: `type BlurAction struct`

Removes focus from an element selected by CSS.

Fields:
- `Selector string`: CSS selector passed to `document.querySelector`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Runs `document.querySelector(Selector)?.blur()` through `wv.evaluate`.

### BoundingBox
Declaration: `type BoundingBox struct`

Represents element coordinates derived from `DOM.getBoxModel`.

Fields:
- `X float64`: Left edge of the content box.
- `Y float64`: Top edge of the content box.
- `Width float64`: Width computed from the first and second X coordinates in the CDP content quad.
- `Height float64`: Height computed from the first and third Y coordinates in the CDP content quad.

### CDPClient
Declaration: `type CDPClient struct`

Low-level Chrome DevTools Protocol client backed by a single WebSocket connection. All fields are unexported.

Methods:
- `Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)`: Clones `params`, assigns a monotonically increasing message ID, writes the request, and waits for the matching CDP response, `ctx.Done()`, or client shutdown.
- `Close() error`: Cancels the client context, fails pending calls, closes the WebSocket, waits for the read loop to exit, and returns a wrapped close error only when the socket close itself fails with a non-terminal error.
- `CloseTab() error`: Extracts the current target ID from the WebSocket URL, calls `Target.closeTarget`, checks `success` when the field is present, and then closes the client.
- `DebugURL() string`: Returns the canonical debug HTTP URL stored on the client.
- `NewTab(url string) (*CDPClient, error)`: Creates a target via `/json/new`, validates the returned WebSocket debugger URL, dials it, and returns a new `CDPClient`.
- `OnEvent(method string, handler func(map[string]any))`: Registers a handler for a CDP event method name. Event dispatch clones the handler list and event params and invokes each handler in its own goroutine.
- `Send(method string, params map[string]any) error`: Clones `params` and writes a fire-and-forget CDP message without waiting for a response.
- `WebSocketURL() string`: Returns the target WebSocket URL currently in use.

### CheckAction
Declaration: `type CheckAction struct`

Synchronises a checkbox state by clicking when the current `checked` state differs from the requested value.

Fields:
- `Selector string`: CSS selector for the checkbox element.
- `Checked bool`: Desired checked state.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Evaluates a script that clicks the element only when `el.checked != Checked`.

### ClearAction
Declaration: `type ClearAction struct`

Clears the value of an input-like element.

Fields:
- `Selector string`: CSS selector for the target element.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Sets `el.value = ""` and dispatches bubbling `input` and `change` events when the element exists.

### ClickAction
Declaration: `type ClickAction struct`

Action wrapper for the `Webview` click implementation.

Fields:
- `Selector string`: CSS selector for the target element.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Delegates to `wv.click(ctx, Selector)`.

### ConsoleFilter
Declaration: `type ConsoleFilter struct`

Filter used by `ConsoleWatcher`.

Fields:
- `Type string`: Exact message-type match. The watcher compares it directly against `ConsoleMessage.Type`.
- `Pattern string`: Case-sensitive substring match against `ConsoleMessage.Text`.

### ConsoleHandler
Declaration: `type ConsoleHandler func(msg ConsoleMessage)`

Callback signature used by `ConsoleWatcher.AddHandler`.

### ConsoleMessage
Declaration: `type ConsoleMessage struct`

Represents one console message captured from `Runtime.consoleAPICalled`.

Fields:
- `Type string`: Console message type reported by CDP.
- `Text string`: Message text built by joining `args[*].value` with spaces.
- `Timestamp time.Time`: Local capture time assigned in Go with `time.Now()`.
- `URL string`: URL taken from the first stack-frame entry when present.
- `Line int`: Line number taken from the first stack-frame entry when present.
- `Column int`: Column number taken from the first stack-frame entry when present.

### ConsoleWatcher
Declaration: `type ConsoleWatcher struct`

Higher-level console log collector layered on top of `Webview`. All fields are unexported. New watchers start with an empty message buffer and a default limit of 1000 messages.

Methods:
- `AddFilter(filter ConsoleFilter)`: Appends a filter. When at least one filter exists, `FilteredMessages` and `FilteredMessagesAll` use OR semantics across filters.
- `AddHandler(handler ConsoleHandler)`: Registers a callback for future messages captured by this watcher.
- `Clear()`: Removes all stored messages.
- `ClearFilters()`: Removes every registered filter.
- `Count() int`: Returns the current number of stored messages.
- `ErrorCount() int`: Counts stored messages where `Type == "error"`.
- `Errors() []ConsoleMessage`: Returns a collected slice of error messages.
- `ErrorsAll() iter.Seq[ConsoleMessage]`: Returns an iterator that yields stored messages whose type is exactly `"error"`.
- `FilteredMessages() []ConsoleMessage`: Returns a collected slice of messages that match the current filter set, or all messages when no filters exist.
- `FilteredMessagesAll() iter.Seq[ConsoleMessage]`: Returns an iterator over the current filtered view.
- `HasErrors() bool`: Reports whether any stored message has `Type == "error"`.
- `Messages() []ConsoleMessage`: Returns a collected slice of all stored messages.
- `MessagesAll() iter.Seq[ConsoleMessage]`: Returns an iterator over the stored message buffer.
- `SetLimit(limit int)`: Replaces the retention limit used for future appends. Existing messages are only trimmed on later writes.
- `WaitForError(ctx context.Context) (*ConsoleMessage, error)`: Equivalent to `WaitForMessage(ctx, ConsoleFilter{Type: "error"})`.
- `WaitForMessage(ctx context.Context, filter ConsoleFilter) (*ConsoleMessage, error)`: Returns the first already-stored message that matches `filter`, otherwise registers a temporary handler and waits for the next matching message or `ctx.Done()`.
- `Warnings() []ConsoleMessage`: Returns a collected slice of messages whose type is exactly `"warning"`.
- `WarningsAll() iter.Seq[ConsoleMessage]`: Returns an iterator over stored messages whose type is exactly `"warning"`.

### DoubleClickAction
Declaration: `type DoubleClickAction struct`

Double-click action for an element selected by CSS.

Fields:
- `Selector string`: CSS selector for the target element.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Uses a JavaScript `dblclick` fallback when the element has no bounding box; otherwise sends two `mousePressed` and `mouseReleased` pairs with increasing `clickCount` values at the element centre.

### ElementInfo
Declaration: `type ElementInfo struct`

Represents DOM metadata returned by `Webview.querySelector` and `Webview.querySelectorAll`.

Fields:
- `NodeID int`: CDP node ID used for later DOM operations.
- `TagName string`: `nodeName` returned by `DOM.describeNode`.
- `Attributes map[string]string`: Attributes parsed from the alternating key/value array returned by CDP.
- `InnerHTML string`: Declared field for inner HTML content. The current implementation does not populate it.
- `InnerText string`: Declared field for inner text content. The current implementation does not populate it.
- `BoundingBox *BoundingBox`: Element box derived from `DOM.getBoxModel`. It is `nil` when box lookup fails.

### ExceptionInfo
Declaration: `type ExceptionInfo struct`

Represents one `Runtime.exceptionThrown` event captured by `ExceptionWatcher`.

Fields:
- `Text string`: Exception text, overridden by `exception.description` when CDP provides one.
- `LineNumber int`: Line number reported by CDP.
- `ColumnNumber int`: Column number reported by CDP.
- `URL string`: Source URL reported by CDP.
- `StackTrace string`: Stack trace formatted in Go as one `at function (url:line:column)` line per call frame.
- `Timestamp time.Time`: Local capture time assigned in Go with `time.Now()`.

### ExceptionWatcher
Declaration: `type ExceptionWatcher struct`

Collector for JavaScript exceptions emitted by the bound `Webview`. All fields are unexported.

Methods:
- `AddHandler(handler func(ExceptionInfo))`: Registers a callback for future exception events.
- `Clear()`: Removes all stored exceptions.
- `Count() int`: Returns the current number of stored exceptions.
- `Exceptions() []ExceptionInfo`: Returns a collected slice of all stored exceptions.
- `ExceptionsAll() iter.Seq[ExceptionInfo]`: Returns an iterator over the stored exception buffer.
- `HasExceptions() bool`: Reports whether any exceptions have been captured.
- `WaitForException(ctx context.Context) (*ExceptionInfo, error)`: Returns the most recently stored exception immediately when one already exists; otherwise registers a temporary handler and waits for the next exception or `ctx.Done()`.

### FocusAction
Declaration: `type FocusAction struct`

Moves focus to an element selected by CSS.

Fields:
- `Selector string`: CSS selector passed to `document.querySelector`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Runs `document.querySelector(Selector)?.focus()` through `wv.evaluate`.

### HoverAction
Declaration: `type HoverAction struct`

Moves the mouse pointer over an element.

Fields:
- `Selector string`: CSS selector for the target element.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Looks up the element, requires a non-nil bounding box, computes the box centre, and sends one `Input.dispatchMouseEvent` call with `type: "mouseMoved"`.

### NavigateAction
Declaration: `type NavigateAction struct`

Action wrapper for page navigation.

Fields:
- `URL string`: URL passed to `Page.navigate`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Calls `Page.navigate` and then waits until `document.readyState == "complete"`.

### Option
Declaration: `type Option func(*Webview) error`

Functional configuration hook applied by `New`. The exported option constructors are `WithDebugURL`, `WithTimeout`, and `WithConsoleLimit`.

### PressKeyAction
Declaration: `type PressKeyAction struct`

Key-press action implemented through `Input.dispatchKeyEvent`.

Fields:
- `Key string`: Key name or character to send.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Uses explicit CDP key metadata for common keys such as `Enter`, `Tab`, arrow keys, `Delete`, and paging keys. For any other string it sends a `keyDown` with `text: Key` followed by a bare `keyUp`.

### RemoveAttributeAction
Declaration: `type RemoveAttributeAction struct`

Removes a DOM attribute.

Fields:
- `Selector string`: CSS selector for the target element.
- `Attribute string`: Attribute name to remove.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Runs `document.querySelector(Selector)?.removeAttribute(Attribute)` through `wv.evaluate`.

### RightClickAction
Declaration: `type RightClickAction struct`

Context-click action for an element selected by CSS.

Fields:
- `Selector string`: CSS selector for the target element.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Uses a JavaScript `contextmenu` fallback when the element has no bounding box; otherwise sends `mousePressed` and `mouseReleased` with `button: "right"` at the element centre.

### ScrollAction
Declaration: `type ScrollAction struct`

Window scroll action.

Fields:
- `X int`: Horizontal scroll target passed to `window.scrollTo`.
- `Y int`: Vertical scroll target passed to `window.scrollTo`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Evaluates `window.scrollTo(X, Y)`.

### ScrollIntoViewAction
Declaration: `type ScrollIntoViewAction struct`

Scrolls a selected element into view.

Fields:
- `Selector string`: CSS selector for the target element.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Evaluates `document.querySelector(Selector)?.scrollIntoView({behavior: "smooth", block: "center"})`.

### SelectAction
Declaration: `type SelectAction struct`

Select-element value setter.

Fields:
- `Selector string`: CSS selector for the target element.
- `Value string`: Option value assigned to `el.value`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Sets `el.value` and dispatches a bubbling `change` event when the element exists.

### SetAttributeAction
Declaration: `type SetAttributeAction struct`

Sets a DOM attribute on the selected element.

Fields:
- `Selector string`: CSS selector for the target element.
- `Attribute string`: Attribute name to set.
- `Value string`: Attribute value passed to `setAttribute`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Runs `document.querySelector(Selector)?.setAttribute(Attribute, Value)` through `wv.evaluate`.

### SetValueAction
Declaration: `type SetValueAction struct`

Directly sets an input-like value.

Fields:
- `Selector string`: CSS selector for the target element.
- `Value string`: Value assigned to `el.value`.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Sets `el.value` and dispatches bubbling `input` and `change` events when the element exists.

### TargetInfo
Declaration: `type TargetInfo struct`

Represents one target entry returned by Chrome's `/json` and `/json/new` endpoints.

Fields:
- `ID string`: Chrome target ID.
- `Type string`: Target type such as `page`.
- `Title string`: Target title reported by Chrome.
- `URL string`: Target URL reported by Chrome.
- `WebSocketDebuggerURL string`: WebSocket debugger URL for the target.

### TypeAction
Declaration: `type TypeAction struct`

Action wrapper for `Webview` text entry.

Fields:
- `Selector string`: CSS selector for the target element.
- `Text string`: Text sent one rune at a time.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Delegates to `wv.typeText(ctx, Selector, Text)`.

### WaitAction
Declaration: `type WaitAction struct`

Time-based delay action.

Fields:
- `Duration time.Duration`: Delay length.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Waits for `Duration` with `time.After`, but returns `ctx.Err()` immediately if the context is cancelled first.

### WaitForSelectorAction
Declaration: `type WaitForSelectorAction struct`

Action wrapper for selector polling.

Fields:
- `Selector string`: CSS selector to wait for.

Methods:
- `Execute(ctx context.Context, wv *Webview) error`: Delegates to `wv.waitForSelector(ctx, Selector)`.

### Webview
Declaration: `type Webview struct`

High-level browser automation client built on `CDPClient`. All fields are unexported. Instances created by `New` carry a default timeout of 30 seconds and a default console retention limit of 1000 messages.

Methods:
- `ClearConsole()`: Removes all console messages stored on the `Webview`.
- `Click(selector string) error`: Creates a timeout-scoped context and delegates to the internal click path. The internal click logic queries the element, uses a JavaScript `.click()` fallback when there is no bounding box, and otherwise sends left-button press and release events at the element centre.
- `Close() error`: Cancels the `Webview` context and closes the underlying `CDPClient` when one exists.
- `DragAndDrop(sourceSelector, targetSelector string) error`: Looks up both elements, requires bounding boxes for both, then sends `mousePressed` at the source centre, `mouseMoved` to the target centre, and `mouseReleased` at the target centre.
- `Evaluate(script string) (any, error)`: Evaluates JavaScript through `Runtime.evaluate` with `returnByValue: true` and `awaitPromise: true`. When CDP reports `exceptionDetails`, the method returns a wrapped JavaScript error.
- `GetConsole() []ConsoleMessage`: Returns a collected slice of stored console messages.
- `GetConsoleAll() iter.Seq[ConsoleMessage]`: Returns an iterator over the stored console message buffer.
- `GetHTML(selector string) (string, error)`: Returns `document.documentElement.outerHTML` when `selector` is empty; otherwise returns `document.querySelector(selector)?.outerHTML || ""`.
- `GetTitle() (string, error)`: Evaluates `document.title` and requires the result to be a string.
- `GetURL() (string, error)`: Evaluates `window.location.href` and requires the result to be a string.
- `GoBack() error`: Calls `Page.goBackOrForward` with `delta: -1`.
- `GoForward() error`: Calls `Page.goBackOrForward` with `delta: 1`.
- `Navigate(url string) error`: Calls `Page.navigate` and then polls `document.readyState` every 100 ms until it becomes `"complete"` or the timeout expires.
- `QuerySelector(selector string) (*ElementInfo, error)`: Fetches the document root, runs `DOM.querySelector`, errors when the selector does not resolve, and returns `ElementInfo` for the matched node.
- `QuerySelectorAll(selector string) ([]*ElementInfo, error)`: Runs `DOM.querySelectorAll` and returns one `ElementInfo` per node ID whose metadata lookup succeeds. Nodes whose metadata fetch fails are skipped.
- `QuerySelectorAllAll(selector string) iter.Seq[*ElementInfo]`: Returns an iterator that runs `QuerySelectorAll` under the `Webview` timeout and yields each element. Errors produce an empty iterator.
- `Reload() error`: Calls `Page.reload` and then waits for `document.readyState == "complete"`.
- `Screenshot() ([]byte, error)`: Calls `Page.captureScreenshot` with `format: "png"`, decodes the returned base64 payload, and returns the PNG bytes.
- `SetUserAgent(userAgent string) error`: Calls `Emulation.setUserAgentOverride`.
- `SetViewport(width, height int) error`: Calls `Emulation.setDeviceMetricsOverride` with the supplied size, `deviceScaleFactor: 1`, and `mobile: false`.
- `Type(selector, text string) error`: Creates a timeout-scoped context and delegates to the internal typing path. The internal logic focuses the element with JavaScript, then sends one `keyDown` with `text` and one `keyUp` per rune in `text`.
- `UploadFile(selector string, filePaths []string) error`: Resolves the target node with `QuerySelector` and passes its node ID to `DOM.setFileInputFiles`.
- `WaitForSelector(selector string) error`: Polls `!!document.querySelector(selector)` every 100 ms until it becomes true or the timeout expires.

## Functions

### FormatConsoleOutput
`func FormatConsoleOutput(messages []ConsoleMessage) string`

Formats each message as `HH:MM:SS.mmm [PREFIX] text\n`. Prefixes are `[ERROR]` for `error`, `[WARN]` for `warning`, `[INFO]` for `info`, `[DEBUG]` for `debug`, and `[LOG]` for every other type.

### GetVersion
`func GetVersion(debugURL string) (map[string]string, error)`

Validates `debugURL`, requests `/json/version`, and decodes the response body into `map[string]string`.

### ListTargets
`func ListTargets(debugURL string) ([]TargetInfo, error)`

Validates `debugURL`, requests `/json`, and decodes the response body into a slice of `TargetInfo`.

### ListTargetsAll
`func ListTargetsAll(debugURL string) iter.Seq[TargetInfo]`

Iterator wrapper around `ListTargets`. When `ListTargets` fails, the iterator yields no values.

### New
`func New(opts ...Option) (*Webview, error)`

Creates a `Webview`, applies `opts` in order, requires an option that installs a `CDPClient`, enables the `Runtime`, `Page`, and `DOM` domains, and subscribes console capture. On any option or initialisation failure it cancels the context and closes the client when one was created.

### NewActionSequence
`func NewActionSequence() *ActionSequence`

Creates an empty `ActionSequence`.

### NewAngularHelper
`func NewAngularHelper(wv *Webview) *AngularHelper`

Creates an `AngularHelper` bound to `wv` with a 30-second default timeout.

### NewCDPClient
`func NewCDPClient(debugURL string) (*CDPClient, error)`

Validates that `debugURL` is an `http` or `https` DevTools root URL with no credentials, query, fragment, or non-root path. The function requests `/json`, picks the first `page` target with a debugger WebSocket URL, creates a new target via `/json/new` when none exists, validates that the WebSocket host matches the debug host, dials the socket, and starts the client's read loop.

### NewConsoleWatcher
`func NewConsoleWatcher(wv *Webview) *ConsoleWatcher`

Creates a `ConsoleWatcher`, initialises an empty message buffer with a 1000-message limit, and subscribes it to `Runtime.consoleAPICalled` events on `wv.client`.

### NewExceptionWatcher
`func NewExceptionWatcher(wv *Webview) *ExceptionWatcher`

Creates an `ExceptionWatcher`, initialises an empty exception buffer, and subscribes it to `Runtime.exceptionThrown` events on `wv.client`.

### WithConsoleLimit
`func WithConsoleLimit(limit int) Option`

Returns an `Option` that replaces `Webview.consoleLimit`. The default used by `New` is 1000.

### WithDebugURL
`func WithDebugURL(url string) Option`

Returns an `Option` that dials a `CDPClient` immediately and stores it on the `Webview`.

### WithTimeout
`func WithTimeout(d time.Duration) Option`

Returns an `Option` that replaces the default per-operation timeout used by `Webview` methods.
