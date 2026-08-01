// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"testing"
)

func TestCdpHelpers_cdpURL_String_Good(t *testing.T) {
	u := &cdpURL{
		Scheme:   "http",
		Host:     "localhost:9222",
		Path:     "/json",
		RawQuery: "a=1",
		Fragment: "frag",
	}
	if got := u.String(); got != "http://localhost:9222/json?a=1#frag" {
		t.Fatalf("String = %q", got)
	}
}

func TestCdpHelpers_cdpURL_String_Bad_NilReceiver(t *testing.T) {
	var u *cdpURL
	if got := u.String(); got != "" {
		t.Fatalf("nil cdpURL.String = %q, want empty", got)
	}
	if got := u.Hostname(); got != "" {
		t.Fatalf("nil cdpURL.Hostname = %q, want empty", got)
	}
	if got := u.Port(); got != "" {
		t.Fatalf("nil cdpURL.Port = %q, want empty", got)
	}
}

func TestCdpHelpers_cdpURL_String_Ugly_RawPathPreferred(t *testing.T) {
	// RawPath, when present, must override Path in the rendered string.
	u := &cdpURL{Scheme: "http", Host: "localhost:9222", Path: "/decoded", RawPath: "/raw%2Fpath"}
	if got := u.String(); got != "http://localhost:9222/raw%2Fpath" {
		t.Fatalf("String = %q, want RawPath form", got)
	}
}

func TestCdpHelpers_cdpURL_HostnamePort_Good(t *testing.T) {
	u := &cdpURL{hostname: "127.0.0.1", port: "9222"}
	if u.Hostname() != "127.0.0.1" {
		t.Fatalf("Hostname = %q", u.Hostname())
	}
	if u.Port() != "9222" {
		t.Fatalf("Port = %q", u.Port())
	}
}

func TestCdpHelpers_ListTargets_Bad_InvalidDebugURL(t *testing.T) {
	if _, err := ListTargets("ftp://localhost:9222"); err == nil {
		t.Fatal("ListTargets accepted a non-http debug URL")
	}
}

func TestCdpHelpers_GetVersion_Bad_InvalidDebugURL(t *testing.T) {
	if _, err := GetVersion("ftp://localhost:9222"); err == nil {
		t.Fatal("GetVersion accepted a non-http debug URL")
	}
}

func TestCdpHelpers_ListTargetsAll_Bad_ErrorYieldsNothing(t *testing.T) {
	// An invalid debug URL makes ListTargets fail; the iterator must yield
	// nothing rather than panic.
	count := 0
	for range ListTargetsAll("ftp://localhost:9222") {
		count++
	}
	if count != 0 {
		t.Fatalf("ListTargetsAll yielded %d targets on error, want 0", count)
	}
}

func TestCdpHelpers_ListTargetsAll_Ugly_ConsumerStopsEarly(t *testing.T) {
	server := newFakeCDPServer(t)
	server.addTarget("target-2")

	count := 0
	for range ListTargetsAll(server.DebugURL()) {
		count++
		break // exercise the yield-returns-false early-stop branch
	}
	if count != 1 {
		t.Fatalf("ListTargetsAll consumed %d targets after break, want 1", count)
	}
}

func TestCdpHelpers_NewTab_Bad_InvalidDebugURL(t *testing.T) {
	server := newFakeCDPServer(t)
	client := newConnectedCDPClient(t, server.primaryTarget())

	// Replace the validated debug URL with one that no longer resolves, so
	// the createTargetAt call fails.
	client.debugHTTPURL = &cdpURL{Scheme: "http", Host: "127.0.0.1:1", hostname: "127.0.0.1", port: "1"}

	if _, err := client.NewTab("about:blank"); err == nil {
		t.Fatal("NewTab succeeded against an unreachable debug endpoint")
	}
}

func TestCdpHelpers_sameEndpointHost_Bad_InvalidInputs(t *testing.T) {
	valid := &cdpURL{hostname: "localhost", port: "9222"}

	if sameEndpointHost(42, valid) {
		t.Fatal("sameEndpointHost returned true for an invalid HTTP endpoint")
	}
	if sameEndpointHost(valid, 42) {
		t.Fatal("sameEndpointHost returned true for an invalid WebSocket endpoint")
	}
}

func TestCdpHelpers_sameEndpointHost_Good_MatchingHosts(t *testing.T) {
	http := &cdpURL{hostname: "localhost", port: "9222"}
	ws := &cdpURL{hostname: "LOCALHOST", port: "9222"}
	if !sameEndpointHost(http, ws) {
		t.Fatal("sameEndpointHost returned false for case-insensitive matching hosts")
	}
}

func TestCdpHelpers_validateTargetWebSocketURL_Bad_InvalidDebugURL(t *testing.T) {
	if _, err := validateTargetWebSocketURL(42, "ws://localhost:9222/devtools/page/1"); err == nil {
		t.Fatal("validateTargetWebSocketURL accepted an invalid debug endpoint")
	}
}
