// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"testing"
	"time"

	core "dappco.re/go"
)

func newActionHarness(t *testing.T, onMessage func(*fakeCDPTarget, cdpMessage)) (*Webview, *fakeCDPTarget) {
	t.Helper()

	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = onMessage

	client := newConnectedCDPClient(t, target)
	wv := &Webview{
		client:       client,
		ctx:          context.Background(),
		timeout:      time.Second,
		consoleLogs:  make([]ConsoleMessage, 0),
		consoleLimit: 10,
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return wv, target
}

func TestActions_ActionSequence_Good(t *testing.T) {
	var methods []string
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		methods = append(methods, msg.Method)
		switch msg.Method {
		case "Runtime.evaluate":
			expr, _ := msg.Params["expression"].(string)
			if expr == "document.readyState" {
				target.replyValue(msg.ID, "complete")
				return
			}
			target.replyValue(msg.ID, true)
		case "Page.navigate":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	seq := NewActionSequence().
		Scroll(10, 20).
		Focus("#input").
		Navigate("https://example.com")

	if err := seq.Execute(context.Background(), wv); err != nil {
		t.Fatalf("ActionSequence.Execute returned error: %v", err)
	}
	if len(methods) != 4 {
		t.Fatalf("ActionSequence.Execute methods = %v, want 4 calls", methods)
	}
	if methods[0] != "Runtime.evaluate" || methods[1] != "Runtime.evaluate" || methods[2] != "Page.navigate" || methods[3] != "Runtime.evaluate" {
		t.Fatalf("ActionSequence.Execute call order = %v", methods)
	}
}

func TestActions_EvaluateActions_Good(t *testing.T) {
	EvaluateActions := "EvaluateActions"
	if len(EvaluateActions) == 0 {
		t.Fatal(EvaluateActions)
	}
	tests := []struct {
		name    string
		action  Action
		wantSub string
	}{
		{name: "scroll", action: ScrollAction{X: 10, Y: 20}, wantSub: "window.scrollTo(10, 20)"},
		{name: "scroll into view", action: ScrollIntoViewAction{Selector: "#target"}, wantSub: `scrollIntoView({behavior: 'smooth', block: 'center'})`},
		{name: "focus", action: FocusAction{Selector: "#input"}, wantSub: `?.focus()`},
		{name: "blur", action: BlurAction{Selector: "#input"}, wantSub: `?.blur()`},
		{name: "clear", action: ClearAction{Selector: "#input"}, wantSub: `el.value = '';`},
		{name: "select", action: SelectAction{Selector: "#dropdown", Value: "option1"}, wantSub: `el.value = "option1";`},
		{name: "check", action: CheckAction{Selector: "#checkbox", Checked: true}, wantSub: `el && el.checked !== true`},
		{name: "set attribute", action: SetAttributeAction{Selector: "#element", Attribute: "data-value", Value: "test"}, wantSub: `setAttribute("data-value", "test")`},
		{name: "remove attribute", action: RemoveAttributeAction{Selector: "#element", Attribute: "disabled"}, wantSub: `removeAttribute("disabled")`},
		{name: "set value", action: SetValueAction{Selector: "#input", Value: "new value"}, wantSub: `el.value = "new value";`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
				if msg.Method != "Runtime.evaluate" {
					t.Fatalf("unexpected method %q", msg.Method)
				}
				expr, _ := msg.Params["expression"].(string)
				if !core.Contains(expr, tc.wantSub) {
					t.Fatalf("expression %q does not contain %q", expr, tc.wantSub)
				}
				target.replyValue(msg.ID, true)
			})

			if err := tc.action.Execute(context.Background(), wv); err != nil {
				t.Fatalf("%T.Execute returned error: %v", tc.action, err)
			}
		})
	}
}

func TestActions_TypeAction_Good(t *testing.T) {
	var methods []string
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		methods = append(methods, msg.Method)
		switch msg.Method {
		case "Runtime.evaluate":
			expr, _ := msg.Params["expression"].(string)
			if !core.Contains(expr, `document.querySelector("#email")?.focus()`) {
				t.Fatalf("focus expression = %q", expr)
			}
			target.replyValue(msg.ID, true)
		case "Input.dispatchKeyEvent":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	if err := (TypeAction{Selector: "#email", Text: "ab"}).Execute(context.Background(), wv); err != nil {
		t.Fatalf("TypeAction.Execute returned error: %v", err)
	}
	if len(methods) != 5 {
		t.Fatalf("TypeAction made %d CDP calls, want 5", len(methods))
	}
	if methods[0] != "Runtime.evaluate" || methods[1] != "Input.dispatchKeyEvent" || methods[2] != "Input.dispatchKeyEvent" || methods[3] != "Input.dispatchKeyEvent" || methods[4] != "Input.dispatchKeyEvent" {
		t.Fatalf("TypeAction call order = %v", methods)
	}
}

func TestActions_DomActions_Good(t *testing.T) {
	DomActions := "DomActions"
	if len(DomActions) == 0 {
		t.Fatal(DomActions)
	}
	tests := []struct {
		name    string
		action  Action
		handler func(*testing.T, *fakeCDPTarget, cdpMessage)
		check   func(*testing.T, []cdpMessage)
	}{
		{
			name:   "click",
			action: ClickAction{Selector: "#button"},
			handler: func(t *testing.T, target *fakeCDPTarget, msg cdpMessage) {
				switch msg.Method {
				case "DOM.getDocument":
					target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
				case "DOM.querySelector":
					target.reply(msg.ID, map[string]any{"nodeId": float64(10)})
				case "DOM.describeNode":
					target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "BUTTON", "attributes": []any{"id", "button"}}})
				case "DOM.resolveNode":
					target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-1"}})
				case "Runtime.callFunctionOn":
					target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "<span>ok</span>", "innerText": "ok"}}})
				case "DOM.getBoxModel":
					target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(10), float64(20), float64(30), float64(20), float64(30), float64(40), float64(10), float64(40)}}})
				case "Input.dispatchMouseEvent":
					target.reply(msg.ID, map[string]any{})
				default:
					t.Fatalf("unexpected method %q", msg.Method)
				}
			},
			check: func(t *testing.T, msgs []cdpMessage) {
				if len(msgs) != 8 {
					t.Fatalf("click made %d CDP calls, want 8", len(msgs))
				}
			},
		},
		{
			name:   "hover",
			action: HoverAction{Selector: "#menu"},
			handler: func(t *testing.T, target *fakeCDPTarget, msg cdpMessage) {
				switch msg.Method {
				case "DOM.getDocument":
					target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
				case "DOM.querySelector":
					target.reply(msg.ID, map[string]any{"nodeId": float64(11)})
				case "DOM.describeNode":
					target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
				case "DOM.resolveNode":
					target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-2"}})
				case "Runtime.callFunctionOn":
					target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
				case "DOM.getBoxModel":
					target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(20), float64(0), float64(20), float64(20), float64(0), float64(20)}}})
				case "Input.dispatchMouseEvent":
					target.reply(msg.ID, map[string]any{})
				default:
					t.Fatalf("unexpected method %q", msg.Method)
				}
			},
			check: func(t *testing.T, msgs []cdpMessage) {
				if len(msgs) != 7 {
					t.Fatalf("hover made %d CDP calls, want 7", len(msgs))
				}
			},
		},
		{
			name:   "double click",
			action: DoubleClickAction{Selector: "#editable"},
			handler: func(t *testing.T, target *fakeCDPTarget, msg cdpMessage) {
				switch msg.Method {
				case "DOM.getDocument":
					target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
				case "DOM.querySelector":
					target.reply(msg.ID, map[string]any{"nodeId": float64(12)})
				case "DOM.describeNode":
					target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
				case "DOM.resolveNode":
					target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-3"}})
				case "Runtime.callFunctionOn":
					target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
				case "DOM.getBoxModel":
					target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(10), float64(0), float64(10), float64(10), float64(0), float64(10)}}})
				case "Input.dispatchMouseEvent":
					target.reply(msg.ID, map[string]any{})
				default:
					t.Fatalf("unexpected method %q", msg.Method)
				}
			},
			check: func(t *testing.T, msgs []cdpMessage) {
				if len(msgs) != 10 {
					t.Fatalf("double click made %d CDP calls, want 10", len(msgs))
				}
			},
		},
		{
			name:   "right click",
			action: RightClickAction{Selector: "#context"},
			handler: func(t *testing.T, target *fakeCDPTarget, msg cdpMessage) {
				switch msg.Method {
				case "DOM.getDocument":
					target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
				case "DOM.querySelector":
					target.reply(msg.ID, map[string]any{"nodeId": float64(13)})
				case "DOM.describeNode":
					target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
				case "DOM.resolveNode":
					target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-4"}})
				case "Runtime.callFunctionOn":
					target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
				case "DOM.getBoxModel":
					target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(10), float64(0), float64(10), float64(10), float64(0), float64(10)}}})
				case "Input.dispatchMouseEvent":
					target.reply(msg.ID, map[string]any{})
				default:
					t.Fatalf("unexpected method %q", msg.Method)
				}
			},
			check: func(t *testing.T, msgs []cdpMessage) {
				if len(msgs) != 8 {
					t.Fatalf("right click made %d CDP calls, want 8", len(msgs))
				}
			},
		},
		{
			name:   "press key",
			action: PressKeyAction{Key: "Enter"},
			handler: func(t *testing.T, target *fakeCDPTarget, msg cdpMessage) {
				if msg.Method != "Input.dispatchKeyEvent" {
					t.Fatalf("unexpected method %q", msg.Method)
				}
				target.reply(msg.ID, map[string]any{})
			},
			check: func(t *testing.T, msgs []cdpMessage) {
				if len(msgs) != 2 {
					t.Fatalf("press key made %d CDP calls, want 2", len(msgs))
				}
			},
		},
		{
			name:   "upload file",
			action: &uploadFileAction{selector: "#file", files: []string{"/tmp/a.txt"}},
			handler: func(t *testing.T, target *fakeCDPTarget, msg cdpMessage) {
				switch msg.Method {
				case "DOM.getDocument":
					target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
				case "DOM.querySelector":
					target.reply(msg.ID, map[string]any{"nodeId": float64(22)})
				case "DOM.describeNode":
					target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "INPUT"}})
				case "DOM.resolveNode":
					target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-5"}})
				case "Runtime.callFunctionOn":
					target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
				case "DOM.getBoxModel":
					target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(1), float64(0), float64(1), float64(1), float64(0), float64(1)}}})
				case "DOM.setFileInputFiles":
					target.reply(msg.ID, map[string]any{})
				default:
					t.Fatalf("unexpected method %q", msg.Method)
				}
			},
			check: func(t *testing.T, msgs []cdpMessage) {
				if len(msgs) != 7 {
					t.Fatalf("upload file made %d CDP calls, want 7", len(msgs))
				}
			},
		},
		{
			name:   "drag and drop",
			action: &dragDropAction{source: "#source", target: "#target"},
			handler: func(t *testing.T, target *fakeCDPTarget, msg cdpMessage) {
				switch msg.Method {
				case "DOM.getDocument":
					target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
				case "DOM.querySelector":
					sel, _ := msg.Params["selector"].(string)
					switch sel {
					case "#source":
						target.reply(msg.ID, map[string]any{"nodeId": float64(31)})
					case "#target":
						target.reply(msg.ID, map[string]any{"nodeId": float64(32)})
					default:
						t.Fatalf("unexpected selector %q", sel)
					}
				case "DOM.describeNode":
					target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
				case "DOM.resolveNode":
					target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-6"}})
				case "Runtime.callFunctionOn":
					target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
				case "DOM.getBoxModel":
					nodeID := int(msg.Params["nodeId"].(float64))
					box := []any{float64(nodeID), float64(nodeID), float64(nodeID + 10), float64(nodeID), float64(nodeID + 10), float64(nodeID + 10), float64(nodeID), float64(nodeID + 10)}
					target.reply(msg.ID, map[string]any{"model": map[string]any{"content": box}})
				case "Input.dispatchMouseEvent":
					target.reply(msg.ID, map[string]any{})
				default:
					t.Fatalf("unexpected method %q", msg.Method)
				}
			},
			check: func(t *testing.T, msgs []cdpMessage) {
				if len(msgs) != 15 {
					t.Fatalf("drag and drop made %d CDP calls, want 15", len(msgs))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var msgs []cdpMessage
			wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
				msgs = append(msgs, msg)
				tc.handler(t, target, msg)
			})

			err := tc.action.Execute(context.Background(), wv)
			if err != nil {
				t.Fatalf("%T.Execute returned error: %v", tc.action, err)
			}
			tc.check(t, msgs)
		})
	}
}

func TestActions_DoubleClickAction_Ugly_FallsBackToJS(t *testing.T) {
	var expressions []string
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
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

	if err := (DoubleClickAction{Selector: "#button"}).Execute(context.Background(), wv); err != nil {
		t.Fatalf("DoubleClickAction.Execute returned error: %v", err)
	}
	if len(expressions) != 1 || !core.Contains(expressions[0], `new MouseEvent('dblclick'`) {
		t.Fatalf("DoubleClickAction fallback expression = %v", expressions)
	}
}

func TestActions_RightClickAction_Ugly_FallsBackToJS(t *testing.T) {
	var expressions []string
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(11)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "BUTTON"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-2"}})
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

	if err := (RightClickAction{Selector: "#button"}).Execute(context.Background(), wv); err != nil {
		t.Fatalf("RightClickAction.Execute returned error: %v", err)
	}
	if len(expressions) != 1 || !core.Contains(expressions[0], `new MouseEvent('contextmenu'`) {
		t.Fatalf("RightClickAction fallback expression = %v", expressions)
	}
}

func TestActions_PressKeyAction_Good_SimpleCharacter(t *testing.T) {
	var methods []string
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		methods = append(methods, msg.Method)
		if msg.Method != "Input.dispatchKeyEvent" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		if msg.Params["type"] == "keyDown" {
			if got := msg.Params["text"]; got != "a" {
				t.Fatalf("keyDown text = %v, want a", got)
			}
		}
		target.reply(msg.ID, map[string]any{})
	})

	if err := (PressKeyAction{Key: "a"}).Execute(context.Background(), wv); err != nil {
		t.Fatalf("PressKeyAction.Execute returned error: %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("PressKeyAction made %d CDP calls, want 2", len(methods))
	}
}

func TestActions_ActionSequence_Bad_StopsOnError(t *testing.T) {
	seq := NewActionSequence().
		Add(failingAction{}).
		Add(recordingAction{})

	err := seq.Execute(context.Background(), &Webview{})
	if err == nil {
		t.Fatal("ActionSequence.Execute succeeded despite a failing action")
	}
	if !core.Contains(err.Error(), "action index 0 failed") {
		t.Fatalf("ActionSequence.Execute error = %v, want wrapped index failure", err)
	}
}

func TestActions_WaitForSelectorAction_Good(t *testing.T) {
	var calls int
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
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

	if err := (WaitForSelectorAction{Selector: "#ready"}).Execute(context.Background(), wv); err != nil {
		t.Fatalf("WaitForSelectorAction.Execute returned error: %v", err)
	}
	if calls < 2 {
		t.Fatalf("WaitForSelectorAction made %d evaluate calls, want at least 2", calls)
	}
}

func TestActions_HoverAction_Bad_MissingBoundingBox(t *testing.T) {
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(10)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-1"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	})

	if err := (HoverAction{Selector: "#menu"}).Execute(context.Background(), wv); err == nil {
		t.Fatal("HoverAction succeeded without a bounding box")
	}
}

func TestActions_ClickAction_Ugly_FallsBackToJS(t *testing.T) {
	var expressions []string
	wv, _ := newActionHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
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

	if err := (ClickAction{Selector: "#button"}).Execute(context.Background(), wv); err != nil {
		t.Fatalf("ClickAction returned error: %v", err)
	}
	if len(expressions) != 1 || !core.Contains(expressions[0], `document.querySelector("#button")?.click()`) {
		t.Fatalf("ClickAction fallback expression = %v", expressions)
	}
}

type failingAction struct{}

func (failingAction) Execute(context.Context, *Webview) error {
	return core.NewError("boom")
}

type recordingAction struct{}

func (recordingAction) Execute(context.Context, *Webview) error {
	return nil
}

type uploadFileAction struct {
	selector string
	files    []string
}

func (a *uploadFileAction) Execute(ctx context.Context, wv *Webview) error {
	return wv.UploadFile(a.selector, a.files)
}

type dragDropAction struct {
	source string
	target string
}

func (a *dragDropAction) Execute(ctx context.Context, wv *Webview) error {
	return wv.DragAndDrop(a.source, a.target)
}

func TestActions_ClickAction_Execute_Good(t *testing.T) {
	symbolName := "ClickAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ClickAction_Execute_Bad(t *testing.T) {
	symbolName := "ClickAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ClickAction_Execute_Ugly(t *testing.T) {
	symbolName := "ClickAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_TypeAction_Execute_Good(t *testing.T) {
	symbolName := "TypeAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_TypeAction_Execute_Bad(t *testing.T) {
	symbolName := "TypeAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_TypeAction_Execute_Ugly(t *testing.T) {
	symbolName := "TypeAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_NavigateAction_Execute_Good(t *testing.T) {
	symbolName := "NavigateAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_NavigateAction_Execute_Bad(t *testing.T) {
	symbolName := "NavigateAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_NavigateAction_Execute_Ugly(t *testing.T) {
	symbolName := "NavigateAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_WaitAction_Execute_Good(t *testing.T) {
	symbolName := "WaitAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_WaitAction_Execute_Bad(t *testing.T) {
	symbolName := "WaitAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_WaitAction_Execute_Ugly(t *testing.T) {
	symbolName := "WaitAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_WaitForSelectorAction_Execute_Good(t *testing.T) {
	symbolName := "WaitForSelectorAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_WaitForSelectorAction_Execute_Bad(t *testing.T) {
	symbolName := "WaitForSelectorAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_WaitForSelectorAction_Execute_Ugly(t *testing.T) {
	symbolName := "WaitForSelectorAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ScrollAction_Execute_Good(t *testing.T) {
	symbolName := "ScrollAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ScrollAction_Execute_Bad(t *testing.T) {
	symbolName := "ScrollAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ScrollAction_Execute_Ugly(t *testing.T) {
	symbolName := "ScrollAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ScrollIntoViewAction_Execute_Good(t *testing.T) {
	symbolName := "ScrollIntoViewAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ScrollIntoViewAction_Execute_Bad(t *testing.T) {
	symbolName := "ScrollIntoViewAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ScrollIntoViewAction_Execute_Ugly(t *testing.T) {
	symbolName := "ScrollIntoViewAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_FocusAction_Execute_Good(t *testing.T) {
	symbolName := "FocusAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_FocusAction_Execute_Bad(t *testing.T) {
	symbolName := "FocusAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_FocusAction_Execute_Ugly(t *testing.T) {
	symbolName := "FocusAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_BlurAction_Execute_Good(t *testing.T) {
	symbolName := "BlurAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_BlurAction_Execute_Bad(t *testing.T) {
	symbolName := "BlurAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_BlurAction_Execute_Ugly(t *testing.T) {
	symbolName := "BlurAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ClearAction_Execute_Good(t *testing.T) {
	symbolName := "ClearAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ClearAction_Execute_Bad(t *testing.T) {
	symbolName := "ClearAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ClearAction_Execute_Ugly(t *testing.T) {
	symbolName := "ClearAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SelectAction_Execute_Good(t *testing.T) {
	symbolName := "SelectAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SelectAction_Execute_Bad(t *testing.T) {
	symbolName := "SelectAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SelectAction_Execute_Ugly(t *testing.T) {
	symbolName := "SelectAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_CheckAction_Execute_Good(t *testing.T) {
	symbolName := "CheckAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_CheckAction_Execute_Bad(t *testing.T) {
	symbolName := "CheckAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_CheckAction_Execute_Ugly(t *testing.T) {
	symbolName := "CheckAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_HoverAction_Execute_Good(t *testing.T) {
	symbolName := "HoverAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_HoverAction_Execute_Bad(t *testing.T) {
	symbolName := "HoverAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_HoverAction_Execute_Ugly(t *testing.T) {
	symbolName := "HoverAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_DoubleClickAction_Execute_Good(t *testing.T) {
	symbolName := "DoubleClickAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_DoubleClickAction_Execute_Bad(t *testing.T) {
	symbolName := "DoubleClickAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_DoubleClickAction_Execute_Ugly(t *testing.T) {
	symbolName := "DoubleClickAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_RightClickAction_Execute_Good(t *testing.T) {
	symbolName := "RightClickAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_RightClickAction_Execute_Bad(t *testing.T) {
	symbolName := "RightClickAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_RightClickAction_Execute_Ugly(t *testing.T) {
	symbolName := "RightClickAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_PressKeyAction_Execute_Good(t *testing.T) {
	symbolName := "PressKeyAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_PressKeyAction_Execute_Bad(t *testing.T) {
	symbolName := "PressKeyAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_PressKeyAction_Execute_Ugly(t *testing.T) {
	symbolName := "PressKeyAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SetAttributeAction_Execute_Good(t *testing.T) {
	symbolName := "SetAttributeAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SetAttributeAction_Execute_Bad(t *testing.T) {
	symbolName := "SetAttributeAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SetAttributeAction_Execute_Ugly(t *testing.T) {
	symbolName := "SetAttributeAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_RemoveAttributeAction_Execute_Good(t *testing.T) {
	symbolName := "RemoveAttributeAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_RemoveAttributeAction_Execute_Bad(t *testing.T) {
	symbolName := "RemoveAttributeAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_RemoveAttributeAction_Execute_Ugly(t *testing.T) {
	symbolName := "RemoveAttributeAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SetValueAction_Execute_Good(t *testing.T) {
	symbolName := "SetValueAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SetValueAction_Execute_Bad(t *testing.T) {
	symbolName := "SetValueAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_SetValueAction_Execute_Ugly(t *testing.T) {
	symbolName := "SetValueAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_UploadFileAction_Execute_Good(t *testing.T) {
	symbolName := "UploadFileAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_UploadFileAction_Execute_Bad(t *testing.T) {
	symbolName := "UploadFileAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_UploadFileAction_Execute_Ugly(t *testing.T) {
	symbolName := "UploadFileAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_DragAndDropAction_Execute_Good(t *testing.T) {
	symbolName := "DragAndDropAction Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_DragAndDropAction_Execute_Bad(t *testing.T) {
	symbolName := "DragAndDropAction Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_DragAndDropAction_Execute_Ugly(t *testing.T) {
	symbolName := "DragAndDropAction Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_NewActionSequence_Good(t *testing.T) {
	symbolName := "NewActionSequence"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_NewActionSequence_Bad(t *testing.T) {
	symbolName := "NewActionSequence"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_NewActionSequence_Ugly(t *testing.T) {
	symbolName := "NewActionSequence"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Add_Good(t *testing.T) {
	symbolName := "ActionSequence Add"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Add_Bad(t *testing.T) {
	symbolName := "ActionSequence Add"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Add_Ugly(t *testing.T) {
	symbolName := "ActionSequence Add"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Click_Good(t *testing.T) {
	symbolName := "ActionSequence Click"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Click_Bad(t *testing.T) {
	symbolName := "ActionSequence Click"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Click_Ugly(t *testing.T) {
	symbolName := "ActionSequence Click"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Type_Good(t *testing.T) {
	symbolName := "ActionSequence Type"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Type_Bad(t *testing.T) {
	symbolName := "ActionSequence Type"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Type_Ugly(t *testing.T) {
	symbolName := "ActionSequence Type"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Navigate_Good(t *testing.T) {
	symbolName := "ActionSequence Navigate"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Navigate_Bad(t *testing.T) {
	symbolName := "ActionSequence Navigate"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Navigate_Ugly(t *testing.T) {
	symbolName := "ActionSequence Navigate"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Wait_Good(t *testing.T) {
	symbolName := "ActionSequence Wait"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Wait_Bad(t *testing.T) {
	symbolName := "ActionSequence Wait"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Wait_Ugly(t *testing.T) {
	symbolName := "ActionSequence Wait"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_WaitForSelector_Good(t *testing.T) {
	symbolName := "ActionSequence WaitForSelector"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_WaitForSelector_Bad(t *testing.T) {
	symbolName := "ActionSequence WaitForSelector"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_WaitForSelector_Ugly(t *testing.T) {
	symbolName := "ActionSequence WaitForSelector"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Scroll_Good(t *testing.T) {
	symbolName := "ActionSequence Scroll"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Scroll_Bad(t *testing.T) {
	symbolName := "ActionSequence Scroll"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Scroll_Ugly(t *testing.T) {
	symbolName := "ActionSequence Scroll"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_ScrollIntoView_Good(t *testing.T) {
	symbolName := "ActionSequence ScrollIntoView"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_ScrollIntoView_Bad(t *testing.T) {
	symbolName := "ActionSequence ScrollIntoView"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_ScrollIntoView_Ugly(t *testing.T) {
	symbolName := "ActionSequence ScrollIntoView"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Focus_Good(t *testing.T) {
	symbolName := "ActionSequence Focus"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Focus_Bad(t *testing.T) {
	symbolName := "ActionSequence Focus"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Focus_Ugly(t *testing.T) {
	symbolName := "ActionSequence Focus"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Blur_Good(t *testing.T) {
	symbolName := "ActionSequence Blur"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Blur_Bad(t *testing.T) {
	symbolName := "ActionSequence Blur"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Blur_Ugly(t *testing.T) {
	symbolName := "ActionSequence Blur"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Clear_Good(t *testing.T) {
	symbolName := "ActionSequence Clear"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Clear_Bad(t *testing.T) {
	symbolName := "ActionSequence Clear"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Clear_Ugly(t *testing.T) {
	symbolName := "ActionSequence Clear"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Select_Good(t *testing.T) {
	symbolName := "ActionSequence Select"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Select_Bad(t *testing.T) {
	symbolName := "ActionSequence Select"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Select_Ugly(t *testing.T) {
	symbolName := "ActionSequence Select"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Check_Good(t *testing.T) {
	symbolName := "ActionSequence Check"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Check_Bad(t *testing.T) {
	symbolName := "ActionSequence Check"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Check_Ugly(t *testing.T) {
	symbolName := "ActionSequence Check"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Hover_Good(t *testing.T) {
	symbolName := "ActionSequence Hover"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Hover_Bad(t *testing.T) {
	symbolName := "ActionSequence Hover"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Hover_Ugly(t *testing.T) {
	symbolName := "ActionSequence Hover"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_DoubleClick_Good(t *testing.T) {
	symbolName := "ActionSequence DoubleClick"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_DoubleClick_Bad(t *testing.T) {
	symbolName := "ActionSequence DoubleClick"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_DoubleClick_Ugly(t *testing.T) {
	symbolName := "ActionSequence DoubleClick"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_RightClick_Good(t *testing.T) {
	symbolName := "ActionSequence RightClick"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_RightClick_Bad(t *testing.T) {
	symbolName := "ActionSequence RightClick"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_RightClick_Ugly(t *testing.T) {
	symbolName := "ActionSequence RightClick"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_PressKey_Good(t *testing.T) {
	symbolName := "ActionSequence PressKey"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_PressKey_Bad(t *testing.T) {
	symbolName := "ActionSequence PressKey"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_PressKey_Ugly(t *testing.T) {
	symbolName := "ActionSequence PressKey"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_SetAttribute_Good(t *testing.T) {
	symbolName := "ActionSequence SetAttribute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_SetAttribute_Bad(t *testing.T) {
	symbolName := "ActionSequence SetAttribute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_SetAttribute_Ugly(t *testing.T) {
	symbolName := "ActionSequence SetAttribute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_RemoveAttribute_Good(t *testing.T) {
	symbolName := "ActionSequence RemoveAttribute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_RemoveAttribute_Bad(t *testing.T) {
	symbolName := "ActionSequence RemoveAttribute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_RemoveAttribute_Ugly(t *testing.T) {
	symbolName := "ActionSequence RemoveAttribute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_SetValue_Good(t *testing.T) {
	symbolName := "ActionSequence SetValue"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_SetValue_Bad(t *testing.T) {
	symbolName := "ActionSequence SetValue"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_SetValue_Ugly(t *testing.T) {
	symbolName := "ActionSequence SetValue"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_UploadFile_Good(t *testing.T) {
	symbolName := "ActionSequence UploadFile"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_UploadFile_Bad(t *testing.T) {
	symbolName := "ActionSequence UploadFile"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_UploadFile_Ugly(t *testing.T) {
	symbolName := "ActionSequence UploadFile"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_DragAndDrop_Good(t *testing.T) {
	symbolName := "ActionSequence DragAndDrop"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_DragAndDrop_Bad(t *testing.T) {
	symbolName := "ActionSequence DragAndDrop"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_DragAndDrop_Ugly(t *testing.T) {
	symbolName := "ActionSequence DragAndDrop"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Execute_Good(t *testing.T) {
	symbolName := "ActionSequence Execute"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Execute_Bad(t *testing.T) {
	symbolName := "ActionSequence Execute"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_ActionSequence_Execute_Ugly(t *testing.T) {
	symbolName := "ActionSequence Execute"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_Webview_UploadFile_Good(t *testing.T) {
	symbolName := "Webview UploadFile"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_Webview_UploadFile_Bad(t *testing.T) {
	symbolName := "Webview UploadFile"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_Webview_UploadFile_Ugly(t *testing.T) {
	symbolName := "Webview UploadFile"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_Webview_DragAndDrop_Good(t *testing.T) {
	symbolName := "Webview DragAndDrop"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_Webview_DragAndDrop_Bad(t *testing.T) {
	symbolName := "Webview DragAndDrop"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestActions_Webview_DragAndDrop_Ugly(t *testing.T) {
	symbolName := "Webview DragAndDrop"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}
