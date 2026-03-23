---
title: API Contract
description: Extracted exported API contract for go-webview with signatures and test coverage notes.
---

# API Contract

This inventory covers the current exported surface of `dappco.re/go/core/webview`.

Coverage notes:
- Coverage is based on `webview_test.go`.
- `Indirect via ...` means the symbol is only exercised through another exported API or helper path.
- `None` means no evidence was found in the current test file.

| Kind | Name | Signature | Description | Test coverage |
| --- | --- | --- | --- | --- |
| Function | `FormatConsoleOutput` | `func FormatConsoleOutput(messages []ConsoleMessage) string` | FormatConsoleOutput formats console messages for display. | `TestFormatConsoleOutput_Good`, `TestFormatConsoleOutput_Good_Empty`. |
| Function | `GetVersion` | `func GetVersion(debugURL string) (map[string]string, error)` | GetVersion returns Chrome version information. | None in `webview_test.go`. |
| Function | `ListTargetsAll` | `func ListTargetsAll(debugURL string) iter.Seq[TargetInfo]` | ListTargetsAll returns an iterator over all available targets. | None in `webview_test.go`. |
| Type | `Action` | `type Action interface { Execute(ctx context.Context, wv *Webview) error }` | Action represents a browser action that can be performed. | Indirect via `TestActionSequence_Good`, `TestWaitAction_Good_ContextCancelled`, and `TestWaitAction_Good_ShortWait`. |
| Method | `Action.Execute` | `Execute(ctx context.Context, wv *Webview) error` | Runs an action against a Webview within the caller's context. | Indirect via `TestWaitAction_Good_ContextCancelled` and `TestWaitAction_Good_ShortWait`. |
| Type | `ActionSequence` | `type ActionSequence struct { /* unexported fields */ }` | ActionSequence represents a sequence of actions to execute. | `TestActionSequence_Good`. |
| Function | `NewActionSequence` | `func NewActionSequence() *ActionSequence` | NewActionSequence creates a new action sequence. | `TestActionSequence_Good`. |
| Method | `ActionSequence.Add` | `func (s *ActionSequence) Add(action Action) *ActionSequence` | Add adds an action to the sequence. | Indirect via `TestActionSequence_Good` builder chaining. |
| Method | `ActionSequence.Click` | `func (s *ActionSequence) Click(selector string) *ActionSequence` | Click adds a click action. | `TestActionSequence_Good`. |
| Method | `ActionSequence.Execute` | `func (s *ActionSequence) Execute(ctx context.Context, wv *Webview) error` | Execute executes all actions in the sequence. | None in `webview_test.go`. |
| Method | `ActionSequence.Navigate` | `func (s *ActionSequence) Navigate(url string) *ActionSequence` | Navigate adds a navigate action. | `TestActionSequence_Good`. |
| Method | `ActionSequence.Type` | `func (s *ActionSequence) Type(selector, text string) *ActionSequence` | Type adds a type action. | `TestActionSequence_Good`. |
| Method | `ActionSequence.Wait` | `func (s *ActionSequence) Wait(d time.Duration) *ActionSequence` | Wait adds a wait action. | `TestActionSequence_Good`. |
| Method | `ActionSequence.WaitForSelector` | `func (s *ActionSequence) WaitForSelector(selector string) *ActionSequence` | WaitForSelector adds a wait for selector action. | `TestActionSequence_Good`. |
| Type | `AngularHelper` | `type AngularHelper struct { /* unexported fields */ }` | AngularHelper provides Angular-specific testing utilities. | None in `webview_test.go`. |
| Function | `NewAngularHelper` | `func NewAngularHelper(wv *Webview) *AngularHelper` | NewAngularHelper creates a new Angular helper for the webview. | None in `webview_test.go`. |
| Method | `AngularHelper.CallComponentMethod` | `func (ah *AngularHelper) CallComponentMethod(selector, methodName string, args ...any) (any, error)` | CallComponentMethod calls a method on an Angular component. | None in `webview_test.go`. |
| Method | `AngularHelper.DispatchEvent` | `func (ah *AngularHelper) DispatchEvent(selector, eventName string, detail any) error` | DispatchEvent dispatches a custom event on an element. | None in `webview_test.go`. |
| Method | `AngularHelper.GetComponentProperty` | `func (ah *AngularHelper) GetComponentProperty(selector, propertyName string) (any, error)` | GetComponentProperty gets a property from an Angular component. | None in `webview_test.go`. |
| Method | `AngularHelper.GetNgModel` | `func (ah *AngularHelper) GetNgModel(selector string) (any, error)` | GetNgModel gets the value of an ngModel-bound input. | None in `webview_test.go`. |
| Method | `AngularHelper.GetRouterState` | `func (ah *AngularHelper) GetRouterState() (*AngularRouterState, error)` | GetRouterState returns the current Angular router state. | None in `webview_test.go`. |
| Method | `AngularHelper.GetService` | `func (ah *AngularHelper) GetService(serviceName string) (any, error)` | GetService gets an Angular service by token name. | None in `webview_test.go`. |
| Method | `AngularHelper.NavigateByRouter` | `func (ah *AngularHelper) NavigateByRouter(path string) error` | NavigateByRouter navigates using Angular Router. | None in `webview_test.go`. |
| Method | `AngularHelper.SetComponentProperty` | `func (ah *AngularHelper) SetComponentProperty(selector, propertyName string, value any) error` | SetComponentProperty sets a property on an Angular component. | None in `webview_test.go`. |
| Method | `AngularHelper.SetNgModel` | `func (ah *AngularHelper) SetNgModel(selector string, value any) error` | SetNgModel sets the value of an ngModel-bound input. | None in `webview_test.go`. |
| Method | `AngularHelper.SetTimeout` | `func (ah *AngularHelper) SetTimeout(d time.Duration)` | SetTimeout sets the default timeout for Angular operations. | None in `webview_test.go`. |
| Method | `AngularHelper.TriggerChangeDetection` | `func (ah *AngularHelper) TriggerChangeDetection() error` | TriggerChangeDetection manually triggers Angular change detection. | None in `webview_test.go`. |
| Method | `AngularHelper.WaitForAngular` | `func (ah *AngularHelper) WaitForAngular() error` | WaitForAngular waits for Angular to finish all pending operations. | None in `webview_test.go`. |
| Method | `AngularHelper.WaitForComponent` | `func (ah *AngularHelper) WaitForComponent(selector string) error` | WaitForComponent waits for an Angular component to be present. | None in `webview_test.go`. |
| Type | `AngularRouterState` | `type AngularRouterState struct { URL string Fragment string Params map[string]string QueryParams map[string]string }` | AngularRouterState represents Angular router state. | `TestAngularRouterState_Good`. |
| Type | `BlurAction` | `type BlurAction struct { Selector string }` | BlurAction removes focus from an element. | `TestBlurAction_Good`. |
| Method | `BlurAction.Execute` | `func (a BlurAction) Execute(ctx context.Context, wv *Webview) error` | Execute removes focus from the element. | None in `webview_test.go`. |
| Type | `BoundingBox` | `type BoundingBox struct { X float64 Y float64 Width float64 Height float64 }` | BoundingBox represents the bounding rectangle of an element. | `TestBoundingBox_Good`; also nested in `TestElementInfo_Good`. |
| Type | `CDPClient` | `type CDPClient struct { /* unexported fields */ }` | CDPClient handles communication with Chrome DevTools Protocol via WebSocket. | None in `webview_test.go`. |
| Function | `NewCDPClient` | `func NewCDPClient(debugURL string) (*CDPClient, error)` | NewCDPClient creates a new CDP client connected to the given debug URL. | Indirect error-path coverage via `TestNew_Bad_InvalidDebugURL`. |
| Method | `CDPClient.Call` | `func (c *CDPClient) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)` | Call sends a CDP method call and waits for the response. | None in `webview_test.go`. |
| Method | `CDPClient.Close` | `func (c *CDPClient) Close() error` | Close closes the CDP connection. | None in `webview_test.go`. |
| Method | `CDPClient.CloseTab` | `func (c *CDPClient) CloseTab() error` | CloseTab closes the current tab (target). | None in `webview_test.go`. |
| Method | `CDPClient.DebugURL` | `func (c *CDPClient) DebugURL() string` | DebugURL returns the debug HTTP URL. | None in `webview_test.go`. |
| Method | `CDPClient.NewTab` | `func (c *CDPClient) NewTab(url string) (*CDPClient, error)` | NewTab creates a new browser tab and returns a new CDPClient connected to it. | None in `webview_test.go`. |
| Method | `CDPClient.OnEvent` | `func (c *CDPClient) OnEvent(method string, handler func(map[string]any))` | OnEvent registers a handler for CDP events. | None in `webview_test.go`. |
| Method | `CDPClient.Send` | `func (c *CDPClient) Send(method string, params map[string]any) error` | Send sends a fire-and-forget CDP message (no response expected). | None in `webview_test.go`. |
| Method | `CDPClient.WebSocketURL` | `func (c *CDPClient) WebSocketURL() string` | WebSocketURL returns the WebSocket URL being used. | None in `webview_test.go`. |
| Type | `CheckAction` | `type CheckAction struct { Selector string Checked bool }` | CheckAction checks or unchecks a checkbox. | `TestCheckAction_Good`. |
| Method | `CheckAction.Execute` | `func (a CheckAction) Execute(ctx context.Context, wv *Webview) error` | Execute checks/unchecks the checkbox. | None in `webview_test.go`. |
| Type | `ClearAction` | `type ClearAction struct { Selector string }` | ClearAction clears the value of an input element. | `TestClearAction_Good`. |
| Method | `ClearAction.Execute` | `func (a ClearAction) Execute(ctx context.Context, wv *Webview) error` | Execute clears the input value. | None in `webview_test.go`. |
| Type | `ClickAction` | `type ClickAction struct { Selector string }` | ClickAction represents a click action. | `TestClickAction_Good`. |
| Method | `ClickAction.Execute` | `func (a ClickAction) Execute(ctx context.Context, wv *Webview) error` | Execute performs the click action. | None in `webview_test.go`. |
| Type | `ConsoleFilter` | `type ConsoleFilter struct { Type string Pattern string }` | ConsoleFilter filters console messages. | `TestConsoleWatcherFilter_Good`, `TestConsoleWatcherFilteredMessages_Good`. |
| Type | `ConsoleHandler` | `type ConsoleHandler func(msg ConsoleMessage)` | ConsoleHandler is called when a matching console message is received. | Indirect via `TestConsoleWatcherHandler_Good`. |
| Type | `ConsoleMessage` | `type ConsoleMessage struct { Type string Text string Timestamp time.Time URL string Line int Column int }` | ConsoleMessage represents a captured console log message. | `TestConsoleMessage_Good`; also used by console watcher tests. |
| Type | `ConsoleWatcher` | `type ConsoleWatcher struct { /* unexported fields */ }` | ConsoleWatcher provides advanced console message watching capabilities. | `TestConsoleWatcherFilter_Good`, `TestConsoleWatcherCounts_Good`, `TestConsoleWatcherAddMessage_Good`, `TestConsoleWatcherHandler_Good`, `TestConsoleWatcherFilteredMessages_Good`. |
| Function | `NewConsoleWatcher` | `func NewConsoleWatcher(wv *Webview) *ConsoleWatcher` | NewConsoleWatcher creates a new console watcher for the webview. | None in `webview_test.go`. |
| Method | `ConsoleWatcher.AddFilter` | `func (cw *ConsoleWatcher) AddFilter(filter ConsoleFilter)` | AddFilter adds a filter to the watcher. | `TestConsoleWatcherFilter_Good`. |
| Method | `ConsoleWatcher.AddHandler` | `func (cw *ConsoleWatcher) AddHandler(handler ConsoleHandler)` | AddHandler adds a handler for console messages. | `TestConsoleWatcherHandler_Good`. |
| Method | `ConsoleWatcher.Clear` | `func (cw *ConsoleWatcher) Clear()` | Clear clears all captured messages. | `TestConsoleWatcherCounts_Good`. |
| Method | `ConsoleWatcher.ClearFilters` | `func (cw *ConsoleWatcher) ClearFilters()` | ClearFilters removes all filters. | `TestConsoleWatcherFilter_Good`. |
| Method | `ConsoleWatcher.Count` | `func (cw *ConsoleWatcher) Count() int` | Count returns the number of captured messages. | `TestConsoleWatcherCounts_Good`. |
| Method | `ConsoleWatcher.ErrorCount` | `func (cw *ConsoleWatcher) ErrorCount() int` | ErrorCount returns the number of error messages. | `TestConsoleWatcherCounts_Good`. |
| Method | `ConsoleWatcher.Errors` | `func (cw *ConsoleWatcher) Errors() []ConsoleMessage` | Errors returns all error messages. | `TestConsoleWatcherCounts_Good`. |
| Method | `ConsoleWatcher.ErrorsAll` | `func (cw *ConsoleWatcher) ErrorsAll() iter.Seq[ConsoleMessage]` | ErrorsAll returns an iterator over all error messages. | Indirect via `ConsoleWatcher.Errors()` in `TestConsoleWatcherCounts_Good`. |
| Method | `ConsoleWatcher.FilteredMessages` | `func (cw *ConsoleWatcher) FilteredMessages() []ConsoleMessage` | FilteredMessages returns messages matching the current filters. | `TestConsoleWatcherFilteredMessages_Good`. |
| Method | `ConsoleWatcher.FilteredMessagesAll` | `func (cw *ConsoleWatcher) FilteredMessagesAll() iter.Seq[ConsoleMessage]` | FilteredMessagesAll returns an iterator over messages matching the current filters. | Indirect via `ConsoleWatcher.FilteredMessages()` in `TestConsoleWatcherFilteredMessages_Good`. |
| Method | `ConsoleWatcher.HasErrors` | `func (cw *ConsoleWatcher) HasErrors() bool` | HasErrors returns true if there are any error messages. | `TestConsoleWatcherCounts_Good`. |
| Method | `ConsoleWatcher.Messages` | `func (cw *ConsoleWatcher) Messages() []ConsoleMessage` | Messages returns all captured messages. | None in `webview_test.go`. |
| Method | `ConsoleWatcher.MessagesAll` | `func (cw *ConsoleWatcher) MessagesAll() iter.Seq[ConsoleMessage]` | MessagesAll returns an iterator over all captured messages. | None in `webview_test.go`. |
| Method | `ConsoleWatcher.SetLimit` | `func (cw *ConsoleWatcher) SetLimit(limit int)` | SetLimit sets the maximum number of messages to retain. | None in `webview_test.go`. |
| Method | `ConsoleWatcher.WaitForError` | `func (cw *ConsoleWatcher) WaitForError(ctx context.Context) (*ConsoleMessage, error)` | WaitForError waits for an error message. | None in `webview_test.go`. |
| Method | `ConsoleWatcher.WaitForMessage` | `func (cw *ConsoleWatcher) WaitForMessage(ctx context.Context, filter ConsoleFilter) (*ConsoleMessage, error)` | WaitForMessage waits for a message matching the filter. | None in `webview_test.go`. |
| Method | `ConsoleWatcher.Warnings` | `func (cw *ConsoleWatcher) Warnings() []ConsoleMessage` | Warnings returns all warning messages. | `TestConsoleWatcherCounts_Good`. |
| Method | `ConsoleWatcher.WarningsAll` | `func (cw *ConsoleWatcher) WarningsAll() iter.Seq[ConsoleMessage]` | WarningsAll returns an iterator over all warning messages. | Indirect via `ConsoleWatcher.Warnings()` in `TestConsoleWatcherCounts_Good`. |
| Type | `DoubleClickAction` | `type DoubleClickAction struct { Selector string }` | DoubleClickAction double-clicks an element. | `TestDoubleClickAction_Good`. |
| Method | `DoubleClickAction.Execute` | `func (a DoubleClickAction) Execute(ctx context.Context, wv *Webview) error` | Execute double-clicks the element. | None in `webview_test.go`. |
| Type | `ElementInfo` | `type ElementInfo struct { NodeID int TagName string Attributes map[string]string InnerHTML string InnerText string BoundingBox *BoundingBox }` | ElementInfo represents information about a DOM element. | `TestElementInfo_Good`. |
| Type | `ExceptionInfo` | `type ExceptionInfo struct { Text string LineNumber int ColumnNumber int URL string StackTrace string Timestamp time.Time }` | ExceptionInfo represents information about a JavaScript exception. | `TestExceptionInfo_Good`; also used by `TestExceptionWatcher_Good`. |
| Type | `ExceptionWatcher` | `type ExceptionWatcher struct { /* unexported fields */ }` | ExceptionWatcher watches for JavaScript exceptions. | `TestExceptionWatcher_Good`. |
| Function | `NewExceptionWatcher` | `func NewExceptionWatcher(wv *Webview) *ExceptionWatcher` | NewExceptionWatcher creates a new exception watcher. | None in `webview_test.go`. |
| Method | `ExceptionWatcher.AddHandler` | `func (ew *ExceptionWatcher) AddHandler(handler func(ExceptionInfo))` | AddHandler adds a handler for exceptions. | None in `webview_test.go`. |
| Method | `ExceptionWatcher.Clear` | `func (ew *ExceptionWatcher) Clear()` | Clear clears all captured exceptions. | `TestExceptionWatcher_Good`. |
| Method | `ExceptionWatcher.Count` | `func (ew *ExceptionWatcher) Count() int` | Count returns the number of exceptions. | `TestExceptionWatcher_Good`. |
| Method | `ExceptionWatcher.Exceptions` | `func (ew *ExceptionWatcher) Exceptions() []ExceptionInfo` | Exceptions returns all captured exceptions. | `TestExceptionWatcher_Good`. |
| Method | `ExceptionWatcher.ExceptionsAll` | `func (ew *ExceptionWatcher) ExceptionsAll() iter.Seq[ExceptionInfo]` | ExceptionsAll returns an iterator over all captured exceptions. | Indirect via `ExceptionWatcher.Exceptions()` in `TestExceptionWatcher_Good`. |
| Method | `ExceptionWatcher.HasExceptions` | `func (ew *ExceptionWatcher) HasExceptions() bool` | HasExceptions returns true if there are any exceptions. | `TestExceptionWatcher_Good`. |
| Method | `ExceptionWatcher.WaitForException` | `func (ew *ExceptionWatcher) WaitForException(ctx context.Context) (*ExceptionInfo, error)` | WaitForException waits for an exception to be thrown. | None in `webview_test.go`. |
| Type | `FocusAction` | `type FocusAction struct { Selector string }` | FocusAction focuses an element. | `TestFocusAction_Good`. |
| Method | `FocusAction.Execute` | `func (a FocusAction) Execute(ctx context.Context, wv *Webview) error` | Execute focuses the element. | None in `webview_test.go`. |
| Type | `HoverAction` | `type HoverAction struct { Selector string }` | HoverAction hovers over an element. | `TestHoverAction_Good`. |
| Method | `HoverAction.Execute` | `func (a HoverAction) Execute(ctx context.Context, wv *Webview) error` | Execute hovers over the element. | None in `webview_test.go`. |
| Type | `NavigateAction` | `type NavigateAction struct { URL string }` | NavigateAction represents a navigation action. | `TestNavigateAction_Good`. |
| Method | `NavigateAction.Execute` | `func (a NavigateAction) Execute(ctx context.Context, wv *Webview) error` | Execute performs the navigate action. | None in `webview_test.go`. |
| Type | `Option` | `type Option func(*Webview) error` | Option configures a Webview instance. | Used in `TestWithTimeout_Good`, `TestWithConsoleLimit_Good`, and `TestNew_Bad_InvalidDebugURL`. |
| Function | `WithConsoleLimit` | `func WithConsoleLimit(limit int) Option` | WithConsoleLimit sets the maximum number of console messages to retain. | `TestWithConsoleLimit_Good`. |
| Function | `WithDebugURL` | `func WithDebugURL(url string) Option` | WithDebugURL sets the Chrome DevTools debugging URL. | Indirect error-path coverage via `TestNew_Bad_InvalidDebugURL`. |
| Function | `WithTimeout` | `func WithTimeout(d time.Duration) Option` | WithTimeout sets the default timeout for operations. | `TestWithTimeout_Good`. |
| Type | `PressKeyAction` | `type PressKeyAction struct { Key string }` | PressKeyAction presses a key. | `TestPressKeyAction_Good`. |
| Method | `PressKeyAction.Execute` | `func (a PressKeyAction) Execute(ctx context.Context, wv *Webview) error` | Execute presses the key. | None in `webview_test.go`. |
| Type | `RemoveAttributeAction` | `type RemoveAttributeAction struct { Selector string Attribute string }` | RemoveAttributeAction removes an attribute from an element. | `TestRemoveAttributeAction_Good`. |
| Method | `RemoveAttributeAction.Execute` | `func (a RemoveAttributeAction) Execute(ctx context.Context, wv *Webview) error` | Execute removes the attribute. | None in `webview_test.go`. |
| Type | `RightClickAction` | `type RightClickAction struct { Selector string }` | RightClickAction right-clicks an element. | `TestRightClickAction_Good`. |
| Method | `RightClickAction.Execute` | `func (a RightClickAction) Execute(ctx context.Context, wv *Webview) error` | Execute right-clicks the element. | None in `webview_test.go`. |
| Type | `ScrollAction` | `type ScrollAction struct { X int Y int }` | ScrollAction represents a scroll action. | `TestScrollAction_Good`. |
| Method | `ScrollAction.Execute` | `func (a ScrollAction) Execute(ctx context.Context, wv *Webview) error` | Execute performs the scroll action. | None in `webview_test.go`. |
| Type | `ScrollIntoViewAction` | `type ScrollIntoViewAction struct { Selector string }` | ScrollIntoViewAction scrolls an element into view. | `TestScrollIntoViewAction_Good`. |
| Method | `ScrollIntoViewAction.Execute` | `func (a ScrollIntoViewAction) Execute(ctx context.Context, wv *Webview) error` | Execute scrolls the element into view. | None in `webview_test.go`. |
| Type | `SelectAction` | `type SelectAction struct { Selector string Value string }` | SelectAction selects an option in a select element. | `TestSelectAction_Good`. |
| Method | `SelectAction.Execute` | `func (a SelectAction) Execute(ctx context.Context, wv *Webview) error` | Execute selects the option. | None in `webview_test.go`. |
| Type | `SetAttributeAction` | `type SetAttributeAction struct { Selector string Attribute string Value string }` | SetAttributeAction sets an attribute on an element. | `TestSetAttributeAction_Good`. |
| Method | `SetAttributeAction.Execute` | `func (a SetAttributeAction) Execute(ctx context.Context, wv *Webview) error` | Execute sets the attribute. | None in `webview_test.go`. |
| Type | `SetValueAction` | `type SetValueAction struct { Selector string Value string }` | SetValueAction sets the value of an input element. | `TestSetValueAction_Good`. |
| Method | `SetValueAction.Execute` | `func (a SetValueAction) Execute(ctx context.Context, wv *Webview) error` | Execute sets the value. | None in `webview_test.go`. |
| Type | `TargetInfo` | `type TargetInfo struct { ID string Type string Title string URL string WebSocketDebuggerURL string }` | TargetInfo represents Chrome DevTools target information. | `TestTargetInfo_Good`. |
| Function | `ListTargets` | `func ListTargets(debugURL string) ([]TargetInfo, error)` | ListTargets returns all available targets. | None in `webview_test.go`. |
| Type | `TypeAction` | `type TypeAction struct { Selector string Text string }` | TypeAction represents a typing action. | `TestTypeAction_Good`. |
| Method | `TypeAction.Execute` | `func (a TypeAction) Execute(ctx context.Context, wv *Webview) error` | Execute performs the type action. | None in `webview_test.go`. |
| Type | `WaitAction` | `type WaitAction struct { Duration time.Duration }` | WaitAction represents a wait action. | `TestWaitAction_Good`, `TestWaitAction_Good_ContextCancelled`, `TestWaitAction_Good_ShortWait`. |
| Method | `WaitAction.Execute` | `func (a WaitAction) Execute(ctx context.Context, wv *Webview) error` | Execute performs the wait action. | `TestWaitAction_Good_ContextCancelled`, `TestWaitAction_Good_ShortWait`. |
| Type | `WaitForSelectorAction` | `type WaitForSelectorAction struct { Selector string }` | WaitForSelectorAction represents waiting for a selector. | `TestWaitForSelectorAction_Good`. |
| Method | `WaitForSelectorAction.Execute` | `func (a WaitForSelectorAction) Execute(ctx context.Context, wv *Webview) error` | Execute waits for the selector to appear. | None in `webview_test.go`. |
| Type | `Webview` | `type Webview struct { /* unexported fields */ }` | Webview represents a connection to a Chrome DevTools Protocol endpoint. | Structural coverage in `TestWithTimeout_Good`, `TestWithConsoleLimit_Good`, and `TestAddConsoleMessage_Good`; no public-method test. |
| Function | `New` | `func New(opts ...Option) (*Webview, error)` | New creates a new Webview instance with the given options. | `TestNew_Bad_NoDebugURL`, `TestNew_Bad_InvalidDebugURL`. |
| Method | `Webview.ClearConsole` | `func (wv *Webview) ClearConsole()` | ClearConsole clears captured console messages. | None in `webview_test.go`. |
| Method | `Webview.Click` | `func (wv *Webview) Click(selector string) error` | Click clicks on an element matching the selector. | None in `webview_test.go`. |
| Method | `Webview.Close` | `func (wv *Webview) Close() error` | Close closes the Webview connection. | None in `webview_test.go`. |
| Method | `Webview.DragAndDrop` | `func (wv *Webview) DragAndDrop(sourceSelector, targetSelector string) error` | DragAndDrop performs a drag and drop operation. | None in `webview_test.go`. |
| Method | `Webview.Evaluate` | `func (wv *Webview) Evaluate(script string) (any, error)` | Evaluate executes JavaScript and returns the result. | None in `webview_test.go`. |
| Method | `Webview.GetConsole` | `func (wv *Webview) GetConsole() []ConsoleMessage` | GetConsole returns captured console messages. | None in `webview_test.go`. |
| Method | `Webview.GetConsoleAll` | `func (wv *Webview) GetConsoleAll() iter.Seq[ConsoleMessage]` | GetConsoleAll returns an iterator over captured console messages. | None in `webview_test.go`. |
| Method | `Webview.GetHTML` | `func (wv *Webview) GetHTML(selector string) (string, error)` | GetHTML returns the outer HTML of an element or the whole document. | None in `webview_test.go`. |
| Method | `Webview.GetTitle` | `func (wv *Webview) GetTitle() (string, error)` | GetTitle returns the current page title. | None in `webview_test.go`. |
| Method | `Webview.GetURL` | `func (wv *Webview) GetURL() (string, error)` | GetURL returns the current page URL. | None in `webview_test.go`. |
| Method | `Webview.GoBack` | `func (wv *Webview) GoBack() error` | GoBack navigates back in history. | None in `webview_test.go`. |
| Method | `Webview.GoForward` | `func (wv *Webview) GoForward() error` | GoForward navigates forward in history. | None in `webview_test.go`. |
| Method | `Webview.Navigate` | `func (wv *Webview) Navigate(url string) error` | Navigate navigates to the specified URL. | None in `webview_test.go`. |
| Method | `Webview.QuerySelector` | `func (wv *Webview) QuerySelector(selector string) (*ElementInfo, error)` | QuerySelector finds an element by CSS selector and returns its information. | None in `webview_test.go`. |
| Method | `Webview.QuerySelectorAll` | `func (wv *Webview) QuerySelectorAll(selector string) ([]*ElementInfo, error)` | QuerySelectorAll finds all elements matching the selector. | None in `webview_test.go`. |
| Method | `Webview.QuerySelectorAllAll` | `func (wv *Webview) QuerySelectorAllAll(selector string) iter.Seq[*ElementInfo]` | QuerySelectorAllAll returns an iterator over all elements matching the selector. | None in `webview_test.go`. |
| Method | `Webview.Reload` | `func (wv *Webview) Reload() error` | Reload reloads the current page. | None in `webview_test.go`. |
| Method | `Webview.Screenshot` | `func (wv *Webview) Screenshot() ([]byte, error)` | Screenshot captures a screenshot and returns it as PNG bytes. | None in `webview_test.go`. |
| Method | `Webview.SetUserAgent` | `func (wv *Webview) SetUserAgent(userAgent string) error` | SetUserAgent sets the user agent string. | None in `webview_test.go`. |
| Method | `Webview.SetViewport` | `func (wv *Webview) SetViewport(width, height int) error` | SetViewport sets the viewport size. | None in `webview_test.go`. |
| Method | `Webview.Type` | `func (wv *Webview) Type(selector, text string) error` | Type types text into an element matching the selector. | None in `webview_test.go`. |
| Method | `Webview.UploadFile` | `func (wv *Webview) UploadFile(selector string, filePaths []string) error` | UploadFile uploads a file to a file input element. | None in `webview_test.go`. |
| Method | `Webview.WaitForSelector` | `func (wv *Webview) WaitForSelector(selector string) error` | WaitForSelector waits for an element matching the selector to appear. | None in `webview_test.go`. |
