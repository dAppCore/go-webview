// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"testing"
)

// TestWebviewContent_handleConsoleEvent_Good_StackTrace covers the
// callFrames extraction path of handleConsoleEvent, which the existing
// warning-normalisation test does not reach.
func TestWebviewContent_handleConsoleEvent_Good_StackTrace(t *testing.T) {
	wv := &Webview{
		consoleLogs:  make([]ConsoleMessage, 0),
		consoleLimit: 10,
	}

	wv.handleConsoleEvent(map[string]any{
		"type": "error",
		"args": []any{
			map[string]any{"value": "kaboom"},
		},
		"stackTrace": map[string]any{
			"callFrames": []any{
				map[string]any{
					"url":          "https://example.com/app.js",
					"lineNumber":   float64(41),
					"columnNumber": float64(7),
				},
			},
		},
	})

	if len(wv.consoleLogs) != 1 {
		t.Fatalf("console messages = %d, want 1", len(wv.consoleLogs))
	}
	msg := wv.consoleLogs[0]
	if msg.URL != "https://example.com/app.js" {
		t.Fatalf("URL = %q, want app.js", msg.URL)
	}
	if msg.Line != 41 || msg.Column != 7 {
		t.Fatalf("line/column = %d/%d, want 41/7", msg.Line, msg.Column)
	}
}

// TestWebviewContent_handleConsoleEvent_Ugly_EmptyCallFrames feeds a
// stackTrace whose callFrames slice is empty; the URL/line/column must
// stay zero-valued without dereferencing a missing frame.
func TestWebviewContent_handleConsoleEvent_Ugly_EmptyCallFrames(t *testing.T) {
	wv := &Webview{
		consoleLogs:  make([]ConsoleMessage, 0),
		consoleLimit: 10,
	}

	wv.handleConsoleEvent(map[string]any{
		"type": "log",
		"args": []any{map[string]any{"value": "plain"}},
		"stackTrace": map[string]any{
			"callFrames": []any{},
		},
	})

	if len(wv.consoleLogs) != 1 {
		t.Fatalf("console messages = %d, want 1", len(wv.consoleLogs))
	}
	if wv.consoleLogs[0].URL != "" || wv.consoleLogs[0].Line != 0 {
		t.Fatalf("expected zero stack info, got %+v", wv.consoleLogs[0])
	}
}

// TestWebviewContent_getElementContent_Bad_ResolveNodeError covers the
// early-return paths of getElementContent when CDP cannot resolve the
// node or returns an unusable object.
func TestWebviewContent_getElementContent_Bad_ResolveNodeError(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.resolveNode":
			target.replyError(msg.ID, "node could not be resolved")
		default:
			target.reply(msg.ID, map[string]any{})
		}
	})

	html, text := wv.getElementContent(context.Background(), 7)
	if html != "" || text != "" {
		t.Fatalf("getElementContent = (%q, %q), want empty on resolve error", html, text)
	}
}

// TestWebviewContent_getElementContent_Ugly_MissingObjectID covers the
// path where resolveNode succeeds but the object lacks an objectId.
func TestWebviewContent_getElementContent_Ugly_MissingObjectID(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{}})
		default:
			target.reply(msg.ID, map[string]any{})
		}
	})

	html, text := wv.getElementContent(context.Background(), 7)
	if html != "" || text != "" {
		t.Fatalf("getElementContent = (%q, %q), want empty on missing objectId", html, text)
	}
}

// TestWebviewContent_getElementContent_Bad_CallFunctionError covers the
// callFunctionOn failure path after a successful node resolution.
func TestWebviewContent_getElementContent_Bad_CallFunctionError(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-9"}})
		case "Runtime.callFunctionOn":
			target.replyError(msg.ID, "evaluation failed")
		default:
			target.reply(msg.ID, map[string]any{})
		}
	})

	html, text := wv.getElementContent(context.Background(), 7)
	if html != "" || text != "" {
		t.Fatalf("getElementContent = (%q, %q), want empty on callFunctionOn error", html, text)
	}
}
