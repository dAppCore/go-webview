// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"testing"
	"time"

	core "dappco.re/go"
)

func newExceptionWatcherForTest() *ExceptionWatcher {
	return &ExceptionWatcher{
		exceptions: make([]ExceptionInfo, 0),
		handlers:   make([]exceptionHandlerRegistration, 0),
		limit:      10,
	}
}

func TestConsoleException_handleException_Good_StackTraceAndHandler(t *testing.T) {
	ew := newExceptionWatcherForTest()

	var delivered ExceptionInfo
	handlerFired := make(chan struct{}, 1)
	ew.AddHandler(func(info ExceptionInfo) {
		delivered = info
		handlerFired <- struct{}{}
	})

	ew.handleException(map[string]any{
		"exceptionDetails": map[string]any{
			"text":         "Uncaught TypeError",
			"lineNumber":   float64(12),
			"columnNumber": float64(5),
			"url":          "https://example.com/app.js",
			"stackTrace": map[string]any{
				"callFrames": []any{
					map[string]any{
						"functionName": "boom",
						"url":          "https://example.com/app.js",
						"lineNumber":   float64(12),
						"columnNumber": float64(5),
					},
					map[string]any{
						"functionName": "main",
						"url":          "https://example.com/app.js",
						"lineNumber":   float64(40),
						"columnNumber": float64(1),
					},
				},
			},
		},
	})

	select {
	case <-handlerFired:
	case <-time.After(time.Second):
		t.Fatal("exception handler was not invoked")
	}

	if ew.Count() != 1 {
		t.Fatalf("exception count = %d, want 1", ew.Count())
	}
	if delivered.LineNumber != 12 || delivered.ColumnNumber != 5 {
		t.Fatalf("delivered exception line/col = %d/%d, want 12/5", delivered.LineNumber, delivered.ColumnNumber)
	}
	if !core.Contains(delivered.StackTrace, "at boom") || !core.Contains(delivered.StackTrace, "at main") {
		t.Fatalf("stack trace missing frames: %q", delivered.StackTrace)
	}
}

func TestConsoleException_handleException_Bad_NoExceptionDetails(t *testing.T) {
	ew := newExceptionWatcherForTest()

	// Missing exceptionDetails must be a silent no-op, not a panic or a
	// phantom exception record.
	ew.handleException(map[string]any{"unrelated": 1})

	if ew.Count() != 0 {
		t.Fatalf("exception count = %d, want 0 when exceptionDetails absent", ew.Count())
	}
}

func TestConsoleException_handleException_Ugly_EmptyCallFrames(t *testing.T) {
	ew := newExceptionWatcherForTest()

	// callFrames present but empty — the loop must produce an empty stack
	// trace string rather than crash.
	ew.handleException(map[string]any{
		"exceptionDetails": map[string]any{
			"text": "boom",
			"stackTrace": map[string]any{
				"callFrames": []any{},
			},
		},
	})

	if ew.Count() != 1 {
		t.Fatalf("exception count = %d, want 1", ew.Count())
	}
	if got := ew.Exceptions()[0].StackTrace; got != "" {
		t.Fatalf("stack trace = %q, want empty for zero call frames", got)
	}
}

func TestConsoleException_WaitForException_Good_AlreadyPresent(t *testing.T) {
	ew := newExceptionWatcherForTest()
	ew.handleException(map[string]any{
		"exceptionDetails": map[string]any{"text": "early"},
	})

	got, err := ew.WaitForException(context.Background())
	if err != nil {
		t.Fatalf("WaitForException returned error: %v", err)
	}
	if got == nil || got.Text != "early" {
		t.Fatalf("WaitForException = %+v, want the pre-existing exception", got)
	}
}

func TestConsoleException_WaitForException_Good_DeliveredToWaiter(t *testing.T) {
	ew := newExceptionWatcherForTest()

	type result struct {
		info *ExceptionInfo
		err  error
	}
	done := make(chan result, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		info, err := ew.WaitForException(ctx)
		done <- result{info, err}
	}()

	// Give the goroutine a moment to register as a waiter, then raise one.
	time.Sleep(20 * time.Millisecond)
	ew.handleException(map[string]any{
		"exceptionDetails": map[string]any{"text": "later"},
	})

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("WaitForException returned error: %v", r.err)
		}
		if r.info == nil || r.info.Text != "later" {
			t.Fatalf("WaitForException = %+v, want the raised exception", r.info)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForException did not receive the raised exception")
	}
}
