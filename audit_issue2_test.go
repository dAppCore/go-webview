// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	core "dappco.re/go/core"

	"github.com/gorilla/websocket"
)

type fakeCDPServer struct {
	t          *testing.T
	server     *httptest.Server
	mu         sync.Mutex
	nextTarget int
	targets    map[string]*fakeCDPTarget
}

type fakeCDPTarget struct {
	server        *fakeCDPServer
	id            string
	onConnect     func(*fakeCDPTarget)
	onMessage     func(*fakeCDPTarget, cdpMessage)
	connMu        sync.Mutex
	conn          *websocket.Conn
	received      chan cdpMessage
	connected     chan struct{}
	closed        chan struct{}
	connectedOnce sync.Once
	closedOnce    sync.Once
}

func newFakeCDPServer(t *testing.T) *fakeCDPServer {
	t.Helper()

	server := &fakeCDPServer{
		t:       t,
		targets: make(map[string]*fakeCDPTarget),
	}
	server.server = httptest.NewServer(http.HandlerFunc(server.handle))
	server.addTarget("target-1")
	t.Cleanup(server.Close)

	return server
}

func (s *fakeCDPServer) Close() {
	s.server.Close()
}

func (s *fakeCDPServer) DebugURL() string {
	return s.server.URL
}

func (s *fakeCDPServer) addTarget(id string) *fakeCDPTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	target := &fakeCDPTarget{
		server:    s,
		id:        id,
		received:  make(chan cdpMessage, 16),
		connected: make(chan struct{}),
		closed:    make(chan struct{}),
	}
	s.targets[id] = target
	return target
}

func (s *fakeCDPServer) newTarget() *fakeCDPTarget {
	s.mu.Lock()
	s.nextTarget++
	id := core.Sprintf("target-%d", s.nextTarget+1)
	s.mu.Unlock()

	return s.addTarget(id)
}

func (s *fakeCDPServer) primaryTarget() *fakeCDPTarget {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.targets["target-1"]
}

func (s *fakeCDPServer) handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/json":
		s.handleListTargets(w)
	case r.URL.Path == "/json/new":
		s.handleNewTarget(w)
	case r.URL.Path == "/json/version":
		s.writeJSON(w, map[string]string{
			"Browser": "Chrome/123.0",
		})
	case core.HasPrefix(r.URL.Path, "/devtools/page/"):
		s.handleWebSocket(w, r, core.TrimPrefix(r.URL.Path, "/devtools/page/"))
	default:
		http.NotFound(w, r)
	}
}

func (s *fakeCDPServer) handleListTargets(w http.ResponseWriter) {
	s.mu.Lock()
	targets := make([]TargetInfo, 0, len(s.targets))
	for id := range s.targets {
		targets = append(targets, TargetInfo{
			ID:                   id,
			Type:                 "page",
			Title:                id,
			URL:                  "about:blank",
			WebSocketDebuggerURL: s.webSocketURL(id),
		})
	}
	s.mu.Unlock()

	s.writeJSON(w, targets)
}

func (s *fakeCDPServer) handleNewTarget(w http.ResponseWriter) {
	target := s.newTarget()
	s.writeJSON(w, TargetInfo{
		ID:                   target.id,
		Type:                 "page",
		Title:                target.id,
		URL:                  "about:blank",
		WebSocketDebuggerURL: s.webSocketURL(target.id),
	})
}

func (s *fakeCDPServer) handleWebSocket(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	target := s.targets[id]
	s.mu.Unlock()
	if target == nil {
		http.NotFound(w, r)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.t.Fatalf("failed to upgrade test WebSocket: %v", err)
	}

	target.attach(conn)
}

func (s *fakeCDPServer) writeJSON(w http.ResponseWriter, value any) {
	s.t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		s.t.Fatalf("failed to encode JSON: %v", err)
	}
}

func (s *fakeCDPServer) webSocketURL(id string) string {
	wsURL, err := url.Parse(s.server.URL)
	if err != nil {
		s.t.Fatalf("failed to parse test server URL: %v", err)
	}
	if wsURL.Scheme == "http" {
		wsURL.Scheme = "ws"
	} else {
		wsURL.Scheme = "wss"
	}
	wsURL.Path = "/devtools/page/" + id
	wsURL.RawQuery = ""
	wsURL.Fragment = ""

	return wsURL.String()
}

func (tgt *fakeCDPTarget) attach(conn *websocket.Conn) {
	tgt.connMu.Lock()
	tgt.conn = conn
	tgt.connMu.Unlock()

	tgt.connectedOnce.Do(func() {
		close(tgt.connected)
	})

	go tgt.readLoop()

	if tgt.onConnect != nil {
		go tgt.onConnect(tgt)
	}
}

func (tgt *fakeCDPTarget) readLoop() {
	defer tgt.closedOnce.Do(func() {
		close(tgt.closed)
	})

	for {
		_, data, err := tgt.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg cdpMessage
		if r := core.JSONUnmarshal(data, &msg); !r.OK {
			continue
		}

		select {
		case tgt.received <- msg:
		default:
		}

		if tgt.onMessage != nil {
			tgt.onMessage(tgt, msg)
		}
	}
}

func (tgt *fakeCDPTarget) reply(id int64, result map[string]any) {
	tgt.writeJSON(cdpResponse{
		ID:     id,
		Result: result,
	})
}

func (tgt *fakeCDPTarget) replyError(id int64, message string) {
	tgt.writeJSON(cdpResponse{
		ID: id,
		Error: &cdpError{
			Message: message,
		},
	})
}

func (tgt *fakeCDPTarget) replyValue(id int64, value any) {
	tgt.reply(id, map[string]any{
		"result": map[string]any{
			"value": value,
		},
	})
}

func (tgt *fakeCDPTarget) writeJSON(value any) {
	tgt.server.t.Helper()

	tgt.connMu.Lock()
	defer tgt.connMu.Unlock()
	if tgt.conn == nil {
		tgt.server.t.Fatal("test WebSocket connection was not established")
	}
	if err := tgt.conn.WriteJSON(value); err != nil {
		tgt.server.t.Fatalf("failed to write test WebSocket message: %v", err)
	}
}

func (tgt *fakeCDPTarget) closeWebSocket() {
	tgt.connMu.Lock()
	defer tgt.connMu.Unlock()
	if tgt.conn != nil {
		_ = tgt.conn.Close()
	}
}

func (tgt *fakeCDPTarget) waitForMessage(tb testing.TB) cdpMessage {
	tb.Helper()

	select {
	case msg := <-tgt.received:
		return msg
	case <-time.After(time.Second):
		tb.Fatal("timed out waiting for CDP message")
		return cdpMessage{}
	}
}

func (tgt *fakeCDPTarget) waitConnected(tb testing.TB) {
	tb.Helper()

	select {
	case <-tgt.connected:
	case <-time.After(time.Second):
		tb.Fatal("timed out waiting for WebSocket connection")
	}
}

func (tgt *fakeCDPTarget) waitClosed(tb testing.TB) {
	tb.Helper()

	select {
	case <-tgt.closed:
	case <-time.After(time.Second):
		tb.Fatal("timed out waiting for WebSocket closure")
	}
}

func TestCDPClientClose_Good_UnblocksReadLoop(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}

	target.waitConnected(t)

	done := make(chan error, 1)
	go func() {
		done <- client.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close blocked waiting for readLoop")
	}
}

func TestCDPClientReadLoop_Ugly_StopsOnTerminalReadError(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onConnect = func(target *fakeCDPTarget) {
		target.closeWebSocket()
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}

	select {
	case <-client.done:
	case <-time.After(time.Second):
		t.Fatal("readLoop did not stop after terminal read error")
	}
}

func TestCDPClientCloseTab_Good_ClosesTargetOnly(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Target.closeTarget" {
			t.Fatalf("CloseTab sent %q, want Target.closeTarget", msg.Method)
		}
		if got := msg.Params["targetId"]; got != target.id {
			t.Fatalf("Target.closeTarget targetId = %v, want %q", got, target.id)
		}
		target.reply(msg.ID, map[string]any{"success": true})
		go func() {
			time.Sleep(10 * time.Millisecond)
			target.closeWebSocket()
		}()
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}

	if err := client.CloseTab(); err != nil {
		t.Fatalf("CloseTab returned error: %v", err)
	}

	msg := target.waitForMessage(t)
	if msg.Method == "Browser.close" {
		t.Fatal("CloseTab closed the whole browser")
	}
}

func TestCDPClientDispatchEvent_Good_HandlerParamsAreIsolated(t *testing.T) {
	client := &CDPClient{
		handlers: make(map[string][]func(map[string]any)),
	}

	firstDone := make(chan map[string]any, 1)
	secondDone := make(chan map[string]any, 1)

	client.OnEvent("Runtime.testEvent", func(params map[string]any) {
		params["value"] = "mutated"
		params["nested"].(map[string]any)["count"] = 1
		params["items"].([]any)[0].(map[string]any)["id"] = "changed"
		firstDone <- params
	})
	client.OnEvent("Runtime.testEvent", func(params map[string]any) {
		secondDone <- params
	})

	original := map[string]any{
		"nested": map[string]any{"count": 0},
		"items":  []any{map[string]any{"id": "original"}},
	}

	client.dispatchEvent("Runtime.testEvent", original)

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first handler did not run")
	}

	var secondParams map[string]any
	select {
	case secondParams = <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second handler did not run")
	}

	if _, ok := secondParams["value"]; ok {
		t.Fatal("second handler observed first handler mutation")
	}
	if got := secondParams["nested"].(map[string]any)["count"]; got != 0 {
		t.Fatalf("second handler nested count = %v, want 0", got)
	}
	if got := secondParams["items"].([]any)[0].(map[string]any)["id"]; got != "original" {
		t.Fatalf("second handler slice payload = %v, want %q", got, "original")
	}
	if got := original["nested"].(map[string]any)["count"]; got != 0 {
		t.Fatalf("original params were mutated: nested count = %v", got)
	}
}

func TestNewCDPClient_Bad_RejectsCrossHostWebSocket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]TargetInfo{{
			ID:                   "target-1",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://example.com/devtools/page/target-1",
		}}); err != nil {
			t.Fatalf("failed to encode targets: %v", err)
		}
	}))
	defer server.Close()

	_, err := NewCDPClient(server.URL)
	if err == nil {
		t.Fatal("NewCDPClient succeeded with a cross-host WebSocket URL")
	}
	if !core.Contains(err.Error(), "invalid target WebSocket URL") {
		t.Fatalf("NewCDPClient error = %v, want cross-host WebSocket validation failure", err)
	}
}

func TestWebviewNew_Bad_ClosesClientWhenEnableConsoleFails(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.enable" {
			t.Fatalf("enableConsole sent %q before Runtime.enable failed", msg.Method)
		}
		target.replyError(msg.ID, "runtime disabled")
	}

	_, err := New(
		WithTimeout(250*time.Millisecond),
		WithDebugURL(server.DebugURL()),
	)
	if err == nil {
		t.Fatal("New succeeded when Runtime.enable failed")
	}

	target.waitClosed(t)
}

func TestAngularHelperWaitForZoneStability_Good_AwaitsPromise(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}
	defer func() { _ = client.Close() }()

	wv := &Webview{
		client:  client,
		ctx:     context.Background(),
		timeout: time.Second,
	}
	ah := NewAngularHelper(wv)

	if err := ah.waitForZoneStability(context.Background()); err != nil {
		t.Fatalf("waitForZoneStability returned error: %v", err)
	}

	msg := target.waitForMessage(t)
	if got := msg.Params["awaitPromise"]; got != true {
		t.Fatalf("Runtime.evaluate awaitPromise = %v, want true", got)
	}
	if got := msg.Params["returnByValue"]; got != true {
		t.Fatalf("Runtime.evaluate returnByValue = %v, want true", got)
	}
}

func TestAngularHelperSetNgModel_Good_EscapesSelectorAndValue(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, true)
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}
	defer func() { _ = client.Close() }()

	wv := &Webview{
		client:  client,
		ctx:     context.Background(),
		timeout: time.Second,
	}
	ah := NewAngularHelper(wv)

	selector := `input[name="x'];window.hacked=true;//"]`
	value := `";window.hacked=true;//`
	if err := ah.SetNgModel(selector, value); err != nil {
		t.Fatalf("SetNgModel returned error: %v", err)
	}

	expression, _ := target.waitForMessage(t).Params["expression"].(string)
	if !core.Contains(expression, "const selector = "+formatJSValue(selector)+";") {
		t.Fatalf("expression did not contain safely quoted selector: %s", expression)
	}
	if !core.Contains(expression, "element.value = "+formatJSValue(value)+";") {
		t.Fatalf("expression did not contain safely quoted value: %s", expression)
	}
	if core.Contains(expression, "throw new Error('Element not found: "+selector+"')") {
		t.Fatalf("expression still embedded selector directly in error text: %s", expression)
	}
}

func TestConsoleWatcherWaitForMessage_Good_IsolatesTemporaryHandlers(t *testing.T) {
	cw := &ConsoleWatcher{
		messages: make([]ConsoleMessage, 0),
		filters:  make([]ConsoleFilter, 0),
		limit:    1000,
		handlers: make([]consoleHandlerRegistration, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	results := make(chan string, 2)
	errorsCh := make(chan error, 2)

	go func() {
		msg, err := cw.WaitForMessage(ctx, ConsoleFilter{Type: "error"})
		if err != nil {
			errorsCh <- err
			return
		}
		results <- "error:" + msg.Text
	}()
	go func() {
		msg, err := cw.WaitForMessage(ctx, ConsoleFilter{Type: "log"})
		if err != nil {
			errorsCh <- err
			return
		}
		results <- "log:" + msg.Text
	}()

	time.Sleep(20 * time.Millisecond)
	cw.addMessage(ConsoleMessage{Type: "error", Text: "first"})
	time.Sleep(20 * time.Millisecond)
	cw.addMessage(ConsoleMessage{Type: "log", Text: "second"})

	got := make(map[string]bool, 2)
	for range 2 {
		select {
		case err := <-errorsCh:
			t.Fatalf("WaitForMessage returned error: %v", err)
		case result := <-results:
			got[result] = true
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for console waiter results")
		}
	}

	if !got["error:first"] || !got["log:second"] {
		t.Fatalf("unexpected console waiter results: %#v", got)
	}
	if len(cw.handlers) != 0 {
		t.Fatalf("temporary handlers leaked: %d", len(cw.handlers))
	}
}

func TestExceptionWatcherWaitForException_Good_PreservesExistingHandlers(t *testing.T) {
	ew := &ExceptionWatcher{
		exceptions: make([]ExceptionInfo, 0),
		handlers:   make([]exceptionHandlerRegistration, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	waitDone := make(chan error, 1)
	go func() {
		_, err := ew.WaitForException(ctx)
		waitDone <- err
	}()

	time.Sleep(20 * time.Millisecond)

	var mu sync.Mutex
	count := 0
	ew.AddHandler(func(ExceptionInfo) {
		mu.Lock()
		defer mu.Unlock()
		count++
	})

	ew.handleException(map[string]any{
		"exceptionDetails": map[string]any{
			"text":         "first",
			"lineNumber":   float64(1),
			"columnNumber": float64(1),
			"url":          "https://example.com/app.js",
		},
	})

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("WaitForException returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for exception waiter")
	}

	ew.handleException(map[string]any{
		"exceptionDetails": map[string]any{
			"text":         "second",
			"lineNumber":   float64(2),
			"columnNumber": float64(1),
			"url":          "https://example.com/app.js",
		},
	})

	mu.Lock()
	defer mu.Unlock()
	if count != 2 {
		t.Fatalf("persistent handler count = %d, want 2", count)
	}
	if len(ew.handlers) != 1 {
		t.Fatalf("unexpected handler count after waiter removal: %d", len(ew.handlers))
	}
}

func TestWebviewGoBack_Good_UsesNavigationHistoryAndWaitsForLoad(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()

	var methods []string
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		methods = append(methods, msg.Method)

		switch msg.Method {
		case "Page.getNavigationHistory":
			target.reply(msg.ID, map[string]any{
				"currentIndex": float64(1),
				"entries": []any{
					map[string]any{"id": float64(11)},
					map[string]any{"id": float64(12)},
					map[string]any{"id": float64(13)},
				},
			})
		case "Page.navigateToHistoryEntry":
			if got, ok := msg.Params["entryId"].(float64); !ok || got != 11 {
				t.Fatalf("navigateToHistoryEntry entryId = %v, want 11", msg.Params["entryId"])
			}
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, "complete")
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}
	defer func() { _ = client.Close() }()

	wv := &Webview{
		client:  client,
		ctx:     context.Background(),
		timeout: time.Second,
	}

	if err := wv.GoBack(); err != nil {
		t.Fatalf("GoBack returned error: %v", err)
	}

	if len(methods) != 3 {
		t.Fatalf("expected 3 CDP calls, got %d (%v)", len(methods), methods)
	}
	if methods[0] != "Page.getNavigationHistory" || methods[1] != "Page.navigateToHistoryEntry" || methods[2] != "Runtime.evaluate" {
		t.Fatalf("unexpected call sequence: %v", methods)
	}
}

func TestWebviewGoForward_Good_UsesNavigationHistoryAndWaitsForLoad(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "Page.getNavigationHistory":
			target.reply(msg.ID, map[string]any{
				"currentIndex": float64(1),
				"entries": []any{
					map[string]any{"id": float64(11)},
					map[string]any{"id": float64(12)},
					map[string]any{"id": float64(13)},
				},
			})
		case "Page.navigateToHistoryEntry":
			if got, ok := msg.Params["entryId"].(float64); !ok || got != 13 {
				t.Fatalf("navigateToHistoryEntry entryId = %v, want 13", msg.Params["entryId"])
			}
			target.reply(msg.ID, map[string]any{})
		case "Runtime.evaluate":
			target.replyValue(msg.ID, "complete")
		default:
			t.Fatalf("unexpected method %q", msg.Method)
		}
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}
	defer func() { _ = client.Close() }()

	wv := &Webview{
		client:  client,
		ctx:     context.Background(),
		timeout: time.Second,
	}

	if err := wv.GoForward(); err != nil {
		t.Fatalf("GoForward returned error: %v", err)
	}
}

func TestWebviewEvaluate_Bad_UsesExceptionText(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.writeJSON(cdpResponse{
			ID: msg.ID,
			Result: map[string]any{
				"exceptionDetails": map[string]any{
					"text": "ReferenceError: missingValue is not defined",
				},
			},
		})
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}
	defer func() { _ = client.Close() }()

	wv := &Webview{
		client:  client,
		ctx:     context.Background(),
		timeout: time.Second,
	}

	if _, err := wv.Evaluate("missingValue"); err == nil || !core.Contains(err.Error(), "ReferenceError: missingValue is not defined") {
		t.Fatalf("Evaluate error = %v, want exception text", err)
	}
}

func TestAngularHelperGetRouterState_Good_KeepsOnlyStringParams(t *testing.T) {
	server := newFakeCDPServer(t)
	target := server.primaryTarget()
	target.onMessage = func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Fatalf("unexpected method %q", msg.Method)
		}
		target.replyValue(msg.ID, map[string]any{
			"url":      "/items/123",
			"fragment": "details",
			"params": map[string]any{
				"id":     "123",
				"active": true,
			},
			"queryParams": map[string]any{
				"page":  "2",
				"debug": float64(1),
			},
		})
	}

	client, err := NewCDPClient(server.DebugURL())
	if err != nil {
		t.Fatalf("NewCDPClient returned error: %v", err)
	}
	defer func() { _ = client.Close() }()

	wv := &Webview{
		client:  client,
		ctx:     context.Background(),
		timeout: time.Second,
	}
	ah := NewAngularHelper(wv)

	state, err := ah.GetRouterState()
	if err != nil {
		t.Fatalf("GetRouterState returned error: %v", err)
	}
	if state.Params["id"] != "123" {
		t.Fatalf("unexpected params: %#v", state.Params)
	}
	if _, ok := state.Params["active"]; ok {
		t.Fatalf("expected non-string params to be omitted, got %#v", state.Params)
	}
	if state.QueryParams["page"] != "2" {
		t.Fatalf("unexpected query params: %#v", state.QueryParams)
	}
	if _, ok := state.QueryParams["debug"]; ok {
		t.Fatalf("expected non-string query params to be omitted, got %#v", state.QueryParams)
	}
}
