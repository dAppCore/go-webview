// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"iter"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	core "dappco.re/go/core"
)

// ConsoleWatcher provides advanced console message watching capabilities.
type ConsoleWatcher struct {
	mu            sync.RWMutex
	wv            *Webview
	messages      []ConsoleMessage
	filters       []ConsoleFilter
	limit         int
	handlers      []consoleHandlerRegistration
	nextHandlerID atomic.Int64
}

// ConsoleFilter filters console messages.
type ConsoleFilter struct {
	Type    string // Filter by type (log, warn, error, info, debug), empty for all
	Pattern string // Filter by text pattern (substring match)
}

// ConsoleHandler is called when a matching console message is received.
type ConsoleHandler func(msg ConsoleMessage)

type consoleHandlerRegistration struct {
	id      int64
	handler ConsoleHandler
}

// NewConsoleWatcher creates a new console watcher for the webview.
func NewConsoleWatcher(wv *Webview) *ConsoleWatcher {
	cw := &ConsoleWatcher{
		wv:       wv,
		messages: make([]ConsoleMessage, 0, 1000),
		filters:  make([]ConsoleFilter, 0),
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	// Subscribe to console events from the webview's client
	wv.client.OnEvent("Runtime.consoleAPICalled", func(params map[string]any) {
		cw.handleConsoleEvent(params)
	})

	return cw
}

// normalizeConsoleType converts CDP event types to package-level values.
func normalizeConsoleType(raw string) string {
	normalized := strings.ToLower(core.Trim(core.Sprint(raw)))
	switch normalized {
	case "warning":
		return "warn"
	default:
		return normalized
	}
}

// consoleTextFromArgs extracts message text from Runtime.consoleAPICalled args.
func consoleTextFromArgs(args []any) string {
	text := core.NewBuilder()
	for i, arg := range args {
		if i > 0 {
			text.WriteString(" ")
		}
		text.WriteString(consoleArgText(arg))
	}

	return text.String()
}

func consoleArgText(arg any) string {
	remoteObj, ok := arg.(map[string]any)
	if !ok {
		return consoleValueToString(arg)
	}

	if value, ok := remoteObj["value"]; ok {
		return consoleValueToString(value)
	}

	if desc, ok := remoteObj["description"].(string); ok && desc != "" {
		return desc
	}

	if preview, ok := remoteObj["preview"].(map[string]any); ok {
		if description, ok := preview["description"].(string); ok && description != "" {
			return description
		}
	}

	if preview, ok := remoteObj["preview"].(map[string]any); ok {
		if value, ok := preview["value"].(string); ok && value != "" {
			return value
		}
	}

	if r := core.JSONMarshal(remoteObj); r.OK {
		if encoded, ok := r.Value.([]byte); ok {
			return string(encoded)
		}
	}

	return ""
}

func consoleValueToString(value any) string {
	if value == nil {
		return "null"
	}
	if valueStr, ok := value.(string); ok {
		return valueStr
	}

	if r := core.JSONMarshal(value); r.OK {
		if encoded, ok := r.Value.([]byte); ok {
			return string(encoded)
		}
	}

	return core.Sprint(value)
}

func consoleMessageTimestamp(params map[string]any) time.Time {
	timestamp, ok := params["timestamp"].(float64)
	if !ok {
		return time.Now()
	}

	seconds := int64(timestamp)
	nanoseconds := int64((timestamp - float64(seconds)) * float64(time.Second))
	return time.Unix(seconds, nanoseconds).UTC()
}

// AddFilter adds a filter to the watcher.
func (cw *ConsoleWatcher) AddFilter(filter ConsoleFilter) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.filters = append(cw.filters, filter)
}

// ClearFilters removes all filters.
func (cw *ConsoleWatcher) ClearFilters() {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.filters = cw.filters[:0]
}

// AddHandler adds a handler for console messages.
func (cw *ConsoleWatcher) AddHandler(handler ConsoleHandler) {
	cw.addHandler(handler)
}

func (cw *ConsoleWatcher) addHandler(handler ConsoleHandler) int64 {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	id := cw.nextHandlerID.Add(1)
	cw.handlers = append(cw.handlers, consoleHandlerRegistration{
		id:      id,
		handler: handler,
	})
	return id
}

func (cw *ConsoleWatcher) removeHandler(id int64) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	for i, registration := range cw.handlers {
		if registration.id == id {
			cw.handlers = slices.Delete(cw.handlers, i, i+1)
			return
		}
	}
}

// SetLimit sets the maximum number of messages to retain.
func (cw *ConsoleWatcher) SetLimit(limit int) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.limit = limit
}

// Messages returns all captured messages.
func (cw *ConsoleWatcher) Messages() []ConsoleMessage {
	return slices.Collect(cw.MessagesAll())
}

// MessagesAll returns an iterator over all captured messages.
func (cw *ConsoleWatcher) MessagesAll() iter.Seq[ConsoleMessage] {
	return func(yield func(ConsoleMessage) bool) {
		cw.mu.RLock()
		defer cw.mu.RUnlock()

		for _, msg := range cw.messages {
			if !yield(msg) {
				return
			}
		}
	}
}

// FilteredMessages returns messages matching the current filters.
func (cw *ConsoleWatcher) FilteredMessages() []ConsoleMessage {
	return slices.Collect(cw.FilteredMessagesAll())
}

// FilteredMessagesAll returns an iterator over messages matching the current filters.
func (cw *ConsoleWatcher) FilteredMessagesAll() iter.Seq[ConsoleMessage] {
	return func(yield func(ConsoleMessage) bool) {
		cw.mu.RLock()
		defer cw.mu.RUnlock()

		for _, msg := range cw.messages {
			if cw.matchesFilter(msg) {
				if !yield(msg) {
					return
				}
			}
		}
	}
}

// Errors returns all error messages.
func (cw *ConsoleWatcher) Errors() []ConsoleMessage {
	return slices.Collect(cw.ErrorsAll())
}

// ErrorsAll returns an iterator over all error messages.
func (cw *ConsoleWatcher) ErrorsAll() iter.Seq[ConsoleMessage] {
	return func(yield func(ConsoleMessage) bool) {
		cw.mu.RLock()
		defer cw.mu.RUnlock()

		for _, msg := range cw.messages {
			if normalizeConsoleType(msg.Type) == "error" {
				if !yield(msg) {
					return
				}
			}
		}
	}
}

// Warnings returns all warning messages.
func (cw *ConsoleWatcher) Warnings() []ConsoleMessage {
	return slices.Collect(cw.WarningsAll())
}

// WarningsAll returns an iterator over all warning messages.
func (cw *ConsoleWatcher) WarningsAll() iter.Seq[ConsoleMessage] {
	return func(yield func(ConsoleMessage) bool) {
		cw.mu.RLock()
		defer cw.mu.RUnlock()

		for _, msg := range cw.messages {
			if normalizeConsoleType(msg.Type) == "warn" {
				if !yield(msg) {
					return
				}
			}
		}
	}
}

// Clear clears all captured messages.
func (cw *ConsoleWatcher) Clear() {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.messages = cw.messages[:0]
}

// WaitForMessage waits for a message matching the filter.
func (cw *ConsoleWatcher) WaitForMessage(ctx context.Context, filter ConsoleFilter) (*ConsoleMessage, error) {
	// First check existing messages
	cw.mu.RLock()
	for _, msg := range cw.messages {
		if cw.matchesSingleFilter(msg, filter) {
			cw.mu.RUnlock()
			return &msg, nil
		}
	}
	cw.mu.RUnlock()

	// Set up a channel for new messages
	msgCh := make(chan ConsoleMessage, 1)
	handler := func(msg ConsoleMessage) {
		if cw.matchesSingleFilter(msg, filter) {
			select {
			case msgCh <- msg:
			default:
			}
		}
	}

	handlerID := cw.addHandler(handler)
	defer cw.removeHandler(handlerID)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-msgCh:
		return &msg, nil
	}
}

// WaitForError waits for an error message.
func (cw *ConsoleWatcher) WaitForError(ctx context.Context) (*ConsoleMessage, error) {
	return cw.WaitForMessage(ctx, ConsoleFilter{Type: "error"})
}

// HasErrors returns true if there are any error messages.
func (cw *ConsoleWatcher) HasErrors() bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	for _, msg := range cw.messages {
		if msg.Type == "error" {
			return true
		}
	}
	return false
}

// Count returns the number of captured messages.
func (cw *ConsoleWatcher) Count() int {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return len(cw.messages)
}

// ErrorCount returns the number of error messages.
func (cw *ConsoleWatcher) ErrorCount() int {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	count := 0
	for _, msg := range cw.messages {
		if msg.Type == "error" {
			count++
		}
	}
	return count
}

// handleConsoleEvent processes incoming console events.
func (cw *ConsoleWatcher) handleConsoleEvent(params map[string]any) {
	msgType := normalizeConsoleType(core.Sprint(params["type"]))

	// Extract args
	args, _ := params["args"].([]any)
	text := consoleTextFromArgs(args)

	// Extract stack trace info
	stackTrace, _ := params["stackTrace"].(map[string]any)
	var url string
	var line, column int
	if callFrames, ok := stackTrace["callFrames"].([]any); ok && len(callFrames) > 0 {
		if frame, ok := callFrames[0].(map[string]any); ok {
			url, _ = frame["url"].(string)
			lineFloat, _ := frame["lineNumber"].(float64)
			colFloat, _ := frame["columnNumber"].(float64)
			line = int(lineFloat)
			column = int(colFloat)
		}
	}

	msg := ConsoleMessage{
		Type:      msgType,
		Text:      text,
		Timestamp: consoleMessageTimestamp(params),
		URL:       url,
		Line:      line,
		Column:    column,
	}

	cw.addMessage(msg)
}

// addMessage adds a message to the store and notifies handlers.
func (cw *ConsoleWatcher) addMessage(msg ConsoleMessage) {
	cw.mu.Lock()

	// Enforce limit
	if len(cw.messages) >= cw.limit {
		drop := min(100, len(cw.messages))
		cw.messages = cw.messages[drop:]
	}
	cw.messages = append(cw.messages, msg)

	// Copy handlers to call outside lock
	handlers := slices.Clone(cw.handlers)
	cw.mu.Unlock()

	// Call handlers
	for _, registration := range handlers {
		registration.handler(msg)
	}
}

// matchesFilter checks if a message matches any filter.
func (cw *ConsoleWatcher) matchesFilter(msg ConsoleMessage) bool {
	if len(cw.filters) == 0 {
		return true
	}
	for _, filter := range cw.filters {
		if cw.matchesSingleFilter(msg, filter) {
			return true
		}
	}
	return false
}

// matchesSingleFilter checks if a message matches a specific filter.
func (cw *ConsoleWatcher) matchesSingleFilter(msg ConsoleMessage, filter ConsoleFilter) bool {
	if filter.Type != "" && msg.Type != normalizeConsoleType(filter.Type) {
		return false
	}
	if filter.Pattern != "" {
		// Simple substring match
		if !containsString(msg.Text, filter.Pattern) {
			return false
		}
	}
	return true
}

// containsString checks if s contains substr (case-sensitive).
func containsString(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findString(s, substr) >= 0)
}

// findString finds substr in s, returns -1 if not found.
func findString(s, substr string) int {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ExceptionInfo represents information about a JavaScript exception.
type ExceptionInfo struct {
	Text         string    `json:"text"`
	LineNumber   int       `json:"lineNumber"`
	ColumnNumber int       `json:"columnNumber"`
	URL          string    `json:"url"`
	StackTrace   string    `json:"stackTrace"`
	Timestamp    time.Time `json:"timestamp"`
}

// ExceptionWatcher watches for JavaScript exceptions.
type ExceptionWatcher struct {
	mu            sync.RWMutex
	wv            *Webview
	exceptions    []ExceptionInfo
	handlers      []exceptionHandlerRegistration
	nextHandlerID atomic.Int64
}

type exceptionHandlerRegistration struct {
	id      int64
	handler func(ExceptionInfo)
}

// NewExceptionWatcher creates a new exception watcher.
func NewExceptionWatcher(wv *Webview) *ExceptionWatcher {
	ew := &ExceptionWatcher{
		wv:         wv,
		exceptions: make([]ExceptionInfo, 0),
		handlers:   make([]exceptionHandlerRegistration, 0),
	}

	// Subscribe to exception events
	wv.client.OnEvent("Runtime.exceptionThrown", func(params map[string]any) {
		ew.handleException(params)
	})

	return ew
}

// Exceptions returns all captured exceptions.
func (ew *ExceptionWatcher) Exceptions() []ExceptionInfo {
	return slices.Collect(ew.ExceptionsAll())
}

// ExceptionsAll returns an iterator over all captured exceptions.
func (ew *ExceptionWatcher) ExceptionsAll() iter.Seq[ExceptionInfo] {
	return func(yield func(ExceptionInfo) bool) {
		ew.mu.RLock()
		defer ew.mu.RUnlock()

		for _, exc := range ew.exceptions {
			if !yield(exc) {
				return
			}
		}
	}
}

// Clear clears all captured exceptions.
func (ew *ExceptionWatcher) Clear() {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	ew.exceptions = ew.exceptions[:0]
}

// HasExceptions returns true if there are any exceptions.
func (ew *ExceptionWatcher) HasExceptions() bool {
	ew.mu.RLock()
	defer ew.mu.RUnlock()
	return len(ew.exceptions) > 0
}

// Count returns the number of exceptions.
func (ew *ExceptionWatcher) Count() int {
	ew.mu.RLock()
	defer ew.mu.RUnlock()
	return len(ew.exceptions)
}

// AddHandler adds a handler for exceptions.
func (ew *ExceptionWatcher) AddHandler(handler func(ExceptionInfo)) {
	ew.addHandler(handler)
}

func (ew *ExceptionWatcher) addHandler(handler func(ExceptionInfo)) int64 {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	id := ew.nextHandlerID.Add(1)
	ew.handlers = append(ew.handlers, exceptionHandlerRegistration{
		id:      id,
		handler: handler,
	})
	return id
}

func (ew *ExceptionWatcher) removeHandler(id int64) {
	ew.mu.Lock()
	defer ew.mu.Unlock()

	for i, registration := range ew.handlers {
		if registration.id == id {
			ew.handlers = slices.Delete(ew.handlers, i, i+1)
			return
		}
	}
}

// WaitForException waits for an exception to be thrown.
func (ew *ExceptionWatcher) WaitForException(ctx context.Context) (*ExceptionInfo, error) {
	// Check existing exceptions first
	ew.mu.RLock()
	if len(ew.exceptions) > 0 {
		exc := ew.exceptions[len(ew.exceptions)-1]
		ew.mu.RUnlock()
		return &exc, nil
	}
	ew.mu.RUnlock()

	// Set up a channel for new exceptions
	excCh := make(chan ExceptionInfo, 1)
	handler := func(exc ExceptionInfo) {
		select {
		case excCh <- exc:
		default:
		}
	}

	handlerID := ew.addHandler(handler)
	defer ew.removeHandler(handlerID)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case exc := <-excCh:
		return &exc, nil
	}
}

// handleException processes exception events.
func (ew *ExceptionWatcher) handleException(params map[string]any) {
	exceptionDetails, ok := params["exceptionDetails"].(map[string]any)
	if !ok {
		return
	}

	text, _ := exceptionDetails["text"].(string)
	lineNum, _ := exceptionDetails["lineNumber"].(float64)
	colNum, _ := exceptionDetails["columnNumber"].(float64)
	url, _ := exceptionDetails["url"].(string)

	// Extract stack trace
	stackTrace := core.NewBuilder()
	if st, ok := exceptionDetails["stackTrace"].(map[string]any); ok {
		if frames, ok := st["callFrames"].([]any); ok {
			for _, f := range frames {
				if frame, ok := f.(map[string]any); ok {
					funcName, _ := frame["functionName"].(string)
					frameURL, _ := frame["url"].(string)
					frameLine, _ := frame["lineNumber"].(float64)
					frameCol, _ := frame["columnNumber"].(float64)
					stackTrace.WriteString(core.Sprintf("  at %s (%s:%d:%d)\n", funcName, frameURL, int(frameLine), int(frameCol)))
				}
			}
		}
	}

	// Try to get exception value description
	if exc, ok := exceptionDetails["exception"].(map[string]any); ok {
		if desc, ok := exc["description"].(string); ok && desc != "" {
			text = desc
		}
	}

	info := ExceptionInfo{
		Text:         text,
		LineNumber:   int(lineNum),
		ColumnNumber: int(colNum),
		URL:          url,
		StackTrace:   stackTrace.String(),
		Timestamp:    time.Now(),
	}

	ew.mu.Lock()
	ew.exceptions = append(ew.exceptions, info)
	handlers := slices.Clone(ew.handlers)
	ew.mu.Unlock()

	// Call handlers
	for _, registration := range handlers {
		registration.handler(info)
	}
}

// FormatConsoleOutput formats console messages for display.
func FormatConsoleOutput(messages []ConsoleMessage) string {
	output := core.NewBuilder()
	for _, msg := range messages {
		prefix := ""
		switch normalizeConsoleType(msg.Type) {
		case "error":
			prefix = "[ERROR]"
		case "warn":
			prefix = "[WARN]"
		case "info":
			prefix = "[INFO]"
		case "debug":
			prefix = "[DEBUG]"
		default:
			prefix = "[LOG]"
		}
		timestamp := msg.Timestamp.Format("15:04:05.000")
		output.WriteString(core.Sprintf("%s %s %s\n", timestamp, prefix, msg.Text))
	}
	return output.String()
}
