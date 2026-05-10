// SPDX-License-Identifier: EUPL-1.2

package webview

import (
	"testing"

	core "dappco.re/go"
)

// TestNewService_NoDebugURL_RegistersNilWebview — happy path lazy-injection.
// Empty DebugURL registers Service with nil Webview for late wiring.
func TestNewService_NoDebugURL_RegistersNilWebview(t *testing.T) {
	c := core.New(core.WithService(NewService(ServiceOptions{})))
	r := c.Service("webview")
	if !r.OK {
		t.Fatal("webview service not registered")
	}
	svc := r.Value.(*Service)
	if svc.Webview != nil {
		t.Fatal("expected nil Webview when DebugURL empty")
	}
}

// TestRegister_DefaultsRegistersWebview — imperative Register(c) shorthand.
func TestRegister_DefaultsRegistersWebview(t *testing.T) {
	c := core.New(core.WithService(Register))
	if !c.Service("webview").OK {
		t.Fatal("webview service not registered via Register")
	}
}
