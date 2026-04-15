// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"testing"
	"time"
)

// TestConsoleMessage_Good verifies the ConsoleMessage struct has expected fields.
func TestConsoleMessage_Good(t *testing.T) {
	msg := ConsoleMessage{
		Type:      "error",
		Text:      "Test error message",
		Timestamp: time.Now(),
		URL:       "https://example.com/script.js",
		Line:      42,
		Column:    10,
	}

	if msg.Type != "error" {
		t.Errorf("Expected type 'error', got %q", msg.Type)
	}
	if msg.Text != "Test error message" {
		t.Errorf("Expected text 'Test error message', got %q", msg.Text)
	}
	if msg.Line != 42 {
		t.Errorf("Expected line 42, got %d", msg.Line)
	}
}

// TestElementInfo_Good verifies the ElementInfo struct has expected fields.
func TestElementInfo_Good(t *testing.T) {
	elem := ElementInfo{
		NodeID:  123,
		TagName: "DIV",
		Attributes: map[string]string{
			"id":    "container",
			"class": "main-content",
		},
		InnerHTML: "<span>Hello</span>",
		InnerText: "Hello",
		BoundingBox: &BoundingBox{
			X:      100,
			Y:      200,
			Width:  300,
			Height: 400,
		},
	}

	if elem.NodeID != 123 {
		t.Errorf("Expected nodeId 123, got %d", elem.NodeID)
	}
	if elem.TagName != "DIV" {
		t.Errorf("Expected tagName 'DIV', got %q", elem.TagName)
	}
	if elem.Attributes["id"] != "container" {
		t.Errorf("Expected id 'container', got %q", elem.Attributes["id"])
	}
	if elem.BoundingBox == nil {
		t.Fatal("Expected bounding box to be set")
	}
	if elem.BoundingBox.Width != 300 {
		t.Errorf("Expected width 300, got %f", elem.BoundingBox.Width)
	}
}

// TestBoundingBox_Good verifies the BoundingBox struct has expected fields.
func TestBoundingBox_Good(t *testing.T) {
	box := BoundingBox{
		X:      10.5,
		Y:      20.5,
		Width:  100.0,
		Height: 50.0,
	}

	if box.X != 10.5 {
		t.Errorf("Expected X 10.5, got %f", box.X)
	}
	if box.Y != 20.5 {
		t.Errorf("Expected Y 20.5, got %f", box.Y)
	}
	if box.Width != 100.0 {
		t.Errorf("Expected width 100.0, got %f", box.Width)
	}
	if box.Height != 50.0 {
		t.Errorf("Expected height 50.0, got %f", box.Height)
	}
}

// TestWithTimeout_Good verifies the WithTimeout option sets timeout correctly.
func TestWithTimeout_Good(t *testing.T) {
	// We can't fully test without a real Chrome connection,
	// but we can verify the option function works
	wv := &Webview{}
	opt := WithTimeout(60 * time.Second)

	err := opt(wv)
	if err != nil {
		t.Fatalf("WithTimeout returned error: %v", err)
	}

	if wv.timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", wv.timeout)
	}
}

// TestWithConsoleLimit_Good verifies the WithConsoleLimit option sets limit correctly.
func TestWithConsoleLimit_Good(t *testing.T) {
	wv := &Webview{}
	opt := WithConsoleLimit(500)

	err := opt(wv)
	if err != nil {
		t.Fatalf("WithConsoleLimit returned error: %v", err)
	}

	if wv.consoleLimit != 500 {
		t.Errorf("Expected consoleLimit 500, got %d", wv.consoleLimit)
	}
}

// TestNew_Bad_NoDebugURL verifies New fails without a debug URL.
func TestNew_Bad_NoDebugURL(t *testing.T) {
	_, err := New()
	if err == nil {
		t.Error("Expected error when creating Webview without debug URL")
	}
}

// TestNew_Bad_InvalidDebugURL verifies New fails with invalid debug URL.
func TestNew_Bad_InvalidDebugURL(t *testing.T) {
	_, err := New(WithDebugURL("http://localhost:99999"))
	if err == nil {
		t.Error("Expected error when connecting to invalid debug URL")
	}
}

// TestActionSequence_Good verifies action sequence building works.
func TestActionSequence_Good(t *testing.T) {
	seq := NewActionSequence().
		Navigate("https://example.com").
		WaitForSelector("#main").
		Click("#button").
		Type("#input", "hello").
		Wait(100 * time.Millisecond)

	if len(seq.actions) != 5 {
		t.Errorf("Expected 5 actions, got %d", len(seq.actions))
	}
}

// TestClickAction_Good verifies ClickAction struct.
func TestClickAction_Good(t *testing.T) {
	action := ClickAction{Selector: "#submit"}
	if action.Selector != "#submit" {
		t.Errorf("Expected selector '#submit', got %q", action.Selector)
	}
}

// TestTypeAction_Good verifies TypeAction struct.
func TestTypeAction_Good(t *testing.T) {
	action := TypeAction{Selector: "#email", Text: "test@example.com"}
	if action.Selector != "#email" {
		t.Errorf("Expected selector '#email', got %q", action.Selector)
	}
	if action.Text != "test@example.com" {
		t.Errorf("Expected text 'test@example.com', got %q", action.Text)
	}
}

// TestNavigateAction_Good verifies NavigateAction struct.
func TestNavigateAction_Good(t *testing.T) {
	action := NavigateAction{URL: "https://example.com"}
	if action.URL != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got %q", action.URL)
	}
}

// TestWaitAction_Good verifies WaitAction struct.
func TestWaitAction_Good(t *testing.T) {
	action := WaitAction{Duration: 5 * time.Second}
	if action.Duration != 5*time.Second {
		t.Errorf("Expected duration 5s, got %v", action.Duration)
	}
}

// TestWaitForSelectorAction_Good verifies WaitForSelectorAction struct.
func TestWaitForSelectorAction_Good(t *testing.T) {
	action := WaitForSelectorAction{Selector: ".loading"}
	if action.Selector != ".loading" {
		t.Errorf("Expected selector '.loading', got %q", action.Selector)
	}
}

// TestScrollAction_Good verifies ScrollAction struct.
func TestScrollAction_Good(t *testing.T) {
	action := ScrollAction{X: 0, Y: 500}
	if action.X != 0 {
		t.Errorf("Expected X 0, got %d", action.X)
	}
	if action.Y != 500 {
		t.Errorf("Expected Y 500, got %d", action.Y)
	}
}

// TestFocusAction_Good verifies FocusAction struct.
func TestFocusAction_Good(t *testing.T) {
	action := FocusAction{Selector: "#input"}
	if action.Selector != "#input" {
		t.Errorf("Expected selector '#input', got %q", action.Selector)
	}
}

// TestBlurAction_Good verifies BlurAction struct.
func TestBlurAction_Good(t *testing.T) {
	action := BlurAction{Selector: "#input"}
	if action.Selector != "#input" {
		t.Errorf("Expected selector '#input', got %q", action.Selector)
	}
}

// TestClearAction_Good verifies ClearAction struct.
func TestClearAction_Good(t *testing.T) {
	action := ClearAction{Selector: "#input"}
	if action.Selector != "#input" {
		t.Errorf("Expected selector '#input', got %q", action.Selector)
	}
}

// TestSelectAction_Good verifies SelectAction struct.
func TestSelectAction_Good(t *testing.T) {
	action := SelectAction{Selector: "#dropdown", Value: "option1"}
	if action.Selector != "#dropdown" {
		t.Errorf("Expected selector '#dropdown', got %q", action.Selector)
	}
	if action.Value != "option1" {
		t.Errorf("Expected value 'option1', got %q", action.Value)
	}
}

// TestCheckAction_Good verifies CheckAction struct.
func TestCheckAction_Good(t *testing.T) {
	action := CheckAction{Selector: "#checkbox", Checked: true}
	if action.Selector != "#checkbox" {
		t.Errorf("Expected selector '#checkbox', got %q", action.Selector)
	}
	if !action.Checked {
		t.Error("Expected checked to be true")
	}
}

// TestHoverAction_Good verifies HoverAction struct.
func TestHoverAction_Good(t *testing.T) {
	action := HoverAction{Selector: "#menu-item"}
	if action.Selector != "#menu-item" {
		t.Errorf("Expected selector '#menu-item', got %q", action.Selector)
	}
}

// TestDoubleClickAction_Good verifies DoubleClickAction struct.
func TestDoubleClickAction_Good(t *testing.T) {
	action := DoubleClickAction{Selector: "#editable"}
	if action.Selector != "#editable" {
		t.Errorf("Expected selector '#editable', got %q", action.Selector)
	}
}

// TestRightClickAction_Good verifies RightClickAction struct.
func TestRightClickAction_Good(t *testing.T) {
	action := RightClickAction{Selector: "#context-menu-trigger"}
	if action.Selector != "#context-menu-trigger" {
		t.Errorf("Expected selector '#context-menu-trigger', got %q", action.Selector)
	}
}

// TestPressKeyAction_Good verifies PressKeyAction struct.
func TestPressKeyAction_Good(t *testing.T) {
	action := PressKeyAction{Key: "Enter"}
	if action.Key != "Enter" {
		t.Errorf("Expected key 'Enter', got %q", action.Key)
	}
}

// TestSetAttributeAction_Good verifies SetAttributeAction struct.
func TestSetAttributeAction_Good(t *testing.T) {
	action := SetAttributeAction{
		Selector:  "#element",
		Attribute: "data-value",
		Value:     "test",
	}
	if action.Selector != "#element" {
		t.Errorf("Expected selector '#element', got %q", action.Selector)
	}
	if action.Attribute != "data-value" {
		t.Errorf("Expected attribute 'data-value', got %q", action.Attribute)
	}
	if action.Value != "test" {
		t.Errorf("Expected value 'test', got %q", action.Value)
	}
}

// TestRemoveAttributeAction_Good verifies RemoveAttributeAction struct.
func TestRemoveAttributeAction_Good(t *testing.T) {
	action := RemoveAttributeAction{
		Selector:  "#element",
		Attribute: "disabled",
	}
	if action.Selector != "#element" {
		t.Errorf("Expected selector '#element', got %q", action.Selector)
	}
	if action.Attribute != "disabled" {
		t.Errorf("Expected attribute 'disabled', got %q", action.Attribute)
	}
}

// TestSetValueAction_Good verifies SetValueAction struct.
func TestSetValueAction_Good(t *testing.T) {
	action := SetValueAction{
		Selector: "#input",
		Value:    "new value",
	}
	if action.Selector != "#input" {
		t.Errorf("Expected selector '#input', got %q", action.Selector)
	}
	if action.Value != "new value" {
		t.Errorf("Expected value 'new value', got %q", action.Value)
	}
}

// TestScrollIntoViewAction_Good verifies ScrollIntoViewAction struct.
func TestScrollIntoViewAction_Good(t *testing.T) {
	action := ScrollIntoViewAction{Selector: "#target"}
	if action.Selector != "#target" {
		t.Errorf("Expected selector '#target', got %q", action.Selector)
	}
}

// TestFormatConsoleOutput_Good verifies console output formatting.
func TestFormatConsoleOutput_Good(t *testing.T) {
	ts := time.Date(2026, 1, 15, 14, 30, 45, 123000000, time.UTC)
	messages := []ConsoleMessage{
		{Type: "error", Text: "something broke", Timestamp: ts},
		{Type: "warning", Text: "deprecated call", Timestamp: ts},
		{Type: "info", Text: "loaded", Timestamp: ts},
		{Type: "debug", Text: "trace data", Timestamp: ts},
		{Type: "log", Text: "hello world", Timestamp: ts},
	}

	output := FormatConsoleOutput(messages)

	expected := []string{
		"14:30:45.123 [ERROR] something broke",
		"14:30:45.123 [WARN] deprecated call",
		"14:30:45.123 [INFO] loaded",
		"14:30:45.123 [DEBUG] trace data",
		"14:30:45.123 [LOG] hello world",
	}
	for _, exp := range expected {
		if !containsString(output, exp) {
			t.Errorf("Expected output to contain %q", exp)
		}
	}
}

// TestFormatConsoleOutput_Good_Empty verifies empty message list.
func TestFormatConsoleOutput_Good_Empty(t *testing.T) {
	output := FormatConsoleOutput(nil)
	if output != "" {
		t.Errorf("Expected empty string, got %q", output)
	}
}

// TestNormalizeConsoleType_Good verifies CDP warning aliases are normalised.
func TestNormalizeConsoleType_Good(t *testing.T) {
	if got := normalizeConsoleType("warn"); got != "warning" {
		t.Fatalf("normalizeConsoleType(\"warn\") = %q, want %q", got, "warning")
	}
	if got := normalizeConsoleType("WARNING"); got != "warning" {
		t.Fatalf("normalizeConsoleType(\"WARNING\") = %q, want %q", got, "warning")
	}
}

// TestWebviewHandleConsoleEvent_Good_NormalizesWarningType verifies CDP warn events are stored as warnings.
func TestWebviewHandleConsoleEvent_Good_NormalizesWarningType(t *testing.T) {
	wv := &Webview{
		consoleLogs:  make([]ConsoleMessage, 0),
		consoleLimit: 10,
	}

	wv.handleConsoleEvent(map[string]any{
		"type": "warn",
		"args": []any{
			map[string]any{"value": "deprecated"},
		},
	})

	if len(wv.consoleLogs) != 1 {
		t.Fatalf("Expected one console message, got %d", len(wv.consoleLogs))
	}
	if wv.consoleLogs[0].Type != "warning" {
		t.Fatalf("Expected warning type, got %q", wv.consoleLogs[0].Type)
	}
	if wv.consoleLogs[0].Text != "deprecated" {
		t.Fatalf("Expected text %q, got %q", "deprecated", wv.consoleLogs[0].Text)
	}
}

// TestContainsString_Good verifies substring matching.
func TestContainsString_Good(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "xyz", false},
		{"hello", "", true},
		{"", "", true},
		{"", "a", false},
		{"abc", "abc", true},
		{"abc", "abcd", false},
	}

	for _, tc := range tests {
		got := containsString(tc.s, tc.substr)
		if got != tc.want {
			t.Errorf("containsString(%q, %q) = %v, want %v", tc.s, tc.substr, got, tc.want)
		}
	}
}

// TestFindString_Good verifies string search.
func TestFindString_Good(t *testing.T) {
	tests := []struct {
		s, substr string
		want      int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello world", "xyz", -1},
		{"abcabc", "abc", 0},
		{"abc", "abc", 0},
	}

	for _, tc := range tests {
		got := findString(tc.s, tc.substr)
		if got != tc.want {
			t.Errorf("findString(%q, %q) = %d, want %d", tc.s, tc.substr, got, tc.want)
		}
	}
}

// TestFormatJSValue_Good verifies JavaScript value formatting.
func TestFormatJSValue_Good(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{"hello", `"hello"`},
		{true, "true"},
		{false, "false"},
		{nil, "null"},
		{42, "42"},
		{3.14, "3.14"},
		{map[string]any{"enabled": true}, `{"enabled":true}`},
		{[]any{1, "two"}, `[1,"two"]`},
	}

	for _, tc := range tests {
		got := formatJSValue(tc.input)
		if got != tc.want {
			t.Errorf("formatJSValue(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestGetString_Good verifies map string extraction.
func TestGetString_Good(t *testing.T) {
	m := map[string]any{
		"name":  "test",
		"count": 42,
	}

	if got := getString(m, "name"); got != "test" {
		t.Errorf("getString(m, 'name') = %q, want 'test'", got)
	}
	if got := getString(m, "count"); got != "" {
		t.Errorf("getString(m, 'count') = %q, want empty (not a string)", got)
	}
	if got := getString(m, "missing"); got != "" {
		t.Errorf("getString(m, 'missing') = %q, want empty", got)
	}
}

// TestParseElementContent_Good verifies inner content extraction from CDP output.
func TestParseElementContent_Good(t *testing.T) {
	result := map[string]any{
		"result": map[string]any{
			"value": map[string]any{
				"innerHTML": "<span>Hello</span>",
				"innerText": "Hello",
			},
		},
	}

	innerHTML, innerText := parseElementContent(result)
	if innerHTML != "<span>Hello</span>" {
		t.Fatalf("parseElementContent innerHTML = %q, want %q", innerHTML, "<span>Hello</span>")
	}
	if innerText != "Hello" {
		t.Fatalf("parseElementContent innerText = %q, want %q", innerText, "Hello")
	}
}

// TestWaitAction_Good_ContextCancelled verifies WaitAction respects context cancellation.
func TestWaitAction_Good_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	action := WaitAction{Duration: 10 * time.Second}
	err := action.Execute(ctx, nil)
	if err == nil {
		t.Error("Expected context cancelled error")
	}
}

// TestWaitAction_Good_ShortWait verifies WaitAction completes after duration.
func TestWaitAction_Good_ShortWait(t *testing.T) {
	ctx := context.Background()
	action := WaitAction{Duration: 10 * time.Millisecond}

	start := time.Now()
	err := action.Execute(ctx, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected at least 10ms elapsed, got %v", elapsed)
	}
}

// TestAddConsoleMessage_Good verifies console message buffer management.
func TestAddConsoleMessage_Good(t *testing.T) {
	wv := &Webview{
		consoleLogs:  make([]ConsoleMessage, 0, 10),
		consoleLimit: 5,
	}

	// Add messages up to the limit
	for i := range 6 {
		wv.addConsoleMessage(ConsoleMessage{
			Type: "log",
			Text: time.Duration(i).String(),
		})
	}

	// Buffer should have been trimmed
	if len(wv.consoleLogs) > wv.consoleLimit {
		t.Errorf("Expected at most %d messages, got %d", wv.consoleLimit, len(wv.consoleLogs))
	}
}

// TestAddConsoleMessage_Good_ZeroLimitDropsMessages verifies zero retention disables storage.
func TestAddConsoleMessage_Good_ZeroLimitDropsMessages(t *testing.T) {
	wv := &Webview{
		consoleLogs:  make([]ConsoleMessage, 0, 1),
		consoleLimit: 0,
	}

	wv.addConsoleMessage(ConsoleMessage{Type: "log", Text: "ignored"})

	if len(wv.consoleLogs) != 0 {
		t.Fatalf("Expected zero retained messages, got %d", len(wv.consoleLogs))
	}
}

// TestConsoleWatcherFilter_Good verifies console watcher filter matching.
func TestConsoleWatcherFilter_Good(t *testing.T) {
	// Create a minimal ConsoleWatcher without a real Webview
	cw := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	// No filters — everything matches
	msg := ConsoleMessage{Type: "error", Text: "test error"}
	if !cw.matchesFilter(msg) {
		t.Error("Expected message to match with no filters")
	}

	// Add type filter
	cw.AddFilter(ConsoleFilter{Type: "error"})
	if !cw.matchesFilter(msg) {
		t.Error("Expected error message to match error filter")
	}

	logMsg := ConsoleMessage{Type: "log", Text: "test log"}
	if cw.matchesFilter(logMsg) {
		t.Error("Expected log message NOT to match error filter")
	}

	// Add pattern filter
	cw.ClearFilters()
	cw.AddFilter(ConsoleFilter{Pattern: "hello"})
	helloMsg := ConsoleMessage{Type: "log", Text: "hello world"}
	if !cw.matchesFilter(helloMsg) {
		t.Error("Expected 'hello world' to match pattern 'hello'")
	}
	if cw.matchesFilter(msg) {
		t.Error("Expected 'test error' NOT to match pattern 'hello'")
	}

	cw.ClearFilters()
	cw.AddFilter(ConsoleFilter{Type: "warning"})
	if !cw.matchesFilter(ConsoleMessage{Type: "warning", Text: "deprecated"}) {
		t.Error("Expected warning message to match warning filter")
	}
	if cw.matchesFilter(ConsoleMessage{Type: "warn", Text: "deprecated"}) {
		t.Error("Expected warn message not to match warning filter")
	}
}

// TestConsoleWatcherCounts_Good verifies console watcher counting methods.
func TestConsoleWatcherCounts_Good(t *testing.T) {
	cw := &ConsoleWatcher{
		messages: []ConsoleMessage{
			{Type: "log", Text: "info 1"},
			{Type: "error", Text: "err 1"},
			{Type: "log", Text: "info 2"},
			{Type: "error", Text: "err 2"},
			{Type: "warning", Text: "warn 1"},
		},
		filters:  make([]ConsoleFilter, 0),
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	if cw.Count() != 5 {
		t.Errorf("Expected count 5, got %d", cw.Count())
	}
	if cw.ErrorCount() != 2 {
		t.Errorf("Expected error count 2, got %d", cw.ErrorCount())
	}
	if !cw.HasErrors() {
		t.Error("Expected HasErrors() to be true")
	}

	errors := cw.Errors()
	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}

	warnings := cw.Warnings()
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(warnings))
	}

	cw.Clear()
	if cw.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", cw.Count())
	}
	if cw.HasErrors() {
		t.Error("Expected HasErrors() to be false after clear")
	}
}

// TestExceptionWatcher_Good verifies exception watcher basic operations.
func TestExceptionWatcher_Good(t *testing.T) {
	ew := &ExceptionWatcher{
		exceptions: make([]ExceptionInfo, 0),
		handlers:   make([]exceptionHandlerRegistration, 0),
	}

	if ew.HasExceptions() {
		t.Error("Expected no exceptions initially")
	}
	if ew.Count() != 0 {
		t.Errorf("Expected count 0, got %d", ew.Count())
	}

	// Simulate adding an exception
	ew.exceptions = append(ew.exceptions, ExceptionInfo{
		Text:       "TypeError: undefined is not a function",
		LineNumber: 10,
		URL:        "https://example.com/app.js",
	})

	if !ew.HasExceptions() {
		t.Error("Expected HasExceptions() to be true")
	}
	if ew.Count() != 1 {
		t.Errorf("Expected count 1, got %d", ew.Count())
	}

	exceptions := ew.Exceptions()
	if len(exceptions) != 1 {
		t.Errorf("Expected 1 exception, got %d", len(exceptions))
	}
	if exceptions[0].Text != "TypeError: undefined is not a function" {
		t.Errorf("Unexpected exception text: %q", exceptions[0].Text)
	}

	ew.Clear()
	if ew.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", ew.Count())
	}
}

// TestAngularRouterState_Good verifies AngularRouterState struct.
func TestAngularRouterState_Good(t *testing.T) {
	state := AngularRouterState{
		URL:      "/dashboard",
		Fragment: "section1",
		Params:   map[string]string{"id": "123"},
		QueryParams: map[string]string{
			"tab": "settings",
		},
	}

	if state.URL != "/dashboard" {
		t.Errorf("Expected URL '/dashboard', got %q", state.URL)
	}
	if state.Fragment != "section1" {
		t.Errorf("Expected fragment 'section1', got %q", state.Fragment)
	}
	if state.Params["id"] != "123" {
		t.Errorf("Expected param id '123', got %q", state.Params["id"])
	}
	if state.QueryParams["tab"] != "settings" {
		t.Errorf("Expected query param tab 'settings', got %q", state.QueryParams["tab"])
	}
}

// TestTargetInfo_Good verifies TargetInfo struct.
func TestTargetInfo_Good(t *testing.T) {
	target := TargetInfo{
		ID:                   "ABC123",
		Type:                 "page",
		Title:                "Example",
		URL:                  "https://example.com",
		WebSocketDebuggerURL: "ws://localhost:9222/devtools/page/ABC123",
	}

	if target.ID != "ABC123" {
		t.Errorf("Expected ID 'ABC123', got %q", target.ID)
	}
	if target.Type != "page" {
		t.Errorf("Expected type 'page', got %q", target.Type)
	}
	if target.WebSocketDebuggerURL == "" {
		t.Error("Expected WebSocketDebuggerURL to be set")
	}
}

// TestConsoleWatcherAddMessage_Good verifies message buffer limit enforcement.
func TestConsoleWatcherAddMessage_Good(t *testing.T) {
	cw := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    5,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	// Add messages past the limit
	for i := range 7 {
		cw.addMessage(ConsoleMessage{
			Type: "log",
			Text: time.Duration(i).String(),
		})
	}

	if len(cw.messages) > cw.limit {
		t.Errorf("Expected at most %d messages, got %d", cw.limit, len(cw.messages))
	}
}

// TestConsoleWatcherHandler_Good verifies handlers are called for new messages.
func TestConsoleWatcherHandler_Good(t *testing.T) {
	cw := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	var received ConsoleMessage
	cw.AddHandler(func(msg ConsoleMessage) {
		received = msg
	})

	cw.addMessage(ConsoleMessage{Type: "error", Text: "handler test"})

	if received.Text != "handler test" {
		t.Errorf("Handler not called or wrong message: got %q", received.Text)
	}
}

// TestConsoleWatcherFilteredMessages_Good verifies filtered message retrieval.
func TestConsoleWatcherFilteredMessages_Good(t *testing.T) {
	cw := &ConsoleWatcher{
		messages: []ConsoleMessage{
			{Type: "log", Text: "info msg"},
			{Type: "error", Text: "error msg"},
			{Type: "log", Text: "another info"},
		},
		filters:  []ConsoleFilter{{Type: "error"}},
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	filtered := cw.FilteredMessages()
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 filtered message, got %d", len(filtered))
	}
	if filtered[0].Type != "error" {
		t.Errorf("Expected error type, got %q", filtered[0].Type)
	}
}

// TestConsoleWatcherFilteredMessages_Good_UsesAnyActiveFilter verifies filters compose as a union.
func TestConsoleWatcherFilteredMessages_Good_UsesAnyActiveFilter(t *testing.T) {
	cw := &ConsoleWatcher{
		messages: []ConsoleMessage{
			{Type: "error", Text: "boom happened"},
			{Type: "error", Text: "different message"},
			{Type: "log", Text: "boom happened"},
		},
		filters: []ConsoleFilter{
			{Type: "error"},
			{Pattern: "boom"},
		},
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	filtered := cw.FilteredMessages()
	if len(filtered) != 3 {
		t.Fatalf("Expected 3 filtered messages, got %d", len(filtered))
	}
	if filtered[0].Text != "boom happened" {
		t.Fatalf("Expected the first matching message, got %q", filtered[0].Text)
	}
	if filtered[1].Text != "different message" {
		t.Fatalf("Expected the second stored message to remain visible, got %q", filtered[1].Text)
	}
	if filtered[2].Text != "boom happened" {
		t.Fatalf("Expected the log message matching the pattern filter, got %q", filtered[2].Text)
	}
}

// TestConsoleWatcherSetLimit_Good_AppliesToFutureWrites verifies shrinking the limit trims buffered messages on the next append.
func TestConsoleWatcherSetLimit_Good_AppliesToFutureWrites(t *testing.T) {
	cw := &ConsoleWatcher{
		messages: []ConsoleMessage{
			{Type: "log", Text: "first"},
			{Type: "log", Text: "second"},
			{Type: "log", Text: "third"},
		},
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	cw.SetLimit(2)

	if cw.Count() != 3 {
		t.Fatalf("Expected 3 messages to remain until the next append, got %d", cw.Count())
	}

	cw.addMessage(ConsoleMessage{Type: "log", Text: "fourth"})

	if cw.Count() != 2 {
		t.Fatalf("Expected 2 messages after the next append, got %d", cw.Count())
	}
	if messages := cw.Messages(); messages[0].Text != "third" || messages[1].Text != "fourth" {
		t.Fatalf("Unexpected retained messages after trimming: %#v", messages)
	}
}

// TestExceptionInfo_Good verifies ExceptionInfo struct.
func TestExceptionInfo_Good(t *testing.T) {
	info := ExceptionInfo{
		Text:         "ReferenceError: foo is not defined",
		LineNumber:   42,
		ColumnNumber: 10,
		URL:          "https://example.com/app.js",
		StackTrace:   "  at bar (app.js:42:10)\n",
		Timestamp:    time.Now(),
	}

	if info.Text != "ReferenceError: foo is not defined" {
		t.Errorf("Unexpected text: %q", info.Text)
	}
	if info.LineNumber != 42 {
		t.Errorf("Expected line 42, got %d", info.LineNumber)
	}
	if info.StackTrace == "" {
		t.Error("Expected stack trace to be set")
	}
}
