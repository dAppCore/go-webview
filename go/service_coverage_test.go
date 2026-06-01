// SPDX-License-Identifier: EUPL-1.2

package webview

import (
	"testing"
	"time"

	core "dappco.re/go"
)

// enableConsoleResponder answers the three domain-enable calls New(...)
// issues through enableConsole, so a Service can be constructed against
// a fake CDP endpoint without a real browser.
//
//	target.onMessage = enableConsoleResponder(t)
func enableConsoleResponder(t *testing.T) func(*fakeCDPTarget, cdpMessage) {
	t.Helper()
	return func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Runtime.enable", "Page.enable", "DOM.enable":
			target.reply(msg.ID, map[string]any{})
		default:
			target.reply(msg.ID, map[string]any{})
		}
	}
}

// TestServiceCoverage_NewService_WithDebugURL_Good drives the live-wire
// branch of NewService: a non-empty DebugURL constructs and connects a
// *Webview, leaving svc.Webview populated.
func TestServiceCoverage_NewService_WithDebugURL_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	server.primaryTarget().onMessage = enableConsoleResponder(t)

	c := core.New(core.WithService(NewService(ServiceOptions{
		DebugURL:     server.DebugURL(),
		Timeout:      5 * time.Second,
		ConsoleLimit: 200,
	})))

	r := c.Service("webview")
	if !r.OK {
		t.Fatalf("webview service not registered: %s", r.Error())
	}
	svc := r.Value.(*Service)
	if svc.Webview == nil {
		t.Fatal("expected non-nil Webview when DebugURL supplied")
	}
	if svc.Webview.timeout != 5*time.Second {
		t.Fatalf("Webview timeout = %v, want 5s", svc.Webview.timeout)
	}
	if svc.Webview.consoleLimit != 200 {
		t.Fatalf("Webview consoleLimit = %d, want 200", svc.Webview.consoleLimit)
	}
	t.Cleanup(func() { _ = svc.Webview.Close() })
}

// TestServiceCoverage_NewService_WithDebugURL_Defaults exercises the
// zero-Timeout / zero-ConsoleLimit path: the package defaults survive
// because the optional options are not appended.
func TestServiceCoverage_NewService_WithDebugURL_Defaults(t *testing.T) {
	server := newFakeCDPServer(t)
	server.primaryTarget().onMessage = enableConsoleResponder(t)

	c := core.New(core.WithService(NewService(ServiceOptions{
		DebugURL: server.DebugURL(),
	})))

	r := c.Service("webview")
	if !r.OK {
		t.Fatalf("webview service not registered: %s", r.Error())
	}
	svc := r.Value.(*Service)
	if svc.Webview == nil {
		t.Fatal("expected non-nil Webview when DebugURL supplied")
	}
	if svc.Webview.timeout != 30*time.Second {
		t.Fatalf("default Webview timeout = %v, want 30s", svc.Webview.timeout)
	}
	if svc.Webview.consoleLimit != 1000 {
		t.Fatalf("default Webview consoleLimit = %d, want 1000", svc.Webview.consoleLimit)
	}
	t.Cleanup(func() { _ = svc.Webview.Close() })
}

// TestServiceCoverage_NewService_WithDebugURL_Bad feeds a DebugURL that
// cannot connect; NewService must propagate the failed Result rather
// than register a half-built Service.
func TestServiceCoverage_NewService_WithDebugURL_Bad(t *testing.T) {
	factory := NewService(ServiceOptions{DebugURL: "http://127.0.0.1:0"})
	r := factory(core.New())
	if r.OK {
		t.Fatal("NewService with unreachable DebugURL returned OK result")
	}
}
