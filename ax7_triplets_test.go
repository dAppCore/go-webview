// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

type ax7OKAction struct{ called *int }

func (a ax7OKAction) Execute(context.Context, *Webview) error {
	*a.called++
	return nil
}

type ax7FailAction struct{}

func (ax7FailAction) Execute(context.Context, *Webview) error {
	return errors.New("ax7 failed")
}

func ax7WebviewBase() *Webview {
	return &Webview{
		ctx:          context.Background(),
		cancel:       func() {},
		timeout:      time.Second,
		consoleLogs:  make([]ConsoleMessage, 0),
		consoleLimit: 10,
	}
}

func ax7ConsoleWatcher(messages ...ConsoleMessage) *ConsoleWatcher {
	cw := NewConsoleWatcher(nil)
	for _, msg := range messages {
		cw.addMessage(msg)
	}
	return cw
}

func ax7ExceptionWatcher(exceptions ...ExceptionInfo) *ExceptionWatcher {
	ew := NewExceptionWatcher(nil)
	ew.exceptions = append(ew.exceptions, exceptions...)
	return ew
}

func ax7EvaluateWebview(t *testing.T, want string, fail bool) *Webview {
	t.Helper()
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		expr, _ := msg.Params["expression"].(string)
		if want != "" && !strings.Contains(expr, want) {
			t.Fatalf("expression %q does not contain %q", expr, want)
		}
		if fail {
			target.replyError(msg.ID, "evaluation failed")
			return
		}
		target.replyValue(msg.ID, true)
	})
	return wv
}

func ax7DOMWebview(t *testing.T, withBox bool) *Webview {
	t.Helper()
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(10)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV", "attributes": []any{"id", "target"}}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-1"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "<b>ok</b>", "innerText": "ok"}}})
		case "DOM.getBoxModel":
			if withBox {
				target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(10), float64(0), float64(10), float64(10), float64(0), float64(10)}}})
				return
			}
			target.reply(msg.ID, map[string]any{})
		case "Input.dispatchMouseEvent", "DOM.setFileInputFiles":
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, true)
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	return wv
}

func ax7MissingDOMWebview(t *testing.T) *Webview {
	t.Helper()
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(0)})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	return wv
}

func ax7NavigationWebview(t *testing.T) *Webview {
	t.Helper()
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Page.navigate", "Page.reload", "Emulation.setDeviceMetricsOverride", "Emulation.setUserAgentOverride":
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			expr, _ := msg.Params["expression"].(string)
			switch expr {
			case "document.readyState":
				target.replyValue(msg.ID, "complete")
			case "window.location.href":
				target.replyValue(msg.ID, "https://example.com")
			case "document.title":
				target.replyValue(msg.ID, "Example")
			case "document.documentElement.outerHTML":
				target.replyValue(msg.ID, "<html></html>")
			default:
				target.replyValue(msg.ID, "<section></section>")
			}
		case "Page.captureScreenshot":
			target.reply(msg.ID, map[string]any{"data": base64.StdEncoding.EncodeToString([]byte("png"))})
		case "Page.getNavigationHistory":
			target.reply(msg.ID, map[string]any{"currentIndex": float64(1), "entries": []any{map[string]any{"id": float64(1)}, map[string]any{"id": float64(2)}, map[string]any{"id": float64(3)}}})
		case "Page.navigateToHistoryEntry":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	return wv
}

func ax7AngularHelper(t *testing.T, handler func(*fakeCDPTarget, cdpMessage)) *AngularHelper {
	t.Helper()
	ah, _, _ := newAngularTestHarness(t, handler)
	return ah
}

func TestAX7_WithTimeout_Good(t *testing.T) {
	wv := ax7WebviewBase()
	err := WithTimeout(250 * time.Millisecond)(wv)
	if err != nil {
		t.Fatalf("WithTimeout returned error: %v", err)
	}
	if wv.timeout != 250*time.Millisecond {
		t.Fatalf("timeout = %v", wv.timeout)
	}
}

func TestAX7_WithTimeout_Bad(t *testing.T) {
	wv := ax7WebviewBase()
	err := WithTimeout(0)(wv)
	if err == nil {
		t.Fatal("WithTimeout accepted zero duration")
	}
	if wv.timeout != time.Second {
		t.Fatalf("timeout changed to %v", wv.timeout)
	}
}

func TestAX7_WithTimeout_Ugly(t *testing.T) {
	wv := ax7WebviewBase()
	err := WithTimeout(time.Nanosecond)(wv)
	if err != nil {
		t.Fatalf("WithTimeout rejected tiny positive duration: %v", err)
	}
	if wv.timeout != time.Nanosecond {
		t.Fatalf("timeout = %v", wv.timeout)
	}
}

func TestAX7_WithConsoleLimit_Good(t *testing.T) {
	wv := ax7WebviewBase()
	err := WithConsoleLimit(2)(wv)
	if err != nil {
		t.Fatalf("WithConsoleLimit returned error: %v", err)
	}
	if wv.consoleLimit != 2 {
		t.Fatalf("consoleLimit = %d", wv.consoleLimit)
	}
}

func TestAX7_WithConsoleLimit_Bad(t *testing.T) {
	wv := ax7WebviewBase()
	err := WithConsoleLimit(-5)(wv)
	if err != nil {
		t.Fatalf("WithConsoleLimit returned error: %v", err)
	}
	if wv.consoleLimit != 0 {
		t.Fatalf("negative limit = %d", wv.consoleLimit)
	}
}

func TestAX7_WithConsoleLimit_Ugly(t *testing.T) {
	wv := ax7WebviewBase()
	err := WithConsoleLimit(0)(wv)
	wv.addConsoleMessage(ConsoleMessage{Text: "drop"})
	if err != nil || len(wv.consoleLogs) != 0 {
		t.Fatalf("zero limit err=%v logs=%v", err, wv.consoleLogs)
	}
}

func TestAX7_WithDebugURL_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	wv := ax7WebviewBase()
	err := WithDebugURL(server.DebugURL())(wv)
	if err != nil {
		t.Fatalf("WithDebugURL returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("WithDebugURL did not install a client")
	}
}

func TestAX7_WithDebugURL_Bad(t *testing.T) {
	wv := ax7WebviewBase()
	err := WithDebugURL("http://example.com:9222")(wv)
	if err == nil {
		t.Fatal("WithDebugURL accepted remote debug host")
	}
	if wv.client != nil {
		t.Fatal("WithDebugURL installed client on failure")
	}
}

func TestAX7_WithDebugURL_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	wv := ax7WebviewBase()
	err := WithDebugURL(server.DebugURL() + "/")(wv)
	if err != nil {
		t.Fatalf("WithDebugURL rejected trailing slash: %v", err)
	}
	if wv.client == nil || wv.client.DebugURL() == "" {
		t.Fatal("WithDebugURL produced unusable client")
	}
}

func TestAX7_New_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	server.primaryTarget().onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		target.reply(msg.ID, map[string]any{})
	}
	wv, err := New(WithDebugURL(server.DebugURL()))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if wv == nil || wv.client == nil {
		t.Fatalf("New returned %#v", wv)
	}
}

func TestAX7_New_Bad(t *testing.T) {
	wv, err := New()
	if err == nil {
		t.Fatal("New succeeded without a debug URL")
	}
	if wv != nil {
		t.Fatalf("New returned webview on failure: %#v", wv)
	}
}

func TestAX7_New_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	server.primaryTarget().onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		target.reply(msg.ID, map[string]any{})
	}
	wv, err := New(WithDebugURL(server.DebugURL()), WithConsoleLimit(0))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if got := wv.GetConsole(); len(got) != 0 {
		t.Fatalf("console with zero limit = %#v", got)
	}
}

func TestAX7_NewActionSequence_Good(t *testing.T) {
	seq := NewActionSequence()
	if seq == nil {
		t.Fatal("NewActionSequence returned nil")
	}
	if len(seq.actions) != 0 {
		t.Fatalf("new sequence actions = %d", len(seq.actions))
	}
}

func TestAX7_NewActionSequence_Bad(t *testing.T) {
	left := NewActionSequence()
	right := NewActionSequence()
	if left == right {
		t.Fatal("NewActionSequence reused the same instance")
	}
	if len(left.actions) != 0 || len(right.actions) != 0 {
		t.Fatal("new sequences should start empty")
	}
}

func TestAX7_NewActionSequence_Ugly(t *testing.T) {
	seq := NewActionSequence().Wait(0)
	if seq == nil {
		t.Fatal("NewActionSequence chain returned nil")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("chained sequence actions = %d", len(seq.actions))
	}
}

func TestAX7_ActionSequence_Add_Good(t *testing.T) {
	seq := NewActionSequence()
	called := 0
	got := seq.Add(ax7OKAction{called: &called})
	if got != seq {
		t.Fatal("Add did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
}

func TestAX7_ActionSequence_Add_Bad(t *testing.T) {
	seq := NewActionSequence().Add(nil)
	defer func() {
		if recover() == nil {
			t.Fatal("Execute with nil action did not panic")
		}
	}()
	_ = seq.Execute(context.Background(), ax7WebviewBase())
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
}

func TestAX7_ActionSequence_Add_Ugly(t *testing.T) {
	var seq *ActionSequence
	defer func() {
		if recover() == nil {
			t.Fatal("Add on nil sequence did not panic")
		}
	}()
	seq.Add(ax7FailAction{})
}

func TestAX7_ActionSequence_Execute_Good(t *testing.T) {
	called := 0
	seq := NewActionSequence().Add(ax7OKAction{called: &called}).Add(ax7OKAction{called: &called})
	err := seq.Execute(context.Background(), ax7WebviewBase())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if called != 2 {
		t.Fatalf("called = %d", called)
	}
}

func TestAX7_ActionSequence_Execute_Bad(t *testing.T) {
	seq := NewActionSequence().Add(ax7FailAction{})
	err := seq.Execute(context.Background(), ax7WebviewBase())
	if err == nil {
		t.Fatal("Execute succeeded despite failing action")
	}
	if !strings.Contains(err.Error(), "action index 0 failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_ActionSequence_Execute_Ugly(t *testing.T) {
	seq := NewActionSequence()
	err := seq.Execute(context.Background(), ax7WebviewBase())
	if err != nil {
		t.Fatalf("empty Execute returned error: %v", err)
	}
	if len(seq.actions) != 0 {
		t.Fatalf("empty sequence mutated to %d actions", len(seq.actions))
	}
}

func TestAX7_ActionSequence_Click_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Click("#save")
	if got != seq {
		t.Fatal("Click did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#save" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Click_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Click("")
	if got != seq {
		t.Fatal("Click did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Click_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Click("button[data-x=\"1\"]")
	if got != seq {
		t.Fatal("Click did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "button[data-x=\"1\"]" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Type_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Type("#email", "a@b.test")
	if got != seq {
		t.Fatal("Type did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(TypeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#email" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Type_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Type("", "")
	if got != seq {
		t.Fatal("Type did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(TypeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Text != "" {
		t.Fatalf("Text = %#v", action.Text)
	}
}

func TestAX7_ActionSequence_Type_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Type("textarea", "line\nnext")
	if got != seq {
		t.Fatal("Type did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(TypeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Text != "line\nnext" {
		t.Fatalf("Text = %#v", action.Text)
	}
}

func TestAX7_ActionSequence_Navigate_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Navigate("https://example.com")
	if got != seq {
		t.Fatal("Navigate did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(NavigateAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.URL != "https://example.com" {
		t.Fatalf("URL = %#v", action.URL)
	}
}

func TestAX7_ActionSequence_Navigate_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Navigate("javascript:alert(1)")
	if got != seq {
		t.Fatal("Navigate did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(NavigateAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.URL != "javascript:alert(1)" {
		t.Fatalf("URL = %#v", action.URL)
	}
}

func TestAX7_ActionSequence_Navigate_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Navigate("about:blank")
	if got != seq {
		t.Fatal("Navigate did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(NavigateAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.URL != "about:blank" {
		t.Fatalf("URL = %#v", action.URL)
	}
}

func TestAX7_ActionSequence_Wait_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Wait(5 * time.Millisecond)
	if got != seq {
		t.Fatal("Wait did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(WaitAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Duration != 5*time.Millisecond {
		t.Fatalf("Duration = %#v", action.Duration)
	}
}

func TestAX7_ActionSequence_Wait_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Wait(-1 * time.Millisecond)
	if got != seq {
		t.Fatal("Wait did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(WaitAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Duration != -1*time.Millisecond {
		t.Fatalf("Duration = %#v", action.Duration)
	}
}

func TestAX7_ActionSequence_Wait_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Wait(0)
	if got != seq {
		t.Fatal("Wait did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(WaitAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Duration != 0 {
		t.Fatalf("Duration = %#v", action.Duration)
	}
}

func TestAX7_ActionSequence_WaitForSelector_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.WaitForSelector("#ready")
	if got != seq {
		t.Fatal("WaitForSelector did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(WaitForSelectorAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#ready" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_WaitForSelector_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.WaitForSelector("")
	if got != seq {
		t.Fatal("WaitForSelector did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(WaitForSelectorAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_WaitForSelector_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.WaitForSelector("[data-ready=true]")
	if got != seq {
		t.Fatal("WaitForSelector did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(WaitForSelectorAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "[data-ready=true]" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Scroll_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Scroll(1, 2)
	if got != seq {
		t.Fatal("Scroll did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ScrollAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.X != 1 {
		t.Fatalf("X = %#v", action.X)
	}
}

func TestAX7_ActionSequence_Scroll_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Scroll(-1, -2)
	if got != seq {
		t.Fatal("Scroll did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ScrollAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Y != -2 {
		t.Fatalf("Y = %#v", action.Y)
	}
}

func TestAX7_ActionSequence_Scroll_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Scroll(0, 0)
	if got != seq {
		t.Fatal("Scroll did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ScrollAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.X != 0 {
		t.Fatalf("X = %#v", action.X)
	}
}

func TestAX7_ActionSequence_ScrollIntoView_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.ScrollIntoView("#target")
	if got != seq {
		t.Fatal("ScrollIntoView did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ScrollIntoViewAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#target" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_ScrollIntoView_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.ScrollIntoView("")
	if got != seq {
		t.Fatal("ScrollIntoView did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ScrollIntoViewAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_ScrollIntoView_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.ScrollIntoView("main > .item")
	if got != seq {
		t.Fatal("ScrollIntoView did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ScrollIntoViewAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "main > .item" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Focus_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Focus("#input")
	if got != seq {
		t.Fatal("Focus did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(FocusAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#input" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Focus_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Focus("")
	if got != seq {
		t.Fatal("Focus did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(FocusAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Focus_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Focus("input[name=email]")
	if got != seq {
		t.Fatal("Focus did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(FocusAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "input[name=email]" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Blur_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Blur("#input")
	if got != seq {
		t.Fatal("Blur did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(BlurAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#input" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Blur_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Blur("")
	if got != seq {
		t.Fatal("Blur did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(BlurAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Blur_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Blur("input[name=email]")
	if got != seq {
		t.Fatal("Blur did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(BlurAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "input[name=email]" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Clear_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Clear("#input")
	if got != seq {
		t.Fatal("Clear did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ClearAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#input" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Clear_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Clear("")
	if got != seq {
		t.Fatal("Clear did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ClearAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Clear_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Clear("textarea")
	if got != seq {
		t.Fatal("Clear did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(ClearAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "textarea" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Select_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Select("select", "a")
	if got != seq {
		t.Fatal("Select did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SelectAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "a" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_Select_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Select("select", "")
	if got != seq {
		t.Fatal("Select did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SelectAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_Select_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Select("select", "1 2")
	if got != seq {
		t.Fatal("Select did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SelectAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "1 2" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_Check_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Check("#ok", true)
	if got != seq {
		t.Fatal("Check did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(CheckAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Checked != true {
		t.Fatalf("Checked = %#v", action.Checked)
	}
}

func TestAX7_ActionSequence_Check_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Check("#ok", false)
	if got != seq {
		t.Fatal("Check did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(CheckAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Checked != false {
		t.Fatalf("Checked = %#v", action.Checked)
	}
}

func TestAX7_ActionSequence_Check_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Check("", true)
	if got != seq {
		t.Fatal("Check did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(CheckAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Hover_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Hover("#menu")
	if got != seq {
		t.Fatal("Hover did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(HoverAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#menu" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Hover_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Hover("")
	if got != seq {
		t.Fatal("Hover did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(HoverAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_Hover_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.Hover("[role=menuitem]")
	if got != seq {
		t.Fatal("Hover did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(HoverAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "[role=menuitem]" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_DoubleClick_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.DoubleClick("#item")
	if got != seq {
		t.Fatal("DoubleClick did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(DoubleClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#item" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_DoubleClick_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.DoubleClick("")
	if got != seq {
		t.Fatal("DoubleClick did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(DoubleClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_DoubleClick_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.DoubleClick(".row:first-child")
	if got != seq {
		t.Fatal("DoubleClick did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(DoubleClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != ".row:first-child" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_RightClick_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.RightClick("#item")
	if got != seq {
		t.Fatal("RightClick did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(RightClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#item" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_RightClick_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.RightClick("")
	if got != seq {
		t.Fatal("RightClick did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(RightClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_RightClick_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.RightClick(".row:first-child")
	if got != seq {
		t.Fatal("RightClick did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(RightClickAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != ".row:first-child" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_PressKey_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.PressKey("Enter")
	if got != seq {
		t.Fatal("PressKey did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(PressKeyAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Key != "Enter" {
		t.Fatalf("Key = %#v", action.Key)
	}
}

func TestAX7_ActionSequence_PressKey_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.PressKey("")
	if got != seq {
		t.Fatal("PressKey did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(PressKeyAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Key != "" {
		t.Fatalf("Key = %#v", action.Key)
	}
}

func TestAX7_ActionSequence_PressKey_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.PressKey("a")
	if got != seq {
		t.Fatal("PressKey did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(PressKeyAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Key != "a" {
		t.Fatalf("Key = %#v", action.Key)
	}
}

func TestAX7_ActionSequence_SetAttribute_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.SetAttribute("#x", "data-id", "1")
	if got != seq {
		t.Fatal("SetAttribute did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SetAttributeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Attribute != "data-id" {
		t.Fatalf("Attribute = %#v", action.Attribute)
	}
}

func TestAX7_ActionSequence_SetAttribute_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.SetAttribute("#x", "", "")
	if got != seq {
		t.Fatal("SetAttribute did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SetAttributeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_SetAttribute_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.SetAttribute("#x", "aria-label", "A B")
	if got != seq {
		t.Fatal("SetAttribute did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SetAttributeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "A B" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_RemoveAttribute_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.RemoveAttribute("#x", "disabled")
	if got != seq {
		t.Fatal("RemoveAttribute did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(RemoveAttributeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Attribute != "disabled" {
		t.Fatalf("Attribute = %#v", action.Attribute)
	}
}

func TestAX7_ActionSequence_RemoveAttribute_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.RemoveAttribute("#x", "")
	if got != seq {
		t.Fatal("RemoveAttribute did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(RemoveAttributeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Attribute != "" {
		t.Fatalf("Attribute = %#v", action.Attribute)
	}
}

func TestAX7_ActionSequence_RemoveAttribute_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.RemoveAttribute("#x", "data-id")
	if got != seq {
		t.Fatal("RemoveAttribute did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(RemoveAttributeAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Attribute != "data-id" {
		t.Fatalf("Attribute = %#v", action.Attribute)
	}
}

func TestAX7_ActionSequence_SetValue_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.SetValue("#x", "ready")
	if got != seq {
		t.Fatal("SetValue did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SetValueAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "ready" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_SetValue_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.SetValue("#x", "")
	if got != seq {
		t.Fatal("SetValue did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SetValueAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_SetValue_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.SetValue("#x", "line\nnext")
	if got != seq {
		t.Fatal("SetValue did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(SetValueAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Value != "line\nnext" {
		t.Fatalf("Value = %#v", action.Value)
	}
}

func TestAX7_ActionSequence_UploadFile_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.UploadFile("#file", []string{"/tmp/a.txt"})
	if got != seq {
		t.Fatal("UploadFile did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(UploadFileAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.Selector != "#file" {
		t.Fatalf("Selector = %#v", action.Selector)
	}
}

func TestAX7_ActionSequence_UploadFile_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.UploadFile("", nil)
	if got != seq {
		t.Fatal("UploadFile did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(UploadFileAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.FilePaths != nil {
		t.Fatalf("FilePaths = %#v", action.FilePaths)
	}
}

func TestAX7_ActionSequence_UploadFile_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.UploadFile("#file", []string{"/tmp/a.txt", "/tmp/b.txt"})
	if got != seq {
		t.Fatal("UploadFile did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(UploadFileAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if len(action.FilePaths) != 2 || action.FilePaths[0] != "/tmp/a.txt" || action.FilePaths[1] != "/tmp/b.txt" {
		t.Fatalf("FilePaths = %#v", action.FilePaths)
	}
}

func TestAX7_ActionSequence_DragAndDrop_Good(t *testing.T) {
	seq := NewActionSequence()
	got := seq.DragAndDrop("#a", "#b")
	if got != seq {
		t.Fatal("DragAndDrop did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(DragAndDropAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.SourceSelector != "#a" {
		t.Fatalf("SourceSelector = %#v", action.SourceSelector)
	}
}

func TestAX7_ActionSequence_DragAndDrop_Bad(t *testing.T) {
	seq := NewActionSequence()
	got := seq.DragAndDrop("", "")
	if got != seq {
		t.Fatal("DragAndDrop did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(DragAndDropAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.TargetSelector != "" {
		t.Fatalf("TargetSelector = %#v", action.TargetSelector)
	}
}

func TestAX7_ActionSequence_DragAndDrop_Ugly(t *testing.T) {
	seq := NewActionSequence()
	got := seq.DragAndDrop("[draggable]", "#drop")
	if got != seq {
		t.Fatal("DragAndDrop did not return the receiver")
	}
	if len(seq.actions) != 1 {
		t.Fatalf("actions = %d", len(seq.actions))
	}
	action, ok := seq.actions[0].(DragAndDropAction)
	if !ok {
		t.Fatalf("action type = %T", seq.actions[0])
	}
	if action.SourceSelector != "[draggable]" {
		t.Fatalf("SourceSelector = %#v", action.SourceSelector)
	}
}

func TestAX7_ScrollAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "window.scrollTo(3, 4)", false)
	err := (ScrollAction{X: 3, Y: 4}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ScrollAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_ScrollAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (ScrollAction{X: 3, Y: 4}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("ScrollAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_ScrollAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (ScrollAction{X: 0, Y: 0}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ScrollAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_ScrollIntoViewAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "scrollIntoView", false)
	err := (ScrollIntoViewAction{Selector: "#target"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ScrollIntoViewAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_ScrollIntoViewAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (ScrollIntoViewAction{Selector: "#target"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("ScrollIntoViewAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_ScrollIntoViewAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (ScrollIntoViewAction{Selector: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ScrollIntoViewAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_FocusAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "focus()", false)
	err := (FocusAction{Selector: "#input"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("FocusAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_FocusAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (FocusAction{Selector: "#input"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("FocusAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_FocusAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (FocusAction{Selector: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("FocusAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_BlurAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "blur()", false)
	err := (BlurAction{Selector: "#input"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("BlurAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_BlurAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (BlurAction{Selector: "#input"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("BlurAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_BlurAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (BlurAction{Selector: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("BlurAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_ClearAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "el.value =", false)
	err := (ClearAction{Selector: "#input"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ClearAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_ClearAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (ClearAction{Selector: "#input"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("ClearAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_ClearAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (ClearAction{Selector: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ClearAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_SelectAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, `el.value = "one"`, false)
	err := (SelectAction{Selector: "select", Value: "one"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("SelectAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_SelectAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (SelectAction{Selector: "select", Value: "one"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("SelectAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_SelectAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (SelectAction{Selector: "select", Value: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("SelectAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_CheckAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "checked !== true", false)
	err := (CheckAction{Selector: "#agree", Checked: true}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("CheckAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_CheckAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (CheckAction{Selector: "#agree", Checked: true}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("CheckAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_CheckAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (CheckAction{Selector: "", Checked: false}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("CheckAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_SetAttributeAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "setAttribute", false)
	err := (SetAttributeAction{Selector: "#x", Attribute: "data-id", Value: "1"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("SetAttributeAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_SetAttributeAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (SetAttributeAction{Selector: "#x", Attribute: "data-id", Value: "1"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("SetAttributeAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_SetAttributeAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (SetAttributeAction{Selector: "#x", Attribute: "", Value: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("SetAttributeAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_RemoveAttributeAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "removeAttribute", false)
	err := (RemoveAttributeAction{Selector: "#x", Attribute: "disabled"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("RemoveAttributeAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_RemoveAttributeAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (RemoveAttributeAction{Selector: "#x", Attribute: "disabled"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("RemoveAttributeAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_RemoveAttributeAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (RemoveAttributeAction{Selector: "#x", Attribute: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("RemoveAttributeAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_SetValueAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, `el.value = "ready"`, false)
	err := (SetValueAction{Selector: "#x", Value: "ready"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("SetValueAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_SetValueAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", true)
	err := (SetValueAction{Selector: "#x", Value: "ready"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("SetValueAction.Execute succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_SetValueAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "", false)
	err := (SetValueAction{Selector: "#x", Value: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("SetValueAction.Execute rejected boundary input: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_ClickAction_Execute_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	err := (ClickAction{Selector: "#button"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ClickAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_ClickAction_Execute_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	err := (ClickAction{Selector: "#missing"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("ClickAction.Execute succeeded for missing element")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_ClickAction_Execute_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	err := (ClickAction{Selector: "#button"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("ClickAction.Execute fallback returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_HoverAction_Execute_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	err := (HoverAction{Selector: "#menu"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("HoverAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_HoverAction_Execute_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	err := (HoverAction{Selector: "#missing"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("HoverAction.Execute succeeded for missing element")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_HoverAction_Execute_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	err := (HoverAction{Selector: "#menu"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("HoverAction.Execute succeeded without bounding box")
	}
	if !strings.Contains(err.Error(), "bounding box") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_DoubleClickAction_Execute_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	err := (DoubleClickAction{Selector: "#item"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("DoubleClickAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_DoubleClickAction_Execute_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	err := (DoubleClickAction{Selector: "#missing"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("DoubleClickAction.Execute succeeded for missing element")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_DoubleClickAction_Execute_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	err := (DoubleClickAction{Selector: "#item"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("DoubleClickAction.Execute fallback returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_RightClickAction_Execute_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	err := (RightClickAction{Selector: "#item"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("RightClickAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_RightClickAction_Execute_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	err := (RightClickAction{Selector: "#missing"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("RightClickAction.Execute succeeded for missing element")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_RightClickAction_Execute_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	err := (RightClickAction{Selector: "#item"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("RightClickAction.Execute fallback returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_TypeAction_Execute_Good(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Runtime.evaluate", "Input.dispatchKeyEvent":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": true}})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	err := (TypeAction{Selector: "#email", Text: "ab"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("TypeAction.Execute returned error: %v", err)
	}
}

func TestAX7_TypeAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "focus", true)
	err := (TypeAction{Selector: "#email", Text: "ab"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("TypeAction.Execute succeeded despite focus failure")
	}
	if !strings.Contains(err.Error(), "focus") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_TypeAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "focus", false)
	err := (TypeAction{Selector: "#email", Text: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("TypeAction.Execute rejected empty text: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_NavigateAction_Execute_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	err := (NavigateAction{URL: "https://example.com"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("NavigateAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_NavigateAction_Execute_Bad(t *testing.T) {
	wv := ax7NavigationWebview(t)
	err := (NavigateAction{URL: "javascript:alert(1)"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("NavigateAction.Execute accepted dangerous URL")
	}
	if !strings.Contains(err.Error(), "navigation URL") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_NavigateAction_Execute_Ugly(t *testing.T) {
	wv := ax7NavigationWebview(t)
	err := (NavigateAction{URL: "about:blank"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("NavigateAction.Execute rejected about:blank: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_WaitAction_Execute_Good(t *testing.T) {
	err := (WaitAction{Duration: time.Millisecond}).Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("WaitAction.Execute returned error: %v", err)
	}
	if time.Millisecond <= 0 {
		t.Fatal("invalid test duration")
	}
}

func TestAX7_WaitAction_Execute_Bad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (WaitAction{Duration: time.Second}).Execute(ctx, nil)
	if err == nil {
		t.Fatal("WaitAction.Execute ignored canceled context")
	}
}

func TestAX7_WaitAction_Execute_Ugly(t *testing.T) {
	err := (WaitAction{Duration: 0}).Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("WaitAction.Execute rejected zero duration: %v", err)
	}
	if (WaitAction{Duration: 0}).Duration != 0 {
		t.Fatal("zero duration mutated")
	}
}

func TestAX7_WaitForSelectorAction_Execute_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "querySelector", false)
	err := (WaitForSelectorAction{Selector: "#ready"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("WaitForSelectorAction.Execute returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_WaitForSelectorAction_Execute_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "querySelector", true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := (WaitForSelectorAction{Selector: "#never"}).Execute(ctx, wv)
	if err == nil {
		t.Fatal("WaitForSelectorAction.Execute succeeded without selector")
	}
}

func TestAX7_WaitForSelectorAction_Execute_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "querySelector", false)
	err := (WaitForSelectorAction{Selector: ""}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("WaitForSelectorAction.Execute rejected empty selector: %v", err)
	}
	if wv.timeout <= 0 {
		t.Fatalf("fixture timeout = %v", wv.timeout)
	}
}

func TestAX7_PressKeyAction_Execute_Good(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Input.dispatchKeyEvent" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.reply(msg.ID, map[string]any{})
	})
	err := (PressKeyAction{Key: "Enter"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("PressKeyAction.Execute returned error: %v", err)
	}
}

func TestAX7_PressKeyAction_Execute_Bad(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyError(msg.ID, "key failed") })
	err := (PressKeyAction{Key: "Enter"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("PressKeyAction.Execute succeeded despite CDP error")
	}
}

func TestAX7_PressKeyAction_Execute_Ugly(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Input.dispatchKeyEvent" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.reply(msg.ID, map[string]any{})
	})
	err := (PressKeyAction{Key: "a"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("PressKeyAction.Execute rejected simple key: %v", err)
	}
}

func TestAX7_UploadFileAction_Execute_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	err := (UploadFileAction{Selector: "#file", FilePaths: []string{"/tmp/a.txt"}}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("UploadFileAction.Execute returned error: %v", err)
	}
}

func TestAX7_UploadFileAction_Execute_Bad(t *testing.T) {
	err := (UploadFileAction{Selector: "#file"}).Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("UploadFileAction.Execute accepted nil webview")
	}
	if !strings.Contains(err.Error(), "webview") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_UploadFileAction_Execute_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	err := (UploadFileAction{Selector: "#file", FilePaths: nil}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("UploadFileAction.Execute rejected nil file list: %v", err)
	}
}

func TestAX7_DragAndDropAction_Execute_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	err := (DragAndDropAction{SourceSelector: "#a", TargetSelector: "#b"}).Execute(context.Background(), wv)
	if err != nil {
		t.Fatalf("DragAndDropAction.Execute returned error: %v", err)
	}
}

func TestAX7_DragAndDropAction_Execute_Bad(t *testing.T) {
	err := (DragAndDropAction{SourceSelector: "#a", TargetSelector: "#b"}).Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("DragAndDropAction.Execute accepted nil webview")
	}
	if !strings.Contains(err.Error(), "webview") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_DragAndDropAction_Execute_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	err := (DragAndDropAction{SourceSelector: "#a", TargetSelector: "#b"}).Execute(context.Background(), wv)
	if err == nil {
		t.Fatal("DragAndDropAction.Execute succeeded without source box")
	}
}

func TestAX7_NewCDPClient_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}
	if client.DebugURL() != server.DebugURL() {
		t.Fatalf("DebugURL = %q", client.DebugURL())
	}
}

func TestAX7_NewCDPClient_Bad(t *testing.T) {
	client, err := NewCDPClient("http://example.com:9222")
	if err == nil {
		t.Fatal("NewCDPClient accepted remote endpoint")
	}
	if client != nil {
		t.Fatalf("client = %#v", client)
	}
}

func TestAX7_NewCDPClient_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	server.mu.Lock()
	server.targets = map[string]*fakeCDPTarget{}
	server.mu.Unlock()
	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient did not auto-create target: %v", err)
	}
	if client.WebSocketURL() == "" {
		t.Fatal("auto-created client has empty WebSocketURL")
	}
}

func TestAX7_CDPClient_Call_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) { target.reply(msg.ID, map[string]any{"ok": true}) }
	client := newConnectedCDPClient(t, target)
	got, err := client.Call(context.Background(), "Runtime.evaluate", nil)
	if err != nil || got["ok"] != true {
		t.Fatalf("Call = %#v, %v", got, err)
	}
}

func TestAX7_CDPClient_Call_Bad(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) { target.replyError(msg.ID, "refused") }
	client := newConnectedCDPClient(t, target)
	_, err := client.Call(context.Background(), "Runtime.evaluate", nil)
	if err == nil {
		t.Fatal("Call succeeded after CDP error")
	}
}

func TestAX7_CDPClient_Call_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Call(ctx, "Runtime.evaluate", map[string]any{"nested": map[string]any{"x": 1}})
	if err == nil {
		t.Fatal("Call ignored canceled context")
	}
}

func TestAX7_CDPClient_Send_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	seen := make(chan cdpMessage, 1)
	target.onMessage = func(_ *fakeCDPTarget, msg cdpMessage) { seen <- msg }
	client := newConnectedCDPClient(t, target)
	if err := client.Send("Page.enable", map[string]any{"x": "y"}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if (<-seen).Method != "Page.enable" {
		t.Fatal("Send delivered wrong method")
	}
}

func TestAX7_CDPClient_Send_Bad(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	if err := client.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := client.Send("Page.enable", nil); err == nil {
		t.Fatal("Send succeeded after Close")
	}
}

func TestAX7_CDPClient_Send_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	seen := make(chan cdpMessage, 1)
	target.onMessage = func(_ *fakeCDPTarget, msg cdpMessage) { seen <- msg }
	client := newConnectedCDPClient(t, target)
	if err := client.Send("Runtime.enable", nil); err != nil {
		t.Fatalf("Send nil params returned error: %v", err)
	}
	if (<-seen).Params != nil {
		t.Fatal("Send nil params encoded unexpected params")
	}
}

func TestAX7_CDPClient_Close_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	if err := client.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if client.ctx.Err() == nil {
		t.Fatal("Close did not cancel context")
	}
}

func TestAX7_CDPClient_Close_Bad(t *testing.T) {
	var client *CDPClient
	defer func() {
		if recover() == nil {
			t.Fatal("Close on nil client did not panic")
		}
	}()
	client.Close()
}

func TestAX7_CDPClient_Close_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	if err := client.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestAX7_CDPClient_CloseTab_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) { target.reply(msg.ID, map[string]any{"success": true}) }
	client := newConnectedCDPClient(t, target)
	if err := client.CloseTab(); err != nil {
		t.Fatalf("CloseTab returned error: %v", err)
	}
}

func TestAX7_CDPClient_CloseTab_Bad(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) { target.reply(msg.ID, map[string]any{"success": false}) }
	client := newConnectedCDPClient(t, target)
	if err := client.CloseTab(); err == nil {
		t.Fatal("CloseTab ignored failed acknowledgement")
	}
}

func TestAX7_CDPClient_CloseTab_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	if err := client.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := client.CloseTab(); err == nil {
		t.Fatal("CloseTab succeeded after client close")
	}
}

func TestAX7_CDPClient_DebugURL_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	if client.DebugURL() != server.DebugURL() {
		t.Fatalf("DebugURL = %q", client.DebugURL())
	}
}

func TestAX7_CDPClient_DebugURL_Bad(t *testing.T) {
	client := &CDPClient{}
	if client.DebugURL() != "" {
		t.Fatalf("zero DebugURL = %q", client.DebugURL())
	}
	if client.WebSocketURL() != "" {
		t.Fatalf("zero WebSocketURL = %q", client.WebSocketURL())
	}
}

func TestAX7_CDPClient_DebugURL_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	_ = client.Close()
	if client.DebugURL() != server.DebugURL() {
		t.Fatalf("DebugURL changed after close: %q", client.DebugURL())
	}
}

func TestAX7_CDPClient_WebSocketURL_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	if !strings.Contains(client.WebSocketURL(), "/devtools/page/") {
		t.Fatalf("WebSocketURL = %q", client.WebSocketURL())
	}
}

func TestAX7_CDPClient_WebSocketURL_Bad(t *testing.T) {
	client := &CDPClient{}
	if client.WebSocketURL() != "" {
		t.Fatalf("zero WebSocketURL = %q", client.WebSocketURL())
	}
	if client.DebugURL() != "" {
		t.Fatalf("zero DebugURL = %q", client.DebugURL())
	}
}

func TestAX7_CDPClient_WebSocketURL_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	before := client.WebSocketURL()
	_ = client.Close()
	if client.WebSocketURL() != before {
		t.Fatalf("WebSocketURL changed after close: %q", client.WebSocketURL())
	}
}

func TestAX7_CDPClient_NewTab_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	tab, err := client.NewTab("about:blank")
	if err != nil {
		t.Fatalf("NewTab returned error: %v", err)
	}
	if tab.WebSocketURL() == "" {
		t.Fatal("NewTab returned empty WebSocketURL")
	}
}

func TestAX7_CDPClient_NewTab_Bad(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	_, err := client.NewTab("javascript:alert(1)")
	if err == nil {
		t.Fatal("NewTab accepted dangerous URL")
	}
}

func TestAX7_CDPClient_NewTab_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	tab, err := client.NewTab("")
	if err != nil {
		t.Fatalf("NewTab rejected empty URL: %v", err)
	}
	if tab.DebugURL() != server.DebugURL() {
		t.Fatalf("tab DebugURL = %q", tab.DebugURL())
	}
}

func TestAX7_CDPClient_OnEvent_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	client := newConnectedCDPClient(t, target)
	seen := make(chan map[string]any, 1)
	client.OnEvent("Runtime.consoleAPICalled", func(params map[string]any) { seen <- params })
	target.writeJSON(cdpEvent{Method: "Runtime.consoleAPICalled", Params: map[string]any{"type": "log"}})
	if (<-seen)["type"] != "log" {
		t.Fatal("OnEvent handler did not receive params")
	}
}

func TestAX7_CDPClient_OnEvent_Bad(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	client.OnEvent("", func(map[string]any) {})
	if len(client.handlers[""]) != 1 {
		t.Fatalf("empty event handlers = %d", len(client.handlers[""]))
	}
}

func TestAX7_CDPClient_OnEvent_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	client.OnEvent("Runtime.event", func(map[string]any) {})
	client.OnEvent("Runtime.event", func(map[string]any) {})
	if len(client.handlers["Runtime.event"]) != 2 {
		t.Fatalf("handlers = %d", len(client.handlers["Runtime.event"]))
	}
}

func TestAX7_ListTargets_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	targets, err := ListTargets(server.DebugURL())
	if err != nil {
		t.Fatalf("ListTargets returned error: %v", err)
	}
	if len(targets) != 1 || targets[0].Type != "page" {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestAX7_ListTargets_Bad(t *testing.T) {
	targets, err := ListTargets("http://example.com:9222")
	if err == nil {
		t.Fatal("ListTargets accepted remote URL")
	}
	if targets != nil {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestAX7_ListTargets_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	server.addTarget("target-ugly")
	targets, err := ListTargets(server.DebugURL())
	if err != nil {
		t.Fatalf("ListTargets returned error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestAX7_ListTargetsAll_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	var targets []TargetInfo
	for target := range ListTargetsAll(server.DebugURL()) {
		targets = append(targets, target)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestAX7_ListTargetsAll_Bad(t *testing.T) {
	var targets []TargetInfo
	for target := range ListTargetsAll("http://example.com:9222") {
		targets = append(targets, target)
	}
	if len(targets) != 0 {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestAX7_ListTargetsAll_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	seen := 0
	for range ListTargetsAll(server.DebugURL()) {
		seen++
		break
	}
	if seen != 1 {
		t.Fatalf("seen = %d", seen)
	}
}

func TestAX7_GetVersion_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	version, err := GetVersion(server.DebugURL())
	if err != nil {
		t.Fatalf("GetVersion returned error: %v", err)
	}
	if version["Browser"] != "Chrome/123.0" {
		t.Fatalf("version = %#v", version)
	}
}

func TestAX7_GetVersion_Bad(t *testing.T) {
	version, err := GetVersion("http://example.com:9222")
	if err == nil {
		t.Fatal("GetVersion accepted remote URL")
	}
	if version != nil {
		t.Fatalf("version = %#v", version)
	}
}

func TestAX7_GetVersion_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	version, err := GetVersion(server.DebugURL() + "/")
	if err != nil {
		t.Fatalf("GetVersion rejected trailing slash: %v", err)
	}
	if version["Browser"] == "" {
		t.Fatalf("version = %#v", version)
	}
}

func TestAX7_URL_String_Good(t *testing.T) {
	u := &cdpURL{Scheme: "http", Host: "localhost:9222", Path: "/"}
	if got := u.String(); got != "http://localhost:9222/" {
		t.Fatalf("String = %q", got)
	}
	if u.Host != "localhost:9222" {
		t.Fatalf("Host = %q", u.Host)
	}
}

func TestAX7_URL_String_Bad(t *testing.T) {
	var u *cdpURL
	if got := u.String(); got != "" {
		t.Fatalf("nil String = %q", got)
	}
	if u != nil {
		t.Fatal("nil URL mutated")
	}
}

func TestAX7_URL_String_Ugly(t *testing.T) {
	u := &cdpURL{Scheme: "http", Host: "localhost", Path: "/json", RawQuery: "a=b", Fragment: "top"}
	if got := u.String(); got != "http://localhost/json?a=b#top" {
		t.Fatalf("String = %q", got)
	}
	if u.RawQuery != "a=b" {
		t.Fatalf("RawQuery = %q", u.RawQuery)
	}
}

func TestAX7_URL_Hostname_Good(t *testing.T) {
	u := &cdpURL{hostname: "localhost"}
	if got := u.Hostname(); got != "localhost" {
		t.Fatalf("Hostname = %q", got)
	}
	if u.hostname == "" {
		t.Fatal("hostname fixture empty")
	}
}

func TestAX7_URL_Hostname_Bad(t *testing.T) {
	var u *cdpURL
	if got := u.Hostname(); got != "" {
		t.Fatalf("nil Hostname = %q", got)
	}
	if u != nil {
		t.Fatal("nil URL mutated")
	}
}

func TestAX7_URL_Hostname_Ugly(t *testing.T) {
	u := &cdpURL{hostname: "::1"}
	if got := u.Hostname(); got != "::1" {
		t.Fatalf("Hostname = %q", got)
	}
	if !strings.Contains(u.hostname, ":") {
		t.Fatalf("hostname = %q", u.hostname)
	}
}

func TestAX7_URL_Port_Good(t *testing.T) {
	u := &cdpURL{port: "9222"}
	if got := u.Port(); got != "9222" {
		t.Fatalf("Port = %q", got)
	}
	if u.port == "" {
		t.Fatal("port fixture empty")
	}
}

func TestAX7_URL_Port_Bad(t *testing.T) {
	var u *cdpURL
	if got := u.Port(); got != "" {
		t.Fatalf("nil Port = %q", got)
	}
	if u != nil {
		t.Fatal("nil URL mutated")
	}
}

func TestAX7_URL_Port_Ugly(t *testing.T) {
	u := &cdpURL{port: ""}
	if got := u.Port(); got != "" {
		t.Fatalf("empty Port = %q", got)
	}
	if u.String() != "" {
		t.Fatalf("empty URL String = %q", u.String())
	}
}

func TestAX7_ConsoleWatcher_Messages_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	got = cw.Messages()
	if len(got) != 2 {
		t.Fatalf("Messages len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Messages_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	got = cw.Messages()
	if len(got) != 0 {
		t.Fatalf("empty Messages len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Messages_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	got = cw.Messages()
	if len(got) != 1 {
		t.Fatalf("ugly Messages len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_MessagesAll_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	for msg := range cw.MessagesAll() {
		got = append(got, msg)
	}
	if len(got) != 2 {
		t.Fatalf("MessagesAll len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_MessagesAll_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	for msg := range cw.MessagesAll() {
		got = append(got, msg)
	}
	if len(got) != 0 {
		t.Fatalf("empty MessagesAll len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_MessagesAll_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	for msg := range cw.MessagesAll() {
		got = append(got, msg)
		break
	}
	if len(got) != 1 {
		t.Fatalf("ugly MessagesAll len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_FilteredMessages_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	got = cw.FilteredMessages()
	if len(got) != 2 {
		t.Fatalf("FilteredMessages len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_FilteredMessages_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	got = cw.FilteredMessages()
	if len(got) != 0 {
		t.Fatalf("empty FilteredMessages len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_FilteredMessages_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	got = cw.FilteredMessages()
	if len(got) != 1 {
		t.Fatalf("ugly FilteredMessages len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_FilteredMessagesAll_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	for msg := range cw.FilteredMessagesAll() {
		got = append(got, msg)
	}
	if len(got) != 2 {
		t.Fatalf("FilteredMessagesAll len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_FilteredMessagesAll_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	for msg := range cw.FilteredMessagesAll() {
		got = append(got, msg)
	}
	if len(got) != 0 {
		t.Fatalf("empty FilteredMessagesAll len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_FilteredMessagesAll_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	for msg := range cw.FilteredMessagesAll() {
		got = append(got, msg)
		break
	}
	if len(got) != 1 {
		t.Fatalf("ugly FilteredMessagesAll len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Errors_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	got = cw.Errors()
	if len(got) != 1 {
		t.Fatalf("Errors len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Errors_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	got = cw.Errors()
	if len(got) != 0 {
		t.Fatalf("empty Errors len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Errors_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	got = cw.Errors()
	if len(got) != 0 {
		t.Fatalf("ugly Errors len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_ErrorsAll_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	for msg := range cw.ErrorsAll() {
		got = append(got, msg)
	}
	if len(got) != 1 {
		t.Fatalf("ErrorsAll len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_ErrorsAll_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	for msg := range cw.ErrorsAll() {
		got = append(got, msg)
	}
	if len(got) != 0 {
		t.Fatalf("empty ErrorsAll len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_ErrorsAll_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	for msg := range cw.ErrorsAll() {
		got = append(got, msg)
		break
	}
	if len(got) != 0 {
		t.Fatalf("ugly ErrorsAll len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Warnings_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	got = cw.Warnings()
	if len(got) != 1 {
		t.Fatalf("Warnings len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Warnings_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	got = cw.Warnings()
	if len(got) != 0 {
		t.Fatalf("empty Warnings len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Warnings_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	got = cw.Warnings()
	if len(got) != 1 {
		t.Fatalf("ugly Warnings len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_WarningsAll_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"}, ConsoleMessage{Type: "warning", Text: "careful"})
	var got []ConsoleMessage
	for msg := range cw.WarningsAll() {
		got = append(got, msg)
	}
	if len(got) != 1 {
		t.Fatalf("WarningsAll len = %d", len(got))
	}
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_WarningsAll_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	var got []ConsoleMessage
	for msg := range cw.WarningsAll() {
		got = append(got, msg)
	}
	if len(got) != 0 {
		t.Fatalf("empty WarningsAll len = %d", len(got))
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_WarningsAll_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warn", Text: "legacy"})
	var got []ConsoleMessage
	for msg := range cw.WarningsAll() {
		got = append(got, msg)
		break
	}
	if len(got) != 1 {
		t.Fatalf("ugly WarningsAll len = %d", len(got))
	}
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_NewConsoleWatcher_Good(t *testing.T) {
	cw := NewConsoleWatcher(nil)
	if cw == nil {
		t.Fatal("NewConsoleWatcher returned nil")
	}
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_NewConsoleWatcher_Bad(t *testing.T) {
	cw := NewConsoleWatcher(&Webview{})
	if cw == nil {
		t.Fatal("NewConsoleWatcher returned nil for empty webview")
	}
	if len(cw.handlers) != 0 {
		t.Fatalf("handlers = %d", len(cw.handlers))
	}
}

func TestAX7_NewConsoleWatcher_Ugly(t *testing.T) {
	wv := ax7WebviewBase()
	cw := NewConsoleWatcher(wv)
	cw.addMessage(ConsoleMessage{Type: "log", Text: "manual"})
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_AddFilter_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"})
	cw.AddFilter(ConsoleFilter{Type: "error"})
	if len(cw.FilteredMessages()) != 1 {
		t.Fatalf("FilteredMessages = %#v", cw.FilteredMessages())
	}
}

func TestAX7_ConsoleWatcher_AddFilter_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "log", Text: "hello"})
	cw.AddFilter(ConsoleFilter{Type: "error"})
	if len(cw.FilteredMessages()) != 0 {
		t.Fatalf("FilteredMessages = %#v", cw.FilteredMessages())
	}
}

func TestAX7_ConsoleWatcher_AddFilter_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "log", Text: "hello"})
	cw.AddFilter(ConsoleFilter{Pattern: ""})
	if len(cw.FilteredMessages()) != 1 {
		t.Fatalf("FilteredMessages = %#v", cw.FilteredMessages())
	}
}

func TestAX7_ConsoleWatcher_ClearFilters_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "log", Text: "hello"})
	cw.AddFilter(ConsoleFilter{Type: "error"})
	cw.ClearFilters()
	if len(cw.FilteredMessages()) != 1 {
		t.Fatalf("FilteredMessages = %#v", cw.FilteredMessages())
	}
}

func TestAX7_ConsoleWatcher_ClearFilters_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	cw.ClearFilters()
	if len(cw.filters) != 0 {
		t.Fatalf("filters = %d", len(cw.filters))
	}
}

func TestAX7_ConsoleWatcher_ClearFilters_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"})
	cw.AddFilter(ConsoleFilter{Type: "error"})
	cw.AddFilter(ConsoleFilter{Pattern: "boom"})
	cw.ClearFilters()
	if len(cw.FilteredMessages()) != 1 {
		t.Fatalf("FilteredMessages = %#v", cw.FilteredMessages())
	}
}

func TestAX7_ConsoleWatcher_AddHandler_Good(t *testing.T) {
	cw := ax7ConsoleWatcher()
	called := 0
	cw.AddHandler(func(ConsoleMessage) { called++ })
	cw.addMessage(ConsoleMessage{Type: "log"})
	if called != 1 {
		t.Fatalf("called = %d", called)
	}
}

func TestAX7_ConsoleWatcher_AddHandler_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	called := 0
	cw.AddHandler(func(ConsoleMessage) { called++ })
	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestAX7_ConsoleWatcher_AddHandler_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher()
	called := 0
	cw.AddHandler(func(ConsoleMessage) { called++ })
	cw.AddHandler(func(ConsoleMessage) { called++ })
	cw.addMessage(ConsoleMessage{Type: "log"})
	if called != 2 {
		t.Fatalf("called = %d", called)
	}
}

func TestAX7_ConsoleWatcher_Clear_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Text: "one"})
	cw.Clear()
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Clear_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	cw.Clear()
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_Clear_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Text: "one"})
	cw.Clear()
	cw.addMessage(ConsoleMessage{Text: "two"})
	if cw.Count() != 1 {
		t.Fatalf("Count = %d", cw.Count())
	}
	if cw.Messages()[0].Text != "two" {
		t.Fatalf("messages = %#v", cw.Messages())
	}
}

func TestAX7_ConsoleWatcher_SetLimit_Good(t *testing.T) {
	cw := ax7ConsoleWatcher()
	cw.SetLimit(1)
	cw.addMessage(ConsoleMessage{Text: "one"})
	cw.addMessage(ConsoleMessage{Text: "two"})
	if cw.Count() != 1 || cw.Messages()[0].Text != "two" {
		t.Fatalf("messages = %#v", cw.Messages())
	}
}

func TestAX7_ConsoleWatcher_SetLimit_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	cw.SetLimit(-1)
	cw.addMessage(ConsoleMessage{Text: "drop"})
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_SetLimit_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Text: "kept"})
	cw.SetLimit(0)
	cw.addMessage(ConsoleMessage{Text: "drop"})
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_HasErrors_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"})
	if !cw.HasErrors() {
		t.Fatal("HasErrors returned false")
	}
	if cw.ErrorCount() != 1 {
		t.Fatalf("ErrorCount = %d", cw.ErrorCount())
	}
}

func TestAX7_ConsoleWatcher_HasErrors_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "log", Text: "ok"})
	if cw.HasErrors() {
		t.Fatal("HasErrors returned true")
	}
	if cw.ErrorCount() != 0 {
		t.Fatalf("ErrorCount = %d", cw.ErrorCount())
	}
}

func TestAX7_ConsoleWatcher_HasErrors_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "ERROR", Text: "boom"})
	if !cw.HasErrors() {
		t.Fatal("HasErrors did not canonicalize type")
	}
	if cw.ErrorCount() != 1 {
		t.Fatalf("ErrorCount = %d", cw.ErrorCount())
	}
}

func TestAX7_ConsoleWatcher_Count_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{}, ConsoleMessage{})
	if cw.Count() != 2 {
		t.Fatalf("Count = %d", cw.Count())
	}
	if len(cw.Messages()) != 2 {
		t.Fatalf("Messages = %#v", cw.Messages())
	}
}

func TestAX7_ConsoleWatcher_Count_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
	if len(cw.Messages()) != 0 {
		t.Fatalf("Messages = %#v", cw.Messages())
	}
}

func TestAX7_ConsoleWatcher_Count_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Text: "one"})
	cw.Clear()
	if cw.Count() != 0 {
		t.Fatalf("Count = %d", cw.Count())
	}
}

func TestAX7_ConsoleWatcher_ErrorCount_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error"}, ConsoleMessage{Type: "log"})
	if cw.ErrorCount() != 1 {
		t.Fatalf("ErrorCount = %d", cw.ErrorCount())
	}
	if !cw.HasErrors() {
		t.Fatal("HasErrors returned false")
	}
}

func TestAX7_ConsoleWatcher_ErrorCount_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "warning"})
	if cw.ErrorCount() != 0 {
		t.Fatalf("ErrorCount = %d", cw.ErrorCount())
	}
	if cw.HasErrors() {
		t.Fatal("HasErrors returned true")
	}
}

func TestAX7_ConsoleWatcher_ErrorCount_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "ERROR"})
	if cw.ErrorCount() != 1 {
		t.Fatalf("ErrorCount = %d", cw.ErrorCount())
	}
	if len(cw.Errors()) != 1 {
		t.Fatalf("Errors = %#v", cw.Errors())
	}
}

func TestAX7_ConsoleWatcher_WaitForMessage_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "log", Text: "ready"})
	msg, err := cw.WaitForMessage(context.Background(), ConsoleFilter{Pattern: "ready"})
	if err != nil || msg.Text != "ready" {
		t.Fatalf("WaitForMessage = %#v, %v", msg, err)
	}
}

func TestAX7_ConsoleWatcher_WaitForMessage_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := cw.WaitForMessage(ctx, ConsoleFilter{Type: "error"})
	if err == nil {
		t.Fatal("WaitForMessage succeeded without a message")
	}
}

func TestAX7_ConsoleWatcher_WaitForMessage_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go cw.addMessage(ConsoleMessage{Type: "warning", Text: "late"})
	msg, err := cw.WaitForMessage(ctx, ConsoleFilter{Type: "warn"})
	if err != nil || msg.Text != "late" {
		t.Fatalf("WaitForMessage = %#v, %v", msg, err)
	}
}

func TestAX7_ConsoleWatcher_WaitForError_Good(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "error", Text: "boom"})
	msg, err := cw.WaitForError(context.Background())
	if err != nil || msg.Text != "boom" {
		t.Fatalf("WaitForError = %#v, %v", msg, err)
	}
}

func TestAX7_ConsoleWatcher_WaitForError_Bad(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "log", Text: "ok"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := cw.WaitForError(ctx)
	if err == nil {
		t.Fatal("WaitForError succeeded without error")
	}
}

func TestAX7_ConsoleWatcher_WaitForError_Ugly(t *testing.T) {
	cw := ax7ConsoleWatcher(ConsoleMessage{Type: "ERROR", Text: "caps"})
	msg, err := cw.WaitForError(context.Background())
	if err != nil || msg.Text != "caps" {
		t.Fatalf("WaitForError = %#v, %v", msg, err)
	}
}

func TestAX7_FormatConsoleOutput_Good(t *testing.T) {
	out := FormatConsoleOutput([]ConsoleMessage{{Type: "error", Text: "boom", Timestamp: time.Date(2026, 1, 1, 1, 2, 3, 4_000_000, time.UTC)}})
	if !strings.Contains(out, "[ERROR]") {
		t.Fatalf("output = %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Fatalf("output = %q", out)
	}
}

func TestAX7_FormatConsoleOutput_Bad(t *testing.T) {
	out := FormatConsoleOutput(nil)
	if out != "" {
		t.Fatalf("empty output = %q", out)
	}
	if len(out) != 0 {
		t.Fatalf("len = %d", len(out))
	}
}

func TestAX7_FormatConsoleOutput_Ugly(t *testing.T) {
	out := FormatConsoleOutput([]ConsoleMessage{{Type: "log", Text: "a\nb\x1b", Timestamp: time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC)}})
	if strings.Contains(out, "a\nb") {
		t.Fatalf("output kept newline: %q", out)
	}
	if !strings.Contains(out, `a\nb\x1b`) {
		t.Fatalf("output = %q", out)
	}
}

func TestAX7_ExceptionWatcher_Exceptions_Good(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "boom"})
	var got []ExceptionInfo
	got = ew.Exceptions()
	if len(got) != 1 || got[0].Text != "boom" {
		t.Fatalf("Exceptions = %#v", got)
	}
	if ew.Count() != 1 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_Exceptions_Bad(t *testing.T) {
	ew := ax7ExceptionWatcher()
	var got []ExceptionInfo
	got = ew.Exceptions()
	if len(got) != 0 {
		t.Fatalf("empty Exceptions = %#v", got)
	}
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_Exceptions_Ugly(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "one"}, ExceptionInfo{Text: "two"})
	var got []ExceptionInfo
	got = ew.Exceptions()
	if len(got) != 2 {
		t.Fatalf("ugly Exceptions = %#v", got)
	}
	if ew.HasExceptions() != true {
		t.Fatal("HasExceptions returned false")
	}
}

func TestAX7_ExceptionWatcher_ExceptionsAll_Good(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "boom"})
	var got []ExceptionInfo
	for exc := range ew.ExceptionsAll() {
		got = append(got, exc)
	}
	if len(got) != 1 || got[0].Text != "boom" {
		t.Fatalf("ExceptionsAll = %#v", got)
	}
	if ew.Count() != 1 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_ExceptionsAll_Bad(t *testing.T) {
	ew := ax7ExceptionWatcher()
	var got []ExceptionInfo
	for exc := range ew.ExceptionsAll() {
		got = append(got, exc)
	}
	if len(got) != 0 {
		t.Fatalf("empty ExceptionsAll = %#v", got)
	}
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_ExceptionsAll_Ugly(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "one"}, ExceptionInfo{Text: "two"})
	var got []ExceptionInfo
	for exc := range ew.ExceptionsAll() {
		got = append(got, exc)
		break
	}
	if len(got) != 1 {
		t.Fatalf("ugly ExceptionsAll = %#v", got)
	}
	if ew.HasExceptions() != true {
		t.Fatal("HasExceptions returned false")
	}
}

func TestAX7_NewExceptionWatcher_Good(t *testing.T) {
	ew := NewExceptionWatcher(nil)
	if ew == nil {
		t.Fatal("NewExceptionWatcher returned nil")
	}
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_NewExceptionWatcher_Bad(t *testing.T) {
	ew := NewExceptionWatcher(&Webview{})
	if ew == nil {
		t.Fatal("NewExceptionWatcher returned nil for empty webview")
	}
	if len(ew.handlers) != 0 {
		t.Fatalf("handlers = %d", len(ew.handlers))
	}
}

func TestAX7_NewExceptionWatcher_Ugly(t *testing.T) {
	ew := NewExceptionWatcher(nil)
	ew.handleException(map[string]any{"exceptionDetails": map[string]any{"text": "boom"}})
	if ew.Count() != 1 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_Clear_Good(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "boom"})
	ew.Clear()
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_Clear_Bad(t *testing.T) {
	ew := ax7ExceptionWatcher()
	ew.Clear()
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_Clear_Ugly(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "boom"})
	ew.Clear()
	ew.handleException(map[string]any{"exceptionDetails": map[string]any{"text": "next"}})
	if ew.Count() != 1 {
		t.Fatalf("Count = %d", ew.Count())
	}
	if ew.Exceptions()[0].Text != "next" {
		t.Fatalf("Exceptions = %#v", ew.Exceptions())
	}
}

func TestAX7_ExceptionWatcher_HasExceptions_Good(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "boom"})
	if !ew.HasExceptions() {
		t.Fatal("HasExceptions returned false")
	}
	if ew.Count() != 1 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_HasExceptions_Bad(t *testing.T) {
	ew := ax7ExceptionWatcher()
	if ew.HasExceptions() {
		t.Fatal("HasExceptions returned true")
	}
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_HasExceptions_Ugly(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{})
	if !ew.HasExceptions() {
		t.Fatal("HasExceptions ignored zero exception")
	}
	if ew.Count() != 1 {
		t.Fatalf("Count = %d", ew.Count())
	}
}

func TestAX7_ExceptionWatcher_Count_Good(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{}, ExceptionInfo{})
	if ew.Count() != 2 {
		t.Fatalf("Count = %d", ew.Count())
	}
	if len(ew.Exceptions()) != 2 {
		t.Fatalf("Exceptions = %#v", ew.Exceptions())
	}
}

func TestAX7_ExceptionWatcher_Count_Bad(t *testing.T) {
	ew := ax7ExceptionWatcher()
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
	if len(ew.Exceptions()) != 0 {
		t.Fatalf("Exceptions = %#v", ew.Exceptions())
	}
}

func TestAX7_ExceptionWatcher_Count_Ugly(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "one"})
	ew.Clear()
	if ew.Count() != 0 {
		t.Fatalf("Count = %d", ew.Count())
	}
	if ew.HasExceptions() {
		t.Fatal("HasExceptions returned true after Clear")
	}
}

func TestAX7_ExceptionWatcher_AddHandler_Good(t *testing.T) {
	ew := ax7ExceptionWatcher()
	called := 0
	ew.AddHandler(func(ExceptionInfo) { called++ })
	ew.handleException(map[string]any{"exceptionDetails": map[string]any{"text": "boom"}})
	if called != 1 {
		t.Fatalf("called = %d", called)
	}
}

func TestAX7_ExceptionWatcher_AddHandler_Bad(t *testing.T) {
	ew := ax7ExceptionWatcher()
	called := 0
	ew.AddHandler(func(ExceptionInfo) { called++ })
	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestAX7_ExceptionWatcher_AddHandler_Ugly(t *testing.T) {
	ew := ax7ExceptionWatcher()
	called := 0
	ew.AddHandler(func(ExceptionInfo) { called++ })
	ew.AddHandler(func(ExceptionInfo) { called++ })
	ew.handleException(map[string]any{"exceptionDetails": map[string]any{"text": "boom"}})
	if called != 2 {
		t.Fatalf("called = %d", called)
	}
}

func TestAX7_ExceptionWatcher_WaitForException_Good(t *testing.T) {
	ew := ax7ExceptionWatcher(ExceptionInfo{Text: "boom"})
	exc, err := ew.WaitForException(context.Background())
	if err != nil || exc.Text != "boom" {
		t.Fatalf("WaitForException = %#v, %v", exc, err)
	}
}

func TestAX7_ExceptionWatcher_WaitForException_Bad(t *testing.T) {
	ew := ax7ExceptionWatcher()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := ew.WaitForException(ctx)
	if err == nil {
		t.Fatal("WaitForException succeeded without exception")
	}
}

func TestAX7_ExceptionWatcher_WaitForException_Ugly(t *testing.T) {
	ew := ax7ExceptionWatcher()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go ew.handleException(map[string]any{"exceptionDetails": map[string]any{"text": "late"}})
	exc, err := ew.WaitForException(ctx)
	if err != nil || exc.Text != "late" {
		t.Fatalf("WaitForException = %#v, %v", exc, err)
	}
}

func TestAX7_Webview_Close_Good(t *testing.T) {
	wv := ax7WebviewBase()
	if err := wv.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if wv.ctx.Err() != nil {
		t.Fatalf("base context changed: %v", wv.ctx.Err())
	}
}

func TestAX7_Webview_Close_Bad(t *testing.T) {
	var wv *Webview
	defer func() {
		if recover() == nil {
			t.Fatal("Close on nil webview did not panic")
		}
	}()
	wv.Close()
}

func TestAX7_Webview_Close_Ugly(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())
	wv := ax7WebviewBase()
	wv.client = client
	if err := wv.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	if err := wv.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestAX7_Webview_Navigate_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.Navigate("https://example.com"); err != nil {
		t.Fatalf("Navigate returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Navigate_Bad(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.Navigate("javascript:alert(1)"); err == nil {
		t.Fatal("Navigate accepted dangerous URL")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Navigate_Ugly(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.Navigate("about:blank"); err != nil {
		t.Fatalf("Navigate rejected about:blank: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Click_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	if err := wv.Click("#button"); err != nil {
		t.Fatalf("Click returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Click_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	if err := wv.Click("#missing"); err == nil {
		t.Fatal("Click succeeded for missing element")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Click_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	if err := wv.Click("#button"); err != nil {
		t.Fatalf("Click fallback returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Type_Good(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Runtime.evaluate", "Input.dispatchKeyEvent":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": true}})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	if err := wv.Type("#input", "ab"); err != nil {
		t.Fatalf("Type returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Type_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "focus", true)
	if err := wv.Type("#input", "ab"); err == nil {
		t.Fatal("Type succeeded despite focus failure")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Type_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "focus", false)
	if err := wv.Type("#input", ""); err != nil {
		t.Fatalf("Type rejected empty text: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_QuerySelector_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	elem, err := wv.QuerySelector("#main")
	if err != nil {
		t.Fatalf("QuerySelector returned error: %v", err)
	}
	if elem.NodeID != 10 {
		t.Fatalf("element = %#v", elem)
	}
}

func TestAX7_Webview_QuerySelector_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	elem, err := wv.QuerySelector("#missing")
	if err == nil {
		t.Fatal("QuerySelector succeeded for missing element")
	}
	if elem != nil {
		t.Fatalf("element = %#v", elem)
	}
}

func TestAX7_Webview_QuerySelector_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	elem, err := wv.QuerySelector("")
	if err != nil {
		t.Fatalf("QuerySelector rejected empty selector fixture: %v", err)
	}
	if elem.BoundingBox != nil {
		t.Fatalf("bounding box = %#v", elem.BoundingBox)
	}
}

func TestAX7_Webview_QuerySelectorAll_Good(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelectorAll":
			target.reply(msg.ID, map[string]any{"nodeIds": []any{float64(10), float64(11)}})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	elems, err := wv.QuerySelectorAll("div")
	if err != nil {
		t.Fatalf("QuerySelectorAll returned error: %v", err)
	}
	if len(elems) != 2 {
		t.Fatalf("elems = %#v", elems)
	}
}

func TestAX7_Webview_QuerySelectorAll_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelectorAll":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	elems, err := wv.QuerySelectorAll("div")
	if err == nil {
		t.Fatal("QuerySelectorAll accepted malformed node list")
	}
	if elems != nil {
		t.Fatalf("elems = %#v", elems)
	}
}

func TestAX7_Webview_QuerySelectorAll_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelectorAll":
			target.reply(msg.ID, map[string]any{"nodeIds": []any{}})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	elems, err := wv.QuerySelectorAll(".none")
	if err != nil {
		t.Fatalf("QuerySelectorAll returned error for empty result: %v", err)
	}
	if len(elems) != 0 {
		t.Fatalf("elems = %#v", elems)
	}
}

func TestAX7_Webview_QuerySelectorAllAll_Good(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelectorAll":
			target.reply(msg.ID, map[string]any{"nodeIds": []any{float64(10)}})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	seen := 0
	for elem := range wv.QuerySelectorAllAll("div") {
		if elem != nil {
			seen++
		}
		break
	}
	if seen != 1 {
		t.Fatalf("seen = %d", seen)
	}
}

func TestAX7_Webview_QuerySelectorAllAll_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelectorAll":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	seen := 0
	for range wv.QuerySelectorAllAll("div") {
		seen++
	}
	if seen != 0 {
		t.Fatalf("seen = %d", seen)
	}
}

func TestAX7_Webview_QuerySelectorAllAll_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelectorAll":
			target.reply(msg.ID, map[string]any{"nodeIds": []any{}})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	seen := 0
	for range wv.QuerySelectorAllAll(".none") {
		seen++
	}
	if seen != 0 {
		t.Fatalf("seen = %d", seen)
	}
}

func TestAX7_Webview_GetConsole_Good(t *testing.T) {
	wv := ax7WebviewBase()
	wv.addConsoleMessage(ConsoleMessage{Text: "one"})
	got := wv.GetConsole()
	if len(got) != 1 || got[0].Text != "one" {
		t.Fatalf("GetConsole = %#v", got)
	}
}

func TestAX7_Webview_GetConsole_Bad(t *testing.T) {
	wv := ax7WebviewBase()
	got := wv.GetConsole()
	if len(got) != 0 {
		t.Fatalf("GetConsole = %#v", got)
	}
}

func TestAX7_Webview_GetConsole_Ugly(t *testing.T) {
	wv := ax7WebviewBase()
	wv.consoleLimit = 1
	wv.addConsoleMessage(ConsoleMessage{Text: "one"})
	wv.addConsoleMessage(ConsoleMessage{Text: "two"})
	got := wv.GetConsole()
	if len(got) != 1 || got[0].Text != "two" {
		t.Fatalf("GetConsole = %#v", got)
	}
}

func TestAX7_Webview_GetConsoleAll_Good(t *testing.T) {
	wv := ax7WebviewBase()
	wv.addConsoleMessage(ConsoleMessage{Text: "one"})
	seen := 0
	for msg := range wv.GetConsoleAll() {
		if msg.Text == "one" {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("seen = %d", seen)
	}
}

func TestAX7_Webview_GetConsoleAll_Bad(t *testing.T) {
	wv := ax7WebviewBase()
	seen := 0
	for range wv.GetConsoleAll() {
		seen++
	}
	if seen != 0 {
		t.Fatalf("seen = %d", seen)
	}
}

func TestAX7_Webview_GetConsoleAll_Ugly(t *testing.T) {
	wv := ax7WebviewBase()
	wv.addConsoleMessage(ConsoleMessage{Text: "one"})
	wv.addConsoleMessage(ConsoleMessage{Text: "two"})
	seen := 0
	for range wv.GetConsoleAll() {
		seen++
		break
	}
	if seen != 1 {
		t.Fatalf("seen = %d", seen)
	}
}

func TestAX7_Webview_ClearConsole_Good(t *testing.T) {
	wv := ax7WebviewBase()
	wv.addConsoleMessage(ConsoleMessage{Text: "one"})
	wv.ClearConsole()
	if len(wv.GetConsole()) != 0 {
		t.Fatalf("console = %#v", wv.GetConsole())
	}
}

func TestAX7_Webview_ClearConsole_Bad(t *testing.T) {
	wv := ax7WebviewBase()
	wv.ClearConsole()
	if len(wv.GetConsole()) != 0 {
		t.Fatalf("console = %#v", wv.GetConsole())
	}
}

func TestAX7_Webview_ClearConsole_Ugly(t *testing.T) {
	wv := ax7WebviewBase()
	wv.addConsoleMessage(ConsoleMessage{Text: "one"})
	wv.ClearConsole()
	wv.addConsoleMessage(ConsoleMessage{Text: "two"})
	if got := wv.GetConsole(); len(got) != 1 || got[0].Text != "two" {
		t.Fatalf("console = %#v", got)
	}
}

func TestAX7_Webview_Screenshot_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	png, err := wv.Screenshot()
	if err != nil {
		t.Fatalf("Screenshot returned error: %v", err)
	}
	if string(png) != "png" {
		t.Fatalf("png = %v", png)
	}
}

func TestAX7_Webview_Screenshot_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.reply(msg.ID, map[string]any{"data": "%%%"}) })
	png, err := wv.Screenshot()
	if err == nil {
		t.Fatal("Screenshot accepted invalid base64")
	}
	if png != nil {
		t.Fatalf("png = %v", png)
	}
}

func TestAX7_Webview_Screenshot_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		target.reply(msg.ID, map[string]any{"data": base64.StdEncoding.EncodeToString(nil)})
	})
	png, err := wv.Screenshot()
	if err != nil {
		t.Fatalf("Screenshot rejected empty PNG data: %v", err)
	}
	if len(png) != 0 {
		t.Fatalf("png = %v", png)
	}
}

func TestAX7_Webview_Evaluate_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	got, err := wv.Evaluate("document.title")
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if got != "Example" && got != "<section></section>" {
		t.Fatalf("Evaluate = %#v", got)
	}
}

func TestAX7_Webview_Evaluate_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		target.reply(msg.ID, map[string]any{"exceptionDetails": map[string]any{"text": "boom"}})
	})
	got, err := wv.Evaluate("throw new Error('boom')")
	if err == nil {
		t.Fatal("Evaluate accepted exception")
	}
	if got != nil {
		t.Fatalf("got = %#v", got)
	}
}

func TestAX7_Webview_Evaluate_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		target.reply(msg.ID, map[string]any{"result": map[string]any{"value": nil}})
	})
	got, err := wv.Evaluate("null")
	if err != nil {
		t.Fatalf("Evaluate rejected null: %v", err)
	}
	if got != nil {
		t.Fatalf("got = %#v", got)
	}
}

func TestAX7_Webview_WaitForSelector_Good(t *testing.T) {
	wv := ax7EvaluateWebview(t, "querySelector", false)
	if err := wv.WaitForSelector("#ready"); err != nil {
		t.Fatalf("WaitForSelector returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_WaitForSelector_Bad(t *testing.T) {
	wv := ax7EvaluateWebview(t, "querySelector", true)
	wv.timeout = time.Millisecond
	if err := wv.WaitForSelector("#never"); err == nil {
		t.Fatal("WaitForSelector succeeded without selector")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_WaitForSelector_Ugly(t *testing.T) {
	wv := ax7EvaluateWebview(t, "querySelector", false)
	if err := wv.WaitForSelector(""); err != nil {
		t.Fatalf("WaitForSelector rejected empty selector fixture: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_GetURL_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	got, err := wv.GetURL()
	if err != nil || got != "https://example.com" {
		t.Fatalf("GetURL = %q, %v", got, err)
	}
}

func TestAX7_Webview_GetURL_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyValue(msg.ID, 1) })
	got, err := wv.GetURL()
	if err == nil {
		t.Fatal("GetURL accepted non-string")
	}
	if got != "" {
		t.Fatalf("url = %q", got)
	}
}

func TestAX7_Webview_GetURL_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyValue(msg.ID, "about:blank") })
	got, err := wv.GetURL()
	if err != nil || got != "about:blank" {
		t.Fatalf("GetURL = %q, %v", got, err)
	}
}

func TestAX7_Webview_GetTitle_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	got, err := wv.GetTitle()
	if err != nil || got != "Example" {
		t.Fatalf("GetTitle = %q, %v", got, err)
	}
}

func TestAX7_Webview_GetTitle_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyValue(msg.ID, 1) })
	got, err := wv.GetTitle()
	if err == nil {
		t.Fatal("GetTitle accepted non-string")
	}
	if got != "" {
		t.Fatalf("title = %q", got)
	}
}

func TestAX7_Webview_GetTitle_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyValue(msg.ID, "") })
	got, err := wv.GetTitle()
	if err != nil || got != "" {
		t.Fatalf("GetTitle = %q, %v", got, err)
	}
}

func TestAX7_Webview_GetHTML_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	got, err := wv.GetHTML("")
	if err != nil || got != "<html></html>" {
		t.Fatalf("GetHTML = %q, %v", got, err)
	}
}

func TestAX7_Webview_GetHTML_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyValue(msg.ID, 1) })
	got, err := wv.GetHTML("#main")
	if err == nil {
		t.Fatal("GetHTML accepted non-string")
	}
	if got != "" {
		t.Fatalf("html = %q", got)
	}
}

func TestAX7_Webview_GetHTML_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyValue(msg.ID, "") })
	got, err := wv.GetHTML("#missing")
	if err != nil || got != "" {
		t.Fatalf("GetHTML = %q, %v", got, err)
	}
}

func TestAX7_Webview_SetViewport_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.SetViewport(1280, 720); err != nil {
		t.Fatalf("SetViewport returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_SetViewport_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyError(msg.ID, "bad viewport") })
	if err := wv.SetViewport(-1, -1); err == nil {
		t.Fatal("SetViewport ignored CDP failure")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_SetViewport_Ugly(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.SetViewport(0, 0); err != nil {
		t.Fatalf("SetViewport rejected zero size fixture: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_SetUserAgent_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.SetUserAgent("Agent/1.0"); err != nil {
		t.Fatalf("SetUserAgent returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_SetUserAgent_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyError(msg.ID, "bad ua") })
	if err := wv.SetUserAgent("Agent/1.0"); err == nil {
		t.Fatal("SetUserAgent ignored CDP failure")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_SetUserAgent_Ugly(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.SetUserAgent(""); err != nil {
		t.Fatalf("SetUserAgent rejected empty user agent fixture: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Reload_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.Reload(); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Reload_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) { target.replyError(msg.ID, "reload failed") })
	if err := wv.Reload(); err == nil {
		t.Fatal("Reload ignored CDP failure")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_Reload_Ugly(t *testing.T) {
	wv := ax7NavigationWebview(t)
	wv.timeout = 250 * time.Millisecond
	if err := wv.Reload(); err != nil {
		t.Fatalf("Reload with short timeout returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_GoBack_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.GoBack(); err != nil {
		t.Fatalf("GoBack returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_GoBack_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		target.reply(msg.ID, map[string]any{"currentIndex": float64(0), "entries": []any{map[string]any{"id": float64(1)}}})
	})
	if err := wv.GoBack(); err == nil {
		t.Fatal("GoBack succeeded without history entry")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_GoBack_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Page.getNavigationHistory":
			target.reply(msg.ID, map[string]any{"currentIndex": float64(1), "entries": []any{map[string]any{"id": float64(1)}, map[string]any{"id": float64(2)}}})
		case "Page.navigateToHistoryEntry":
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, "complete")
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	if err := wv.GoBack(); err != nil {
		t.Fatalf("GoBack returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_GoForward_Good(t *testing.T) {
	wv := ax7NavigationWebview(t)
	if err := wv.GoForward(); err != nil {
		t.Fatalf("GoForward returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_GoForward_Bad(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		target.reply(msg.ID, map[string]any{"currentIndex": float64(0), "entries": []any{map[string]any{"id": float64(1)}}})
	})
	if err := wv.GoForward(); err == nil {
		t.Fatal("GoForward succeeded without history entry")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_GoForward_Ugly(t *testing.T) {
	wv, _ := newWebviewHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Page.getNavigationHistory":
			target.reply(msg.ID, map[string]any{"currentIndex": float64(0), "entries": []any{map[string]any{"id": float64(1)}, map[string]any{"id": float64(2)}}})
		case "Page.navigateToHistoryEntry":
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, "complete")
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})
	if err := wv.GoForward(); err != nil {
		t.Fatalf("GoForward returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_UploadFile_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	if err := wv.UploadFile("#file", []string{"/tmp/a.txt"}); err != nil {
		t.Fatalf("UploadFile returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_UploadFile_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	if err := wv.UploadFile("#missing", []string{"/tmp/a.txt"}); err == nil {
		t.Fatal("UploadFile succeeded for missing input")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_UploadFile_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	if err := wv.UploadFile("#file", nil); err != nil {
		t.Fatalf("UploadFile rejected nil list: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_DragAndDrop_Good(t *testing.T) {
	wv := ax7DOMWebview(t, true)
	if err := wv.DragAndDrop("#a", "#b"); err != nil {
		t.Fatalf("DragAndDrop returned error: %v", err)
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_DragAndDrop_Bad(t *testing.T) {
	wv := ax7MissingDOMWebview(t)
	if err := wv.DragAndDrop("#a", "#b"); err == nil {
		t.Fatal("DragAndDrop succeeded for missing source")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func TestAX7_Webview_DragAndDrop_Ugly(t *testing.T) {
	wv := ax7DOMWebview(t, false)
	if err := wv.DragAndDrop("#a", "#b"); err == nil {
		t.Fatal("DragAndDrop succeeded without bounding box")
	}
	if wv.client == nil {
		t.Fatal("fixture webview lost client")
	}
}

func nilCall(err error) (any, error) { return nil, err }

func TestAX7_NewAngularHelper_Good(t *testing.T) {
	wv := ax7WebviewBase()
	ah := NewAngularHelper(wv)
	if ah == nil || ah.wv != wv {
		t.Fatalf("NewAngularHelper = %#v", ah)
	}
	if ah.timeout != 30*time.Second {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_NewAngularHelper_Bad(t *testing.T) {
	ah := NewAngularHelper(nil)
	if ah == nil {
		t.Fatal("NewAngularHelper returned nil")
	}
	if ah.wv != nil {
		t.Fatalf("wv = %#v", ah.wv)
	}
}

func TestAX7_NewAngularHelper_Ugly(t *testing.T) {
	wv := &Webview{timeout: time.Nanosecond}
	ah := NewAngularHelper(wv)
	if ah.wv != wv {
		t.Fatalf("wv = %#v", ah.wv)
	}
	if ah.timeout != 30*time.Second {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_SetTimeout_Good(t *testing.T) {
	ah := NewAngularHelper(ax7WebviewBase())
	ah.SetTimeout(5 * time.Second)
	if ah.timeout != 5*time.Second {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_SetTimeout_Bad(t *testing.T) {
	ah := NewAngularHelper(ax7WebviewBase())
	ah.SetTimeout(-time.Second)
	if ah.timeout != -time.Second {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_SetTimeout_Ugly(t *testing.T) {
	ah := NewAngularHelper(ax7WebviewBase())
	ah.SetTimeout(0)
	if ah.timeout != 0 {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_WaitForAngular_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	err := ah.WaitForAngular()
	if err != nil {
		t.Fatalf("WaitForAngular returned error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_WaitForAngular_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	err := ah.WaitForAngular()
	if err == nil {
		t.Fatal("WaitForAngular succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_WaitForAngular_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	err := ah.WaitForAngular()
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_NavigateByRouter_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	err := ah.NavigateByRouter("/dashboard")
	if err != nil {
		t.Fatalf("NavigateByRouter returned error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_NavigateByRouter_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	err := ah.NavigateByRouter("/dashboard")
	if err == nil {
		t.Fatal("NavigateByRouter succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_NavigateByRouter_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	err := ah.NavigateByRouter("")
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_GetComponentProperty_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, "Ada")
	})
	got, err := ah.GetComponentProperty("app-user", "name")
	if err != nil {
		t.Fatalf("GetComponentProperty returned error: %v", err)
	}
	if got == nil {
		t.Fatal("GetComponentProperty returned nil value")
	}
}

func TestAX7_AngularHelper_GetComponentProperty_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := ah.GetComponentProperty("app-user", "name")
	if err == nil {
		t.Fatal("GetComponentProperty succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_GetComponentProperty_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := ah.GetComponentProperty("app-user", "name")
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_SetComponentProperty_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	_, err := nilCall(ah.SetComponentProperty("app-user", "active", true))
	if err != nil {
		t.Fatalf("SetComponentProperty returned error: %v", err)
	}
	if ah.timeout <= 0 {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_SetComponentProperty_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := nilCall(ah.SetComponentProperty("app-user", "active", true))
	if err == nil {
		t.Fatal("SetComponentProperty succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_SetComponentProperty_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := nilCall(ah.SetComponentProperty("app-user", "active", true))
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_CallComponentMethod_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, map[string]any{"ok": true})
	})
	got, err := ah.CallComponentMethod("app-user", "save", 1)
	if err != nil {
		t.Fatalf("CallComponentMethod returned error: %v", err)
	}
	if got == nil {
		t.Fatal("CallComponentMethod returned nil value")
	}
}

func TestAX7_AngularHelper_CallComponentMethod_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := ah.CallComponentMethod("app-user", "save", 1)
	if err == nil {
		t.Fatal("CallComponentMethod succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_CallComponentMethod_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := ah.CallComponentMethod("app-user", "save", 1)
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_TriggerChangeDetection_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	_, err := nilCall(ah.TriggerChangeDetection())
	if err != nil {
		t.Fatalf("TriggerChangeDetection returned error: %v", err)
	}
	if ah.timeout <= 0 {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_TriggerChangeDetection_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := nilCall(ah.TriggerChangeDetection())
	if err == nil {
		t.Fatal("TriggerChangeDetection succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_TriggerChangeDetection_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := nilCall(ah.TriggerChangeDetection())
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_GetService_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, map[string]any{"name": "session"})
	})
	got, err := ah.GetService("SessionService")
	if err != nil {
		t.Fatalf("GetService returned error: %v", err)
	}
	if got == nil {
		t.Fatal("GetService returned nil value")
	}
}

func TestAX7_AngularHelper_GetService_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := ah.GetService("SessionService")
	if err == nil {
		t.Fatal("GetService succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_GetService_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := ah.GetService("SessionService")
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_GetRouterState_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, map[string]any{"url": "/dashboard", "params": map[string]any{"id": "42"}, "queryParams": map[string]any{"tab": "users"}})
	})
	state, err := ah.GetRouterState()
	if err != nil {
		t.Fatalf("GetRouterState returned error: %v", err)
	}
	if state.URL != "/dashboard" || state.Params["id"] != "42" {
		t.Fatalf("state = %#v", state)
	}
}

func TestAX7_AngularHelper_GetRouterState_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "router failed")
	})
	state, err := ah.GetRouterState()
	if err == nil {
		t.Fatal("GetRouterState succeeded despite evaluation failure")
	}
	if state != nil {
		t.Fatalf("state = %#v", state)
	}
}

func TestAX7_AngularHelper_GetRouterState_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, map[string]any{"url": "/empty", "params": map[string]any{"id": 42}, "queryParams": nil})
	})
	state, err := ah.GetRouterState()
	if err != nil {
		t.Fatalf("GetRouterState returned error: %v", err)
	}
	if state.URL != "/empty" || len(state.Params) != 0 {
		t.Fatalf("state = %#v", state)
	}
}

func TestAX7_AngularHelper_WaitForComponent_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	_, err := nilCall(ah.WaitForComponent("app-user"))
	if err != nil {
		t.Fatalf("WaitForComponent returned error: %v", err)
	}
	if ah.timeout <= 0 {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_WaitForComponent_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	ah.SetTimeout(20 * time.Millisecond)
	_, err := nilCall(ah.WaitForComponent("app-user"))
	if err == nil {
		t.Fatal("WaitForComponent succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_WaitForComponent_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	_, err := nilCall(ah.WaitForComponent(""))
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_DispatchEvent_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	_, err := nilCall(ah.DispatchEvent("app-user", "change", map[string]any{"x": 1}))
	if err != nil {
		t.Fatalf("DispatchEvent returned error: %v", err)
	}
	if ah.timeout <= 0 {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_DispatchEvent_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := nilCall(ah.DispatchEvent("app-user", "change", map[string]any{"x": 1}))
	if err == nil {
		t.Fatal("DispatchEvent succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_DispatchEvent_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := nilCall(ah.DispatchEvent("app-user", "change", map[string]any{"x": 1}))
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_GetNgModel_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, "value")
	})
	got, err := ah.GetNgModel("input")
	if err != nil {
		t.Fatalf("GetNgModel returned error: %v", err)
	}
	if got == nil {
		t.Fatal("GetNgModel returned nil value")
	}
}

func TestAX7_AngularHelper_GetNgModel_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := ah.GetNgModel("input")
	if err == nil {
		t.Fatal("GetNgModel succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_GetNgModel_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := ah.GetNgModel("input")
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}

func TestAX7_AngularHelper_SetNgModel_Good(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})
	_, err := nilCall(ah.SetNgModel("input", "value"))
	if err != nil {
		t.Fatalf("SetNgModel returned error: %v", err)
	}
	if ah.timeout <= 0 {
		t.Fatalf("timeout = %v", ah.timeout)
	}
}

func TestAX7_AngularHelper_SetNgModel_Bad(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "angular failed")
	})
	_, err := nilCall(ah.SetNgModel("input", "value"))
	if err == nil {
		t.Fatal("SetNgModel succeeded despite evaluation failure")
	}
	if !strings.Contains(err.Error(), "angular failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestAX7_AngularHelper_SetNgModel_Ugly(t *testing.T) {
	ah := ax7AngularHelper(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, nil)
	})
	_, err := nilCall(ah.SetNgModel("input", "value"))
	if err != nil && !strings.Contains(err.Error(), "not an Angular") && !strings.Contains(err.Error(), "could not") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ah.wv == nil {
		t.Fatal("helper lost webview")
	}
}
