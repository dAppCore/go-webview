// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"io"
	"iter"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	core "dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"github.com/gorilla/websocket"
)

const debugEndpointTimeout = 10 * time.Second

var (
	defaultDebugHTTPClient = &http.Client{
		Timeout: debugEndpointTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	errCDPClientClosed = core.NewError("cdp client closed")
)

// CDPClient handles communication with Chrome DevTools Protocol via WebSocket.
type CDPClient struct {
	mu        sync.RWMutex
	conn      *websocket.Conn
	debugURL  string
	debugBase *url.URL
	wsURL     string

	// Message tracking
	msgID   atomic.Int64
	pending map[int64]chan *cdpResponse
	pendMu  sync.Mutex

	// Event handlers
	handlers map[string][]func(map[string]any)
	handMu   sync.RWMutex

	// Lifecycle
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// cdpMessage represents a CDP protocol message.
type cdpMessage struct {
	ID     int64          `json:"id,omitempty"`
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

// cdpResponse represents a CDP protocol response.
type cdpResponse struct {
	ID     int64          `json:"id"`
	Result map[string]any `json:"result,omitempty"`
	Error  *cdpError      `json:"error,omitempty"`
}

// cdpEvent represents a CDP event.
type cdpEvent struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

// cdpError represents a CDP error.
type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// TargetInfo represents Chrome DevTools target information.
type TargetInfo struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// NewCDPClient creates a new CDP client connected to the given debug URL.
// The debug URL should be the Chrome DevTools HTTP endpoint (e.g., http://localhost:9222).
func NewCDPClient(debugURL string) (*CDPClient, error) {
	debugBase, err := parseDebugURL(debugURL)
	if err != nil {
		return nil, coreerr.E("CDPClient.New", "invalid debug URL", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), debugEndpointTimeout)
	defer cancel()

	targets, err := listTargetsAt(ctx, debugBase)
	if err != nil {
		return nil, coreerr.E("CDPClient.New", "failed to get targets", err)
	}

	// Find a page target
	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			wsURL, err = validateTargetWebSocketURL(debugBase, t.WebSocketDebuggerURL)
			if err != nil {
				return nil, coreerr.E("CDPClient.New", "invalid target WebSocket URL", err)
			}
			break
		}
	}

	if wsURL == "" {
		newTarget, err := createTargetAt(ctx, debugBase, "")
		if err != nil {
			return nil, coreerr.E("CDPClient.New", "no page targets found and failed to create new", err)
		}

		wsURL, err = validateTargetWebSocketURL(debugBase, newTarget.WebSocketDebuggerURL)
		if err != nil {
			return nil, coreerr.E("CDPClient.New", "invalid new target WebSocket URL", err)
		}
	}

	if wsURL == "" {
		return nil, coreerr.E("CDPClient.New", "no WebSocket URL available", nil)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, coreerr.E("CDPClient.New", "failed to connect to WebSocket", err)
	}

	return newCDPClient(debugBase, wsURL, conn), nil
}

// Close closes the CDP connection.
func (c *CDPClient) Close() error {
	c.close(errCDPClientClosed)
	<-c.done
	if c.closeErr != nil {
		return coreerr.E("CDPClient.Close", "failed to close WebSocket", c.closeErr)
	}
	return nil
}

// Call sends a CDP method call and waits for the response.
func (c *CDPClient) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	id := c.msgID.Add(1)

	msg := cdpMessage{
		ID:     id,
		Method: method,
		Params: cloneMapAny(params),
	}

	// Register response channel
	respCh := make(chan *cdpResponse, 1)
	c.pendMu.Lock()
	c.pending[id] = respCh
	c.pendMu.Unlock()

	defer func() {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
	}()

	// Send message
	c.mu.Lock()
	err := c.conn.WriteJSON(msg)
	c.mu.Unlock()
	if err != nil {
		return nil, coreerr.E("CDPClient.Call", "failed to send message", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, coreerr.E("CDPClient.Call", "client closed", errCDPClientClosed)
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, coreerr.E("CDPClient.Call", resp.Error.Message, nil)
		}
		return resp.Result, nil
	}
}

// OnEvent registers a handler for CDP events.
func (c *CDPClient) OnEvent(method string, handler func(map[string]any)) {
	c.handMu.Lock()
	defer c.handMu.Unlock()
	c.handlers[method] = append(c.handlers[method], handler)
}

// readLoop reads messages from the WebSocket connection.
func (c *CDPClient) readLoop() {
	defer close(c.done)

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if c.ctx.Err() != nil {
				return
			}
			if isTerminalReadError(err) {
				c.close(err)
				return
			}

			var netErr net.Error
			if core.As(err, &netErr) && netErr.Timeout() {
				continue
			}

			c.close(err)
			return
		}

		// Try to parse as response
		var resp cdpResponse
		if r := core.JSONUnmarshal(data, &resp); r.OK && resp.ID > 0 {
			c.pendMu.Lock()
			if ch, ok := c.pending[resp.ID]; ok {
				respCopy := resp
				select {
				case ch <- &respCopy:
				default:
				}
			}
			c.pendMu.Unlock()
			continue
		}

		// Try to parse as event
		var event cdpEvent
		if r := core.JSONUnmarshal(data, &event); r.OK && event.Method != "" {
			c.dispatchEvent(event.Method, event.Params)
		}
	}
}

// dispatchEvent dispatches an event to registered handlers.
func (c *CDPClient) dispatchEvent(method string, params map[string]any) {
	c.handMu.RLock()
	handlers := slices.Clone(c.handlers[method])
	c.handMu.RUnlock()

	for _, handler := range handlers {
		// Call handler in goroutine to avoid blocking
		handlerParams := cloneMapAny(params)
		go handler(handlerParams)
	}
}

// Send sends a fire-and-forget CDP message (no response expected).
func (c *CDPClient) Send(method string, params map[string]any) error {
	msg := cdpMessage{
		Method: method,
		Params: cloneMapAny(params),
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(msg)
}

// DebugURL returns the debug HTTP URL.
func (c *CDPClient) DebugURL() string {
	return c.debugURL
}

// WebSocketURL returns the WebSocket URL being used.
func (c *CDPClient) WebSocketURL() string {
	return c.wsURL
}

// NewTab creates a new browser tab and returns a new CDPClient connected to it.
func (c *CDPClient) NewTab(url string) (*CDPClient, error) {
	ctx, cancel := context.WithTimeout(c.ctx, debugEndpointTimeout)
	defer cancel()

	target, err := createTargetAt(ctx, c.debugBase, url)
	if err != nil {
		return nil, coreerr.E("CDPClient.NewTab", "failed to create new tab", err)
	}

	if target.WebSocketDebuggerURL == "" {
		return nil, coreerr.E("CDPClient.NewTab", "no WebSocket URL for new tab", nil)
	}

	wsURL, err := validateTargetWebSocketURL(c.debugBase, target.WebSocketDebuggerURL)
	if err != nil {
		return nil, coreerr.E("CDPClient.NewTab", "invalid WebSocket URL for new tab", err)
	}

	// Connect to new tab
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, coreerr.E("CDPClient.NewTab", "failed to connect to new tab", err)
	}

	return newCDPClient(c.debugBase, wsURL, conn), nil
}

// CloseTab closes the current tab (target).
func (c *CDPClient) CloseTab() error {
	targetID, err := targetIDFromWebSocketURL(c.wsURL)
	if err != nil {
		return coreerr.E("CDPClient.CloseTab", "failed to determine target ID", err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, debugEndpointTimeout)
	defer cancel()

	result, err := c.Call(ctx, "Target.closeTarget", map[string]any{
		"targetId": targetID,
	})
	if err != nil {
		return coreerr.E("CDPClient.CloseTab", "failed to close target", err)
	}

	if success, ok := result["success"].(bool); ok && !success {
		return coreerr.E("CDPClient.CloseTab", "target close was not acknowledged", nil)
	}

	return c.Close()
}

// ListTargets returns all available targets.
func ListTargets(debugURL string) ([]TargetInfo, error) {
	debugBase, err := parseDebugURL(debugURL)
	if err != nil {
		return nil, coreerr.E("ListTargets", "invalid debug URL", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), debugEndpointTimeout)
	defer cancel()

	targets, err := listTargetsAt(ctx, debugBase)
	if err != nil {
		return nil, coreerr.E("ListTargets", "failed to get targets", err)
	}

	return targets, nil
}

// ListTargetsAll returns an iterator over all available targets.
func ListTargetsAll(debugURL string) iter.Seq[TargetInfo] {
	return func(yield func(TargetInfo) bool) {
		targets, err := ListTargets(debugURL)
		if err != nil {
			return
		}
		for _, t := range targets {
			if !yield(t) {
				return
			}
		}
	}
}

// GetVersion returns Chrome version information.
func GetVersion(debugURL string) (map[string]string, error) {
	debugBase, err := parseDebugURL(debugURL)
	if err != nil {
		return nil, coreerr.E("GetVersion", "invalid debug URL", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), debugEndpointTimeout)
	defer cancel()

	body, err := doDebugRequest(ctx, debugBase, "/json/version", "")
	if err != nil {
		return nil, coreerr.E("GetVersion", "failed to get version", err)
	}

	var version map[string]string
	if r := core.JSONUnmarshal(body, &version); !r.OK {
		return nil, coreerr.E("GetVersion", "failed to parse version", nil)
	}

	return version, nil
}

func newCDPClient(debugBase *url.URL, wsURL string, conn *websocket.Conn) *CDPClient {
	ctx, cancel := context.WithCancel(context.Background())
	baseCopy := *debugBase

	client := &CDPClient{
		conn:      conn,
		debugURL:  canonicalDebugURL(&baseCopy),
		debugBase: &baseCopy,
		wsURL:     wsURL,
		pending:   make(map[int64]chan *cdpResponse),
		handlers:  make(map[string][]func(map[string]any)),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	go client.readLoop()

	return client
}

func parseDebugURL(raw string) (*url.URL, error) {
	debugURL, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if debugURL.Scheme != "http" && debugURL.Scheme != "https" {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL must use http or https", nil)
	}
	if debugURL.Host == "" {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL host is required", nil)
	}
	if debugURL.User != nil {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL must not include credentials", nil)
	}
	if debugURL.RawQuery != "" || debugURL.Fragment != "" {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL must not include query or fragment", nil)
	}
	if debugURL.Path == "" {
		debugURL.Path = "/"
	}
	if debugURL.Path != "/" {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL must point at the DevTools root", nil)
	}
	return debugURL, nil
}

func canonicalDebugURL(debugURL *url.URL) string {
	return core.TrimSuffix(debugURL.String(), "/")
}

func doDebugRequest(ctx context.Context, debugBase *url.URL, endpoint, rawQuery string) ([]byte, error) {
	reqURL := *debugBase
	reqURL.Path = endpoint
	reqURL.RawPath = ""
	reqURL.RawQuery = rawQuery
	reqURL.Fragment = ""

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := defaultDebugHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, coreerr.E("CDPClient.doDebugRequest", "debug endpoint returned "+resp.Status, nil)
	}

	return body, nil
}

func listTargetsAt(ctx context.Context, debugBase *url.URL) ([]TargetInfo, error) {
	body, err := doDebugRequest(ctx, debugBase, "/json", "")
	if err != nil {
		return nil, err
	}

	var targets []TargetInfo
	if r := core.JSONUnmarshal(body, &targets); !r.OK {
		return nil, coreerr.E("CDPClient.listTargetsAt", "failed to parse targets", nil)
	}

	return targets, nil
}

func createTargetAt(ctx context.Context, debugBase *url.URL, pageURL string) (*TargetInfo, error) {
	rawQuery := ""
	if pageURL != "" {
		rawQuery = url.QueryEscape(pageURL)
	}

	body, err := doDebugRequest(ctx, debugBase, "/json/new", rawQuery)
	if err != nil {
		return nil, err
	}

	var target TargetInfo
	if r := core.JSONUnmarshal(body, &target); !r.OK {
		return nil, coreerr.E("CDPClient.createTargetAt", "failed to parse target", nil)
	}

	return &target, nil
}

func validateTargetWebSocketURL(debugBase *url.URL, raw string) (string, error) {
	wsURL, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if wsURL.Scheme != "ws" && wsURL.Scheme != "wss" {
		return "", coreerr.E("CDPClient.validateTargetWebSocketURL", "target WebSocket URL must use ws or wss", nil)
	}
	if !sameEndpointHost(debugBase, wsURL) {
		return "", coreerr.E("CDPClient.validateTargetWebSocketURL", "target WebSocket URL must match debug URL host", nil)
	}
	return wsURL.String(), nil
}

func sameEndpointHost(httpURL, wsURL *url.URL) bool {
	return strings.EqualFold(httpURL.Hostname(), wsURL.Hostname()) && normalisedPort(httpURL) == normalisedPort(wsURL)
}

func normalisedPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}

	switch u.Scheme {
	case "http", "ws":
		return "80"
	case "https", "wss":
		return "443"
	default:
		return ""
	}
}

func targetIDFromWebSocketURL(raw string) (string, error) {
	wsURL, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	targetID := path.Base(core.TrimSuffix(wsURL.Path, "/"))
	if targetID == "." || targetID == "/" || targetID == "" {
		return "", coreerr.E("CDPClient.targetIDFromWebSocketURL", "missing target ID in WebSocket URL", nil)
	}

	return targetID, nil
}

func (c *CDPClient) close(reason error) {
	c.closeOnce.Do(func() {
		c.cancel()
		c.failPending(reason)

		c.mu.Lock()
		err := c.conn.Close()
		c.mu.Unlock()
		if err != nil && !isTerminalReadError(err) {
			c.closeErr = err
		}
	})
}

func (c *CDPClient) failPending(err error) {
	c.pendMu.Lock()
	defer c.pendMu.Unlock()

	for id, ch := range c.pending {
		resp := &cdpResponse{
			ID: id,
			Error: &cdpError{
				Message: err.Error(),
			},
		}
		select {
		case ch <- resp:
		default:
		}
	}
}

func isTerminalReadError(err error) bool {
	if err == nil {
		return false
	}
	if core.Is(err, net.ErrClosed) || core.Is(err, websocket.ErrCloseSent) {
		return true
	}
	var closeErr *websocket.CloseError
	return core.As(err, &closeErr)
}

func cloneMapAny(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = cloneAny(value)
	}
	return dst
}

func cloneSliceAny(src []any) []any {
	if src == nil {
		return nil
	}

	dst := make([]any, len(src))
	for i, value := range src {
		dst[i] = cloneAny(value)
	}
	return dst
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMapAny(typed)
	case []any:
		return cloneSliceAny(typed)
	default:
		return typed
	}
}
