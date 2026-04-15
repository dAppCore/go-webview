// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestConsole_normalizeConsoleType_Good(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "warn", want: "warn"},
		{raw: "warning", want: "warn"},
		{raw: "  WARNING  ", want: "warn"},
		{raw: "error", want: "error"},
		{raw: "info", want: "info"},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			if got := normalizeConsoleType(tc.raw); got != tc.want {
				t.Fatalf("normalizeConsoleType(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestConsole_consoleValueToString_Good(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{name: "nil", val: nil, want: "null"},
		{name: "string", val: "hello", want: "hello"},
		{name: "number", val: float64(12), want: "12"},
		{name: "bool", val: true, want: "true"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := consoleValueToString(tc.val); got != tc.want {
				t.Fatalf("consoleValueToString(%v) = %q, want %q", tc.val, got, tc.want)
			}
		})
	}
}

func TestConsole_consoleArgText_Good(t *testing.T) {
	tests := []struct {
		name string
		arg  any
		want string
	}{
		{name: "value", arg: map[string]any{"value": "alpha"}, want: "alpha"},
		{name: "description", arg: map[string]any{"description": "bravo"}, want: "bravo"},
		{name: "preview description", arg: map[string]any{"preview": map[string]any{"description": "charlie"}}, want: "charlie"},
		{name: "preview value", arg: map[string]any{"preview": map[string]any{"value": "delta"}}, want: "delta"},
		{name: "plain scalar", arg: 42, want: "42"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := consoleArgText(tc.arg); got != tc.want {
				t.Fatalf("consoleArgText(%v) = %q, want %q", tc.arg, got, tc.want)
			}
		})
	}
}

func TestConsole_consoleArgText_Ugly(t *testing.T) {
	got := consoleArgText(map[string]any{"value": map[string]any{"nested": true}})
	if !strings.Contains(got, `"nested":true`) {
		t.Fatalf("consoleArgText fallback JSON = %q, want JSON encoding", got)
	}
}

func TestConsole_trimConsoleMessages_Good(t *testing.T) {
	messages := []ConsoleMessage{
		{Text: "one"},
		{Text: "two"},
		{Text: "three"},
	}

	tests := []struct {
		name  string
		limit int
		want  []string
	}{
		{name: "no trim", limit: 3, want: []string{"one", "two", "three"}},
		{name: "trim to one", limit: 1, want: []string{"three"}},
		{name: "zero", limit: 0, want: nil},
		{name: "negative", limit: -1, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cloned := append([]ConsoleMessage(nil), messages...)
			got := trimConsoleMessages(cloned, tc.limit)
			if len(got) != len(tc.want) {
				t.Fatalf("trimConsoleMessages len = %d, want %d", len(got), len(tc.want))
			}
			for i, want := range tc.want {
				if got[i].Text != want {
					t.Fatalf("trimConsoleMessages[%d] = %q, want %q", i, got[i].Text, want)
				}
			}
		})
	}
}

func TestConsole_trimExceptionInfos_Good(t *testing.T) {
	exceptions := []ExceptionInfo{
		{Text: "one"},
		{Text: "two"},
		{Text: "three"},
	}

	tests := []struct {
		name  string
		limit int
		want  []string
	}{
		{name: "no trim", limit: 3, want: []string{"one", "two", "three"}},
		{name: "trim to one", limit: 1, want: []string{"three"}},
		{name: "zero", limit: 0, want: nil},
		{name: "negative", limit: -1, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cloned := append([]ExceptionInfo(nil), exceptions...)
			got := trimExceptionInfos(cloned, tc.limit)
			if len(got) != len(tc.want) {
				t.Fatalf("trimExceptionInfos len = %d, want %d", len(got), len(tc.want))
			}
			for i, want := range tc.want {
				if got[i].Text != want {
					t.Fatalf("trimExceptionInfos[%d] = %q, want %q", i, got[i].Text, want)
				}
			}
		})
	}
}

func TestConsole_sanitizeConsoleText_Good(t *testing.T) {
	got := sanitizeConsoleText("line1\nline2\r\t\x1b[31m\x7f")
	if !strings.Contains(got, `line1\nline2\r\t\x1b[31m`) {
		t.Fatalf("sanitizeConsoleText did not escape control characters: %q", got)
	}
	if strings.Contains(got, "\x7f") {
		t.Fatalf("sanitizeConsoleText kept DEL byte: %q", got)
	}
}

func TestConsole_runtimeExceptionText_Good(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]any
		want string
	}{
		{name: "description", in: map[string]any{"exception": map[string]any{"description": "stack"}}, want: "stack"},
		{name: "text", in: map[string]any{"text": "boom"}, want: "boom"},
		{name: "default", in: map[string]any{}, want: "JavaScript error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := runtimeExceptionText(tc.in); got != tc.want {
				t.Fatalf("runtimeExceptionText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConsole_NewConsoleWatcher_Good(t *testing.T) {
	watcher := NewConsoleWatcher(nil)
	if watcher == nil {
		t.Fatal("NewConsoleWatcher returned nil")
	}
	if watcher.Count() != 0 {
		t.Fatalf("NewConsoleWatcher count = %d, want 0", watcher.Count())
	}
}

func TestConsole_NewConsoleWatcher_Good_SubscribesToClient(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	client := newConnectedCDPClient(t, target)
	watcher := NewConsoleWatcher(&Webview{client: client})

	target.writeJSON(cdpEvent{
		Method: "Runtime.consoleAPICalled",
		Params: map[string]any{
			"type": "log",
			"args": []any{map[string]any{"value": "hello"}},
		},
	})

	time.Sleep(50 * time.Millisecond)

	if watcher.Count() != 1 {
		t.Fatalf("NewConsoleWatcher subscription count = %d, want 1", watcher.Count())
	}
}

func TestConsole_WaitForError_Good(t *testing.T) {
	watcher := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    10,
		handlers: make([]consoleHandlerRegistration, 0),
	}
	watcher.addMessage(ConsoleMessage{Type: "warn", Text: "ignore"})
	watcher.addMessage(ConsoleMessage{Type: "error", Text: "boom"})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg, err := watcher.WaitForError(ctx)
	if err != nil {
		t.Fatalf("WaitForError returned error: %v", err)
	}
	if msg.Text != "boom" {
		t.Fatalf("WaitForError text = %q, want boom", msg.Text)
	}
}

func TestConsole_WaitForError_Bad(t *testing.T) {
	watcher := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    10,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := watcher.WaitForError(ctx); err == nil {
		t.Fatal("WaitForError succeeded without an error message")
	}
}

func TestConsole_WaitForMessage_Bad_TimesOut(t *testing.T) {
	watcher := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    10,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := watcher.WaitForMessage(ctx, ConsoleFilter{Type: "error"}); err == nil {
		t.Fatal("WaitForMessage succeeded without a matching message")
	}
}

func TestConsole_handleConsoleEvent_Good(t *testing.T) {
	watcher := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    10,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	watcher.handleConsoleEvent(map[string]any{
		"type": "warning",
		"args": []any{
			map[string]any{"value": "alpha"},
			map[string]any{"description": "beta"},
		},
		"stackTrace": map[string]any{
			"callFrames": []any{
				map[string]any{
					"url":          "https://example.com/app.js",
					"lineNumber":   float64(12),
					"columnNumber": float64(34),
				},
			},
		},
	})

	msgs := watcher.Messages()
	if len(msgs) != 1 {
		t.Fatalf("handleConsoleEvent stored %d messages, want 1", len(msgs))
	}
	if msgs[0].Type != "warning" {
		t.Fatalf("handleConsoleEvent type = %q, want warning", msgs[0].Type)
	}
	if msgs[0].Text != "alpha beta" {
		t.Fatalf("handleConsoleEvent text = %q, want %q", msgs[0].Text, "alpha beta")
	}
	if msgs[0].URL != "https://example.com/app.js" || msgs[0].Line != 12 || msgs[0].Column != 34 {
		t.Fatalf("handleConsoleEvent stack info = %#v", msgs[0])
	}
}

func TestConsole_removeHandler_Good(t *testing.T) {
	cw := &ConsoleWatcher{
		handlers: []consoleHandlerRegistration{
			{id: 1},
			{id: 2},
		},
	}

	cw.removeHandler(1)
	if len(cw.handlers) != 1 || cw.handlers[0].id != 2 {
		t.Fatalf("removeHandler did not remove the requested handler: %#v", cw.handlers)
	}

	cw.removeHandler(99)
	if len(cw.handlers) != 1 || cw.handlers[0].id != 2 {
		t.Fatalf("removeHandler changed handlers unexpectedly: %#v", cw.handlers)
	}
}

func TestConsole_SetLimit_Bad_NegativeBecomesZero(t *testing.T) {
	watcher := &ConsoleWatcher{
		limit:    10,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	watcher.SetLimit(-1)
	if watcher.limit != 0 {
		t.Fatalf("SetLimit(-1) = %d, want 0", watcher.limit)
	}
}

func TestConsole_NewExceptionWatcher_Good(t *testing.T) {
	watcher := NewExceptionWatcher(nil)
	if watcher == nil {
		t.Fatal("NewExceptionWatcher returned nil")
	}
	if watcher.Count() != 0 {
		t.Fatalf("NewExceptionWatcher count = %d, want 0", watcher.Count())
	}
	if watcher.limit != 1000 {
		t.Fatalf("NewExceptionWatcher limit = %d, want 1000", watcher.limit)
	}
}

func TestConsole_NewExceptionWatcher_Good_SubscribesToClient(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	client := newConnectedCDPClient(t, target)
	watcher := NewExceptionWatcher(&Webview{client: client})

	target.writeJSON(cdpEvent{
		Method: "Runtime.exceptionThrown",
		Params: map[string]any{
			"exceptionDetails": map[string]any{
				"text":         "boom",
				"lineNumber":   float64(1),
				"columnNumber": float64(2),
				"url":          "https://example.com/app.js",
			},
		},
	})

	time.Sleep(50 * time.Millisecond)

	if watcher.Count() != 1 {
		t.Fatalf("NewExceptionWatcher subscription count = %d, want 1", watcher.Count())
	}
}

func TestConsole_ExceptionWatcherTrimsOldExceptions_Good(t *testing.T) {
	watcher := &ExceptionWatcher{
		exceptions: make([]ExceptionInfo, 0),
		limit:      2,
		handlers:   make([]exceptionHandlerRegistration, 0),
	}

	for i := range 3 {
		watcher.handleException(map[string]any{
			"exceptionDetails": map[string]any{
				"text":         string(rune('a' + i)),
				"lineNumber":   float64(i + 1),
				"columnNumber": float64(1),
				"url":          "https://example.com/app.js",
			},
		})
	}

	if got := watcher.Count(); got != 2 {
		t.Fatalf("ExceptionWatcher count = %d, want 2", got)
	}

	excs := watcher.Exceptions()
	if len(excs) != 2 || excs[0].Text != "b" || excs[1].Text != "c" {
		t.Fatalf("ExceptionWatcher retained %#v, want [b c]", excs)
	}
}

func TestConsole_ExceptionWatcher_removeHandler_Good(t *testing.T) {
	ew := &ExceptionWatcher{
		handlers: []exceptionHandlerRegistration{
			{id: 1},
			{id: 2},
		},
	}

	ew.removeHandler(2)
	if len(ew.handlers) != 1 || ew.handlers[0].id != 1 {
		t.Fatalf("removeHandler did not remove the requested exception handler: %#v", ew.handlers)
	}

	ew.removeHandler(99)
	if len(ew.handlers) != 1 || ew.handlers[0].id != 1 {
		t.Fatalf("removeHandler changed exception handlers unexpectedly: %#v", ew.handlers)
	}
}

func TestConsole_WaitForException_Bad_TimesOut(t *testing.T) {
	ew := &ExceptionWatcher{
		exceptions: make([]ExceptionInfo, 0),
		handlers:   make([]exceptionHandlerRegistration, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := ew.WaitForException(ctx); err == nil {
		t.Fatal("WaitForException succeeded without an exception")
	}
}

func TestConsole_isWarningType_Good(t *testing.T) {
	tests := map[string]bool{
		"warn":    true,
		"warning": true,
		"ERROR":   false,
	}
	for raw, want := range tests {
		if got := isWarningType(raw); got != want {
			t.Fatalf("isWarningType(%q) = %v, want %v", raw, got, want)
		}
	}
}
