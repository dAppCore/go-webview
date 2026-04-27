// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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
				if !strings.Contains(expr, tc.wantSub) {
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
			if !strings.Contains(expr, `document.querySelector("#email")?.focus()`) {
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
	if len(expressions) != 1 || !strings.Contains(expressions[0], `new MouseEvent('dblclick'`) {
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
	if len(expressions) != 1 || !strings.Contains(expressions[0], `new MouseEvent('contextmenu'`) {
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
	if !strings.Contains(err.Error(), "action index 0 failed") {
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
	if len(expressions) != 1 || !strings.Contains(expressions[0], `document.querySelector("#button")?.click()`) {
		t.Fatalf("ClickAction fallback expression = %v", expressions)
	}
}

type failingAction struct{}

func (failingAction) Execute(context.Context, *Webview) error {
	return errors.New("boom")
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
