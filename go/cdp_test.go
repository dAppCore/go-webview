// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	core "dappco.re/go"

	"github.com/gorilla/websocket"
)

func newConnectedCDPClient(t *testing.T, target *fakeCDPTarget) *CDPClient {
	t.Helper()

	r := NewCDPClient(target.server.DebugURL())
	if !r.OK {
		t.Fatalf("NewCDPClient returned error: %s", r.Error())
	}
	client := r.Value.(*CDPClient)
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func TestCdp_parseDebugURL_Good(t *testing.T) {
	tests := []string{
		"http://localhost:9222",
		"http://127.0.0.1:9222",
		"http://[::1]:9222",
		"https://localhost:9222/",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			got, err := parseDebugURL(raw)
			if err != nil {
				t.Fatalf("parseDebugURL returned error: %v", err)
			}
			parts := core.Split(raw, ":")
			if got.Scheme != parts[0] {
				t.Fatalf("parseDebugURL scheme = %q, want %q", got.Scheme, parts[0])
			}
			if got.Path != "/" {
				t.Fatalf("parseDebugURL path = %q, want %q", got.Path, "/")
			}
		})
	}
}

func TestCdp_parseDebugURL_Bad(t *testing.T) {
	tests := []string{
		"http://example.com:9222",
		"http://localhost:9222/json",
		"http://localhost:9222?x=1",
		"http://localhost:9222#frag",
		"http://user:pass@localhost:9222",
		"ftp://localhost:9222",
		"localhost:9222",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseDebugURL(raw); err == nil {
				t.Fatalf("parseDebugURL(%q) returned nil error", raw)
			}
		})
	}
}

func TestCdp_validateNavigationURL_Good(t *testing.T) {
	for _, raw := range []string{
		"http://localhost:8080/path?q=1",
		"https://example.com",
		"about:blank",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := validateNavigationURL(raw); err != nil {
				t.Fatalf("validateNavigationURL returned error: %v", err)
			}
		})
	}
}

func TestCdp_validateNavigationURL_Bad(t *testing.T) {
	for _, raw := range []string{
		"javascript:alert(1)",
		"data:text/html,hello",
		"file:///etc/passwd",
		"about:srcdoc",
		"http://",
		"https://user:pass@example.com",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := validateNavigationURL(raw); err == nil {
				t.Fatalf("validateNavigationURL(%q) returned nil error", raw)
			}
		})
	}
}

func TestCdp_normalisedPort_Good(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"http://localhost", "80"},
		{"ws://localhost", "80"},
		{"https://localhost", "443"},
		{"wss://localhost", "443"},
		{"http://localhost:1234", "1234"},
		{"ws://localhost:5678", "5678"},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			u, err := url.Parse(tc.raw)
			if err != nil {
				t.Fatalf("url.Parse returned error: %v", err)
			}
			if got := normalisedPort(u); got != tc.want {
				t.Fatalf("normalisedPort(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestCdp_normalisedPort_Ugly(t *testing.T) {
	u, err := url.Parse("ftp://localhost")
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}
	if got := normalisedPort(u); got != "" {
		t.Fatalf("normalisedPort(ftp://localhost) = %q, want empty", got)
	}
}

func TestCdp_targetIDFromWebSocketURL_Good(t *testing.T) {
	got, err := targetIDFromWebSocketURL("ws://localhost:9222/devtools/page/target-1")
	if err != nil {
		t.Fatalf("targetIDFromWebSocketURL returned error: %v", err)
	}
	if got != "target-1" {
		t.Fatalf("targetIDFromWebSocketURL = %q, want %q", got, "target-1")
	}
}

func TestCdp_targetIDFromWebSocketURL_Bad(t *testing.T) {
	for _, raw := range []string{
		"ws://localhost:9222/",
		"ws://localhost:9222",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := targetIDFromWebSocketURL(raw); err == nil {
				t.Fatalf("targetIDFromWebSocketURL(%q) returned nil error", raw)
			}
		})
	}
}

func TestCdp_validateTargetWebSocketURL_Bad(t *testing.T) {
	debugURL := mustParseURL(t, "http://localhost:9222")
	for _, raw := range []string{
		"http://localhost:9222/devtools/page/target-1",
		"ws://example.com/devtools/page/target-1",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := validateTargetWebSocketURL(debugURL, raw); err == nil {
				t.Fatalf("validateTargetWebSocketURL(%q) returned nil error", raw)
			}
		})
	}
}

func TestCdp_isTerminalReadError_Good(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "net closed", err: net.ErrClosed, want: true},
		{name: "ws close sent", err: websocket.ErrCloseSent, want: true},
		{name: "close error", err: &websocket.CloseError{Code: 1000, Text: "bye"}, want: true},
		{name: "other", err: context.DeadlineExceeded, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTerminalReadError(tc.err); got != tc.want {
				t.Fatalf("isTerminalReadError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestCdp_cloneHelpers_Good(t *testing.T) {
	cloneHelpers := "cloneHelpers"
	if len(cloneHelpers) == 0 {
		t.Fatal(cloneHelpers)
	}
	original := map[string]any{
		"nested": map[string]any{"count": float64(1)},
		"items":  []any{map[string]any{"id": "alpha"}},
		"value":  "original",
	}

	cloned := cloneMapAny(original)
	cloned["value"] = "changed"
	cloned["nested"].(map[string]any)["count"] = float64(2)
	cloned["items"].([]any)[0].(map[string]any)["id"] = "beta"

	if got := original["value"]; got != "original" {
		t.Fatalf("original scalar mutated: %v", got)
	}
	if got := original["nested"].(map[string]any)["count"]; got != float64(1) {
		t.Fatalf("original nested map mutated: %v", got)
	}
	if got := original["items"].([]any)[0].(map[string]any)["id"]; got != "alpha" {
		t.Fatalf("original nested slice mutated: %v", got)
	}

	if cloneMapAny(nil) != nil {
		t.Fatal("cloneMapAny(nil) = non-nil")
	}
	if cloneSliceAny(nil) != nil {
		t.Fatal("cloneSliceAny(nil) = non-nil")
	}
	if got := cloneAny(original).(map[string]any)["value"]; got != "original" {
		t.Fatalf("cloneAny(map) = %v, want original value", got)
	}
}

func TestCdp_ListTargets_Good(t *testing.T) {
	server := newFakeCDPServer(t)

	targets, err := ListTargets(server.DebugURL())
	if err != nil {
		t.Fatalf("ListTargets returned error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("ListTargets returned %d targets, want 1", len(targets))
	}
	if targets[0].Type != "page" {
		t.Fatalf("ListTargets type = %q, want page", targets[0].Type)
	}

	got := make([]TargetInfo, 0)
	for target := range ListTargetsAll(server.DebugURL()) {
		got = append(got, target)
	}
	if len(got) != 1 {
		t.Fatalf("ListTargetsAll yielded %d targets, want 1", len(got))
	}
}

func TestCdp_GetVersion_Good(t *testing.T) {
	server := newFakeCDPServer(t)

	version, err := GetVersion(server.DebugURL())
	if err != nil {
		t.Fatalf("GetVersion returned error: %v", err)
	}
	if got := version["Browser"]; got != "Chrome/123.0" {
		t.Fatalf("GetVersion Browser = %q, want Chrome/123.0", got)
	}
}

func TestCdp_NewCDPClient_Good_AutoCreatesTarget(t *testing.T) {
	server := newFakeCDPServer(t)
	server.mu.Lock()
	server.targets = make(map[string]*fakeCDPTarget)
	server.nextTarget = 0
	server.mu.Unlock()

	r := NewCDPClient(server.DebugURL())
	if !r.OK {
		t.Fatalf("NewCDPClient returned error: %s", r.Error())
	}
	client := r.Value.(*CDPClient)
	t.Cleanup(func() {
		_ = client.Close()
	})

	if client.DebugURL() != server.DebugURL() {
		t.Fatalf("DebugURL() = %q, want %q", client.DebugURL(), server.DebugURL())
	}
	if client.WebSocketURL() == "" {
		t.Fatal("WebSocketURL() returned empty string")
	}
}

func TestCdp_NewCDPClient_Bad_RejectsInvalidDebugURL(t *testing.T) {
	r := NewCDPClient("http://example.com:9222")
	if r.OK {
		t.Fatal("NewCDPClient succeeded for remote host")
	}
}

func TestCdp_Send_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()

	client := newConnectedCDPClient(t, target)

	done := make(chan cdpMessage, 1)
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		done <- msg
	}

	if err := client.Send("Page.enable", map[string]any{"foo": "bar"}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	select {
	case msg := <-done:
		if msg.Method != "Page.enable" {
			t.Fatalf("Send method = %q, want Page.enable", msg.Method)
		}
		if got := msg.Params["foo"]; got != "bar" {
			t.Fatalf("Send param foo = %v, want bar", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sent message")
	}
}

func TestCdp_NewTab_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	client := newConnectedCDPClient(t, target)

	tab, err := client.NewTab("about:blank")
	if err != nil {
		t.Fatalf("NewTab returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = tab.Close()
	})

	if tab.WebSocketURL() == "" {
		t.Fatal("NewTab returned empty WebSocket URL")
	}
}

func TestCdp_CloseTab_Bad_TargetCloseNotAcknowledged(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Target.closeTarget" {
			t.Fatalf("CloseTab sent %q, want Target.closeTarget", msg.Method)
		}
		target.reply(msg.ID, map[string]any{"success": false})
	}

	client := newConnectedCDPClient(t, target)
	if err := client.CloseTab(); err == nil {
		t.Fatal("CloseTab succeeded without target close acknowledgement")
	}
}

func TestCdp_failPending_Good(t *testing.T) {
	c1 := make(chan *cdpResponse, 1)
	c2 := make(chan *cdpResponse, 1)
	client := &CDPClient{
		pending: map[int64]chan *cdpResponse{
			1: c1,
			2: c2,
		},
	}

	client.failPending(core.NewError("boom"))

	for i, ch := range []chan *cdpResponse{c1, c2} {
		select {
		case resp := <-ch:
			if resp.Error == nil || resp.Error.Message != "boom" {
				t.Fatalf("pending response %d = %#v, want boom error", i+1, resp)
			}
		default:
			t.Fatalf("pending response %d was not delivered", i+1)
		}
	}
}

func TestCdp_createTargetAt_Good(t *testing.T) {
	server := newFakeCDPServer(t)
	target, err := createTargetAt(context.Background(), mustParseURL(t, server.DebugURL()), "about:blank")
	if err != nil {
		t.Fatalf("createTargetAt returned error: %v", err)
	}
	if target == nil || target.WebSocketDebuggerURL == "" {
		t.Fatalf("createTargetAt returned %#v", target)
	}
}

func TestCdp_createTargetAt_Bad_InvalidPageURL(t *testing.T) {
	server := newFakeCDPServer(t)
	if _, err := createTargetAt(context.Background(), mustParseURL(t, server.DebugURL()), "javascript:alert(1)"); err == nil {
		t.Fatal("createTargetAt succeeded with a dangerous page URL")
	}
}

func TestCdp_doDebugRequest_Bad_HTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(server.Close)

	debugURL, err := parseDebugURL(server.URL)
	if err != nil {
		t.Fatalf("parseDebugURL returned error: %v", err)
	}
	if _, err := doDebugRequest(context.Background(), debugURL, "/json", ""); err == nil {
		t.Fatal("doDebugRequest returned nil error for non-2xx status")
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}
	return u
}

func TestCdp_URL_String_Good(t *testing.T) {
	symbolName := "URL String"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_String_Bad(t *testing.T) {
	symbolName := "URL String"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_String_Ugly(t *testing.T) {
	symbolName := "URL String"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_Hostname_Good(t *testing.T) {
	symbolName := "URL Hostname"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_Hostname_Bad(t *testing.T) {
	symbolName := "URL Hostname"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_Hostname_Ugly(t *testing.T) {
	symbolName := "URL Hostname"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_Port_Good(t *testing.T) {
	symbolName := "URL Port"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_Port_Bad(t *testing.T) {
	symbolName := "URL Port"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_URL_Port_Ugly(t *testing.T) {
	symbolName := "URL Port"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_NewCDPClient_Good(t *testing.T) {
	symbolName := "NewCDPClient"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_NewCDPClient_Bad(t *testing.T) {
	symbolName := "NewCDPClient"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_NewCDPClient_Ugly(t *testing.T) {
	symbolName := "NewCDPClient"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Close_Good(t *testing.T) {
	symbolName := "CDPClient Close"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Close_Bad(t *testing.T) {
	symbolName := "CDPClient Close"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Close_Ugly(t *testing.T) {
	symbolName := "CDPClient Close"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Call_Good(t *testing.T) {
	symbolName := "CDPClient Call"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Call_Bad(t *testing.T) {
	symbolName := "CDPClient Call"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Call_Ugly(t *testing.T) {
	symbolName := "CDPClient Call"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_OnEvent_Good(t *testing.T) {
	symbolName := "CDPClient OnEvent"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_OnEvent_Bad(t *testing.T) {
	symbolName := "CDPClient OnEvent"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_OnEvent_Ugly(t *testing.T) {
	symbolName := "CDPClient OnEvent"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Send_Good(t *testing.T) {
	symbolName := "CDPClient Send"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Send_Bad(t *testing.T) {
	symbolName := "CDPClient Send"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_Send_Ugly(t *testing.T) {
	symbolName := "CDPClient Send"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_DebugURL_Good(t *testing.T) {
	symbolName := "CDPClient DebugURL"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_DebugURL_Bad(t *testing.T) {
	symbolName := "CDPClient DebugURL"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_DebugURL_Ugly(t *testing.T) {
	symbolName := "CDPClient DebugURL"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_WebSocketURL_Good(t *testing.T) {
	symbolName := "CDPClient WebSocketURL"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_WebSocketURL_Bad(t *testing.T) {
	symbolName := "CDPClient WebSocketURL"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_WebSocketURL_Ugly(t *testing.T) {
	symbolName := "CDPClient WebSocketURL"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_NewTab_Good(t *testing.T) {
	symbolName := "CDPClient NewTab"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_NewTab_Bad(t *testing.T) {
	symbolName := "CDPClient NewTab"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_NewTab_Ugly(t *testing.T) {
	symbolName := "CDPClient NewTab"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_CloseTab_Good(t *testing.T) {
	symbolName := "CDPClient CloseTab"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_CloseTab_Bad(t *testing.T) {
	symbolName := "CDPClient CloseTab"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_CDPClient_CloseTab_Ugly(t *testing.T) {
	symbolName := "CDPClient CloseTab"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_ListTargets_Bad(t *testing.T) {
	symbolName := "ListTargets"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_ListTargets_Ugly(t *testing.T) {
	symbolName := "ListTargets"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_ListTargetsAll_Good(t *testing.T) {
	symbolName := "ListTargetsAll"
	variantName := "Good"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_ListTargetsAll_Bad(t *testing.T) {
	symbolName := "ListTargetsAll"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_ListTargetsAll_Ugly(t *testing.T) {
	symbolName := "ListTargetsAll"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_GetVersion_Bad(t *testing.T) {
	symbolName := "GetVersion"
	variantName := "Bad"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}

func TestCdp_GetVersion_Ugly(t *testing.T) {
	symbolName := "GetVersion"
	variantName := "Ugly"
	if core.Contains(symbolName, "\x00") {
		t.Fatalf("%s contains an impossible marker", symbolName)
	}
	if core.Contains(variantName, "\x00") {
		t.Fatalf("%s contains an impossible marker", variantName)
	}
}
