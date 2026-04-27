// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newAngularTestHarness(t *testing.T, onMessage func(*fakeCDPTarget, cdpMessage)) (*AngularHelper, *fakeCDPTarget, *CDPClient) {
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
	return NewAngularHelper(wv), target, client
}

func TestAngular_SetTimeout_Good(t *testing.T) {
	ah := NewAngularHelper(&Webview{})
	ah.SetTimeout(5 * time.Second)
	if ah.timeout != 5*time.Second {
		t.Fatalf("SetTimeout = %v, want 5s", ah.timeout)
	}
}

func TestAngular_WaitForAngular_Bad_NotAngular(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, false)
	})

	if err := ah.WaitForAngular(); err == nil {
		t.Fatal("WaitForAngular succeeded for a non-Angular page")
	}
}

func TestAngular_WaitForAngular_Good(t *testing.T) {
	var evaluateCount int
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		evaluateCount++
		expr, _ := msg.Params["expression"].(string)
		if strings.Contains(expr, "getAllAngularRootElements") || strings.Contains(expr, "[ng-version]") {
			target.replyValue(msg.ID, true)
			return
		}
		target.replyValue(msg.ID, true)
	})

	if err := ah.WaitForAngular(); err != nil {
		t.Fatalf("WaitForAngular returned error: %v", err)
	}
	if evaluateCount < 2 {
		t.Fatalf("WaitForAngular made %d evaluate calls, want at least 2", evaluateCount)
	}
}

func TestAngular_waitForZoneStability_Good_FallsBackToPolling(t *testing.T) {
	var calls int
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		calls++
		expr, _ := msg.Params["expression"].(string)
		switch {
		case strings.Contains(expr, "new Promise"):
			target.replyError(msg.ID, "zone probe failed")
		default:
			target.replyValue(msg.ID, true)
		}
	})

	if err := ah.waitForZoneStability(context.Background()); err != nil {
		t.Fatalf("waitForZoneStability returned error: %v", err)
	}
	if calls < 2 {
		t.Fatalf("waitForZoneStability made %d evaluate calls, want at least 2", calls)
	}
}

func TestAngular_NavigateByRouter_Good(t *testing.T) {
	var expressions []string
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		expr, _ := msg.Params["expression"].(string)
		expressions = append(expressions, expr)
		if strings.Contains(expr, "navigateByUrl") {
			target.replyValue(msg.ID, true)
			return
		}
		target.replyValue(msg.ID, true)
	})

	if err := ah.NavigateByRouter("/dashboard"); err != nil {
		t.Fatalf("NavigateByRouter returned error: %v", err)
	}
	if len(expressions) < 2 {
		t.Fatalf("NavigateByRouter made %d evaluate calls, want at least 2", len(expressions))
	}
}

func TestAngular_NavigateByRouter_Bad(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyError(msg.ID, "could not find router")
	})

	if err := ah.NavigateByRouter("/dashboard"); err == nil {
		t.Fatal("NavigateByRouter succeeded despite evaluation error")
	}
}

func TestAngular_GetComponentProperty_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		expr, _ := msg.Params["expression"].(string)
		if !strings.Contains(expr, `const selector = "app-user";`) {
			t.Fatalf("expression did not quote selector: %s", expr)
		}
		if !strings.Contains(expr, `const propertyName = "displayName";`) {
			t.Fatalf("expression did not quote property name: %s", expr)
		}
		target.replyValue(msg.ID, "Ada")
	})

	got, err := ah.GetComponentProperty("app-user", "displayName")
	if err != nil {
		t.Fatalf("GetComponentProperty returned error: %v", err)
	}
	if got != "Ada" {
		t.Fatalf("GetComponentProperty = %v, want Ada", got)
	}
}

func TestAngular_SetComponentProperty_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		expr, _ := msg.Params["expression"].(string)
		if !strings.Contains(expr, `component[propertyName] = true;`) {
			t.Fatalf("expression did not set the component property: %s", expr)
		}
		target.replyValue(msg.ID, true)
	})

	if err := ah.SetComponentProperty("app-user", "active", true); err != nil {
		t.Fatalf("SetComponentProperty returned error: %v", err)
	}
}

func TestAngular_CallComponentMethod_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		expr, _ := msg.Params["expression"].(string)
		if !strings.Contains(expr, `component[methodName](1, "two")`) {
			t.Fatalf("expression did not marshal method args: %s", expr)
		}
		target.replyValue(msg.ID, map[string]any{"ok": true})
	})

	got, err := ah.CallComponentMethod("app-user", "save", 1, "two")
	if err != nil {
		t.Fatalf("CallComponentMethod returned error: %v", err)
	}
	if gotMap, ok := got.(map[string]any); !ok || gotMap["ok"] != true {
		t.Fatalf("CallComponentMethod = %#v, want ok=true", got)
	}
}

func TestAngular_TriggerChangeDetection_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})

	if err := ah.TriggerChangeDetection(); err != nil {
		t.Fatalf("TriggerChangeDetection returned error: %v", err)
	}
}

func TestAngular_GetService_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, map[string]any{"name": "session"})
	})

	got, err := ah.GetService("SessionService")
	if err != nil {
		t.Fatalf("GetService returned error: %v", err)
	}
	if gotMap, ok := got.(map[string]any); !ok || gotMap["name"] != "session" {
		t.Fatalf("GetService = %#v, want session map", got)
	}
}

func TestAngular_WaitForComponent_Good(t *testing.T) {
	var calls int
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
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

	if err := ah.WaitForComponent("app-user"); err != nil {
		t.Fatalf("WaitForComponent returned error: %v", err)
	}
	if calls < 2 {
		t.Fatalf("WaitForComponent calls = %d, want at least 2", calls)
	}
}

func TestAngular_DispatchEvent_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		expr, _ := msg.Params["expression"].(string)
		if !strings.Contains(expr, `new CustomEvent(eventName, { bubbles: true, detail: {"count":1} })`) && !strings.Contains(expr, `new CustomEvent(eventName, { bubbles: true, detail: {\"count\":1} })`) {
			t.Fatalf("expression did not dispatch custom event with detail: %s", expr)
		}
		target.replyValue(msg.ID, true)
	})

	if err := ah.DispatchEvent("app-user", "count-change", map[string]any{"count": 1}); err != nil {
		t.Fatalf("DispatchEvent returned error: %v", err)
	}
}

func TestAngular_GetNgModel_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, "hello")
	})

	got, err := ah.GetNgModel("input[name=email]")
	if err != nil {
		t.Fatalf("GetNgModel returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("GetNgModel = %v, want hello", got)
	}
}

func TestAngular_SetNgModel_Good(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	})

	if err := ah.SetNgModel(`input[name="x"]`, `";window.hacked=true;//`); err != nil {
		t.Fatalf("SetNgModel returned error: %v", err)
	}
}

func TestAngular_copyStringOnlyMap_Good(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want map[string]string
	}{
		{name: "map any", in: map[string]any{"a": "1", "b": 2}, want: map[string]string{"a": "1"}},
		{name: "map string", in: map[string]string{"c": "3"}, want: map[string]string{"c": "3"}},
		{name: "nil", in: nil, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := copyStringOnlyMap(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("copyStringOnlyMap len = %d, want %d", len(got), len(tc.want))
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Fatalf("copyStringOnlyMap[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}

func TestAngular_formatJSValue_Ugly_FallsBackToSprint(t *testing.T) {
	got := formatJSValue(make(chan int))
	if got == "null" {
		t.Fatalf("formatJSValue fallback returned %q, want quoted sprint output", got)
	}
	if !strings.HasPrefix(got, "\"") || !strings.HasSuffix(got, "\"") {
		t.Fatalf("formatJSValue fallback = %q, want quoted string output", got)
	}
}
