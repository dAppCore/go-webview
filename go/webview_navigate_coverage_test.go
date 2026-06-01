// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"testing"
)

// TestWebviewNav_navigateHistory_Good drives the full success path: a valid
// history payload, a navigateToHistoryEntry call, and waitForLoad observing
// document.readyState === "complete".
func TestWebviewNav_navigateHistory_Good(t *testing.T) {
	var navigated bool
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Page.getNavigationHistory":
			target.reply(msg.ID, map[string]any{
				"currentIndex": float64(1),
				"entries": []any{
					map[string]any{"id": float64(10)},
					map[string]any{"id": float64(11)},
				},
			})
		case "Page.navigateToHistoryEntry":
			navigated = true
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, "complete")
		default:
			t.Errorf("unexpected method %q", msg.Method)
		}
	})

	if err := wv.GoBack(); err != nil {
		t.Fatalf("GoBack returned error: %v", err)
	}
	if !navigated {
		t.Fatal("GoBack did not issue Page.navigateToHistoryEntry")
	}
}

// TestWebviewNav_navigateHistory_Bad_OutOfRange covers the guard that
// rejects a delta moving the index past the available entries.
func TestWebviewNav_navigateHistory_Bad_OutOfRange(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Page.getNavigationHistory" {
			t.Errorf("unexpected method %q", msg.Method)
			return
		}
		target.reply(msg.ID, map[string]any{
			"currentIndex": float64(0),
			"entries":      []any{map[string]any{"id": float64(10)}},
		})
	})

	if err := wv.GoBack(); err == nil {
		t.Fatal("GoBack succeeded when no earlier history entry exists")
	}
}

// TestWebviewNav_navigateHistory_Ugly_EntryMissingID covers the path where
// the target entry exists but carries no numeric id.
func TestWebviewNav_navigateHistory_Ugly_EntryMissingID(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Page.getNavigationHistory" {
			t.Errorf("unexpected method %q", msg.Method)
			return
		}
		target.reply(msg.ID, map[string]any{
			"currentIndex": float64(1),
			"entries": []any{
				map[string]any{"id": float64(10)},
				map[string]any{"id": "not-a-number"},
			},
		})
	})

	if err := wv.GoForward(); err == nil {
		t.Fatal("GoForward succeeded when the history entry id is non-numeric")
	}
}

// TestWebviewNav_querySelector_Bad_DocumentError covers the getDocument
// failure branch of querySelector.
func TestWebviewNav_querySelector_Bad_DocumentError(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method == "DOM.getDocument" {
			target.replyError(msg.ID, "document unavailable")
			return
		}
		target.reply(msg.ID, map[string]any{})
	})

	if _, err := wv.QuerySelector("#x"); err == nil {
		t.Fatal("QuerySelector succeeded despite DOM.getDocument failure")
	}
}

// TestWebviewNav_querySelector_Ugly_ElementNotFound covers the zero-nodeId
// branch where the selector matches nothing.
func TestWebviewNav_querySelector_Ugly_ElementNotFound(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(0)})
		default:
			t.Errorf("unexpected method %q", msg.Method)
		}
	})

	if _, err := wv.QuerySelector("#missing"); err == nil {
		t.Fatal("QuerySelector succeeded for an unmatched selector")
	}
}

// TestWebviewNav_querySelectorAll_Bad_DocumentError covers the getDocument
// failure branch of querySelectorAll.
func TestWebviewNav_querySelectorAll_Bad_DocumentError(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method == "DOM.getDocument" {
			target.replyError(msg.ID, "document unavailable")
			return
		}
		target.reply(msg.ID, map[string]any{})
	})

	if _, err := wv.QuerySelectorAll(".item"); err == nil {
		t.Fatal("QuerySelectorAll succeeded despite DOM.getDocument failure")
	}
}

// TestWebviewNav_dragAndDrop_Bad_TargetNotFound covers the branch where the
// source resolves but the target selector cannot be found.
func TestWebviewNav_dragAndDrop_Bad_TargetNotFound(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			sel, _ := msg.Params["selector"].(string)
			if sel == "#source" {
				target.reply(msg.ID, map[string]any{"nodeId": float64(31)})
				return
			}
			target.reply(msg.ID, map[string]any{"nodeId": float64(0)}) // target not found
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-7"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(1), float64(0), float64(1), float64(1), float64(0), float64(1)}}})
		default:
			target.reply(msg.ID, map[string]any{})
		}
	})

	err := wv.DragAndDrop("#source", "#target")
	if err == nil {
		t.Fatal("DragAndDrop succeeded when the target element was not found")
	}
}

func TestWebviewNav_dragAndDrop_Ugly_TargetNoBoundingBox(t *testing.T) {
	// Source has a box; target resolves but reports no box model. The drag
	// must abort on the target's nil bounding box.
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			sel, _ := msg.Params["selector"].(string)
			if sel == "#source" {
				target.reply(msg.ID, map[string]any{"nodeId": float64(31)})
				return
			}
			target.reply(msg.ID, map[string]any{"nodeId": float64(32)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-8"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			nodeID := int(msg.Params["nodeId"].(float64))
			if nodeID == 31 {
				target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(1), float64(0), float64(1), float64(1), float64(0), float64(1)}}})
				return
			}
			target.replyError(msg.ID, "no box model for target")
		default:
			target.reply(msg.ID, map[string]any{})
		}
	})

	if err := wv.DragAndDrop("#source", "#target"); err == nil {
		t.Fatal("DragAndDrop succeeded when the target had no bounding box")
	}
}
