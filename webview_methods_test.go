// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func newWebviewHarness(t *testing.T, onMessage func(*fakeCDPTarget, cdpMessage)) (*Webview, *fakeCDPTarget) {
	t.Helper()

	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = onMessage

	client := newConnectedCDPClient(t, target)
	wv := &Webview{
		client:       client,
		ctx:          context.Background(),
		timeout:      time.Second,
		consoleLogs:   make([]ConsoleMessage, 0),
		consoleLimit:  10,
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return wv, target
}

func TestWebview_Close_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	wv := &Webview{
		client:       client,
		ctx:          context.Background(),
		cancel:       func() {},
		consoleLogs:   make([]ConsoleMessage, 0),
		consoleLimit:  10,
	}

	if err := wv.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestWebview_New_Good_EnablesConsoleCapture(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Runtime.enable", "Page.enable", "DOM.enable":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q during New", msg.Method)
		}
	}

	wv, err := New(WithDebugURL(server.DebugURL()))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() { _ = wv.Close() }()

	target.writeJSON(cdpEvent{
		Method: "Runtime.consoleAPICalled",
		Params: map[string]any{
			"type": "log",
			"args": []any{map[string]any{"value": "hello"}},
		},
	})

	time.Sleep(50 * time.Millisecond)

	if got := wv.GetConsole(); len(got) != 1 || got[0].Text != "hello" {
		t.Fatalf("New console capture = %#v", got)
	}
}

func TestWebview_Navigate_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		t.Fatalf("unexpected CDP call %q for invalid navigation URL", msg.Method)
	})

	if err := wv.Navigate("javascript:alert(1)"); err == nil {
		t.Fatal("Navigate succeeded with dangerous URL")
	}
}

func TestWebview_Navigate_Good(t *testing.T) {
	var methods []string
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		methods = append(methods, msg.Method)
		switch msg.Method {
		case "Page.navigate":
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, "complete")
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	if err := wv.Navigate("https://example.com"); err != nil {
		t.Fatalf("Navigate returned error: %v", err)
	}
	if len(methods) != 2 || methods[0] != "Page.navigate" || methods[1] != "Runtime.evaluate" {
		t.Fatalf("Navigate call order = %v", methods)
	}
}

func TestWebview_QuerySelectorAndAll_Good(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(21)})
		case "DOM.querySelectorAll":
			target.reply(msg.ID, map[string]any{"nodeIds": []any{float64(21), float64(22)}})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV", "attributes": []any{"id", "main"}}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-1"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "<span>hello</span>", "innerText": "hello"}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(1), float64(2), float64(11), float64(2), float64(11), float64(12), float64(1), float64(12)}}})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	elem, err := wv.QuerySelector("#main")
	if err != nil {
		t.Fatalf("QuerySelector returned error: %v", err)
	}
	if elem.NodeID != 21 || elem.TagName != "DIV" || elem.InnerText != "hello" {
		t.Fatalf("QuerySelector returned %#v", elem)
	}
	if elem.BoundingBox == nil || elem.BoundingBox.Width != 10 || elem.BoundingBox.Height != 10 {
		t.Fatalf("QuerySelector bounding box = %#v", elem.BoundingBox)
	}

	all, err := wv.QuerySelectorAll("div.item")
	if err != nil {
		t.Fatalf("QuerySelectorAll returned error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("QuerySelectorAll len = %d, want 2", len(all))
	}

	iterated := make([]int, 0)
	for elem := range wv.QuerySelectorAllAll("div.item") {
		iterated = append(iterated, elem.NodeID)
		break
	}
	if len(iterated) != 1 || iterated[0] != 21 {
		t.Fatalf("QuerySelectorAllAll yielded %v, want first node 21", iterated)
	}
}

func TestWebview_ClickAndType_Good(t *testing.T) {
	var methods []string
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		methods = append(methods, msg.Method)
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(10)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "BUTTON"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-1"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(10), float64(20), float64(30), float64(20), float64(30), float64(40), float64(10), float64(40)}}})
		case "Input.dispatchMouseEvent", "Input.dispatchKeyEvent":
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, true)
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	if err := wv.Click("#button"); err != nil {
		t.Fatalf("Click returned error: %v", err)
	}
	if err := wv.Type("#input", "ab"); err != nil {
		t.Fatalf("Type returned error: %v", err)
	}
	if len(methods) < 8 {
		t.Fatalf("Click+Type methods = %v", methods)
	}
}

func TestWebview_WaitForSelector_Good(t *testing.T) {
	var calls int
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		calls++
		if calls == 1 {
			target.replyValue(msg.ID, false)
			return
		}
		target.replyValue(msg.ID, true)
	})

	if err := wv.WaitForSelector("#ready"); err != nil {
		t.Fatalf("WaitForSelector returned error: %v", err)
	}
	if calls < 2 {
		t.Fatalf("WaitForSelector calls = %d, want at least 2", calls)
	}
}

func TestWebview_ScreenshotAndInfo_Good(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Page.captureScreenshot":
			if got := msg.Params["format"]; got != "png" {
				t.Fatalf("captureScreenshot format = %v, want png", got)
			}
			target.reply(msg.ID, map[string]any{"data": base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47})})
		case "Runtime.evaluate":
			expr, _ := msg.Params["expression"].(string)
			switch expr {
			case "window.location.href":
				target.replyValue(msg.ID, "https://example.com")
			case "document.title":
				target.replyValue(msg.ID, "Example")
			case "document.documentElement.outerHTML":
				target.replyValue(msg.ID, "<html></html>")
			case "document.readyState":
				target.replyValue(msg.ID, "complete")
			default:
				t.Fatalf("unexpected evaluate expression %q", expr)
			}
		case "Emulation.setDeviceMetricsOverride", "Emulation.setUserAgentOverride", "Page.reload":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	png, err := wv.Screenshot()
	if err != nil {
		t.Fatalf("Screenshot returned error: %v", err)
	}
	if len(png) != 4 || png[0] != 0x89 {
		t.Fatalf("Screenshot bytes = %v", png)
	}

	if got, err := wv.GetURL(); err != nil || got != "https://example.com" {
		t.Fatalf("GetURL = %q, %v", got, err)
	}
	if got, err := wv.GetTitle(); err != nil || got != "Example" {
		t.Fatalf("GetTitle = %q, %v", got, err)
	}
	if got, err := wv.GetHTML(""); err != nil || got != "<html></html>" {
		t.Fatalf("GetHTML = %q, %v", got, err)
	}
	if err := wv.SetViewport(1440, 900); err != nil {
		t.Fatalf("SetViewport returned error: %v", err)
	}
	if err := wv.SetUserAgent("AgentHarness/1.0"); err != nil {
		t.Fatalf("SetUserAgent returned error: %v", err)
	}
	if err := wv.Reload(); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}
}

func TestWebview_Console_Good(t *testing.T) {
	wv := &Webview{
		consoleLogs:  make([]ConsoleMessage, 0),
		consoleLimit: 2,
	}

	wv.addConsoleMessage(ConsoleMessage{Text: "one"})
	wv.addConsoleMessage(ConsoleMessage{Text: "two"})
	wv.addConsoleMessage(ConsoleMessage{Text: "three"})

	got := wv.GetConsole()
	if len(got) != 2 || got[0].Text != "two" || got[1].Text != "three" {
		t.Fatalf("GetConsole = %#v", got)
	}

	iterated := make([]string, 0)
	for msg := range wv.GetConsoleAll() {
		iterated = append(iterated, msg.Text)
	}
	if len(iterated) != 2 {
		t.Fatalf("GetConsoleAll = %#v", iterated)
	}

	wv.ClearConsole()
	if got := wv.GetConsole(); len(got) != 0 {
		t.Fatalf("ClearConsole did not empty logs: %#v", got)
	}
}

func TestWebview_UploadFileAndDragAndDrop_Good(t *testing.T) {
	var methods []string
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		methods = append(methods, msg.Method)
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			sel, _ := msg.Params["selector"].(string)
			switch sel {
			case "#file":
				target.reply(msg.ID, map[string]any{"nodeId": float64(41)})
			case "#source":
				target.reply(msg.ID, map[string]any{"nodeId": float64(42)})
			case "#target":
				target.reply(msg.ID, map[string]any{"nodeId": float64(43)})
			default:
				t.Fatalf("unexpected selector %q", sel)
			}
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "INPUT"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-1"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			nodeID := int(msg.Params["nodeId"].(float64))
			box := []any{float64(nodeID), float64(nodeID), float64(nodeID + 1), float64(nodeID), float64(nodeID + 1), float64(nodeID + 1), float64(nodeID), float64(nodeID + 1)}
			target.reply(msg.ID, map[string]any{"model": map[string]any{"content": box}})
		case "DOM.setFileInputFiles", "Input.dispatchMouseEvent":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	if err := wv.UploadFile("#file", []string{"/tmp/a.txt"}); err != nil {
		t.Fatalf("UploadFile returned error: %v", err)
	}
	if err := wv.DragAndDrop("#source", "#target"); err != nil {
		t.Fatalf("DragAndDrop returned error: %v", err)
	}
	if len(methods) < 10 {
		t.Fatalf("UploadFile+DragAndDrop methods = %v", methods)
	}
}

func TestWebview_WaitForSelector_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, false)
	})
	wv.timeout = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(wv.ctx, 50*time.Millisecond)
	defer cancel()
	if err := wv.waitForSelector(ctx, "#never"); err == nil {
		t.Fatal("waitForSelector succeeded without matching element")
	}
}

func TestWebview_Click_Ugly_FallsBackToJS(t *testing.T) {
	var expressions []string
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(10)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "BUTTON"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-1"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			expr, _ := msg.Params["expression"].(string)
			expressions = append(expressions, expr)
			target.replyValue(msg.ID, true)
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	if err := wv.Click("#button"); err != nil {
		t.Fatalf("Click returned error: %v", err)
	}
	if len(expressions) != 1 || !strings.Contains(expressions[0], `document.querySelector("#button")?.click()`) {
		t.Fatalf("Click fallback expression = %v", expressions)
	}
}
