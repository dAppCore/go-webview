// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"io"       // Note: AX-6 intrinsic — bounded streaming read from CDP HTTP response bodies; coreio.Medium does not model transient HTTP bodies.
	"iter"     // Note: intrinsic — stdlib iterator primitive for Seq[TargetInfo] return type
	"net"      // Note: AX-6 intrinsic — loopback IP parsing and terminal network error sentinels; no Core Process context is available here.
	"net/http" // Note: AX-6 intrinsic — CDP DevTools discovery is an HTTP boundary (/json, /json/new, /json/version); no core HTTP fetch primitive yet.
	"reflect"
	"slices"      // Note: intrinsic — slices.Clone duplicates event handler slice under RLock before dispatch
	"sync"        // Note: AX-6 — internal concurrency primitive; structural per RFC §3/§6
	"sync/atomic" // Note: AX-6 — internal concurrency primitive; structural per RFC §3/§6
	"time"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"

	"github.com/gorilla/websocket"
)

const debugEndpointTimeout = 10 * time.Second
const maxDebugResponseBytes = 1 << 20
const maxCDPMessageBytes = 16 << 20

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
	mu           sync.RWMutex
	conn         *websocket.Conn
	debugURL     string
	debugHTTPURL *cdpURL
	wsURL        string

	// Message tracking
	messageID atomic.Int64
	pending   map[int64]chan *cdpResponse
	pendingMu sync.Mutex

	// Event handlers
	handlers   map[string][]func(map[string]any)
	handlersMu sync.RWMutex

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

type coreParsedURL interface {
	String() string
	Hostname() string
	Port() string
}

type cdpURL struct {
	Scheme   string
	Host     string
	Path     string
	RawPath  string
	RawQuery string
	Fragment string

	hasUser  bool
	hostname string
	port     string
}

func (u *cdpURL) String() string {
	if u == nil {
		return ""
	}

	rawPath := u.Path
	if u.RawPath != "" {
		rawPath = u.RawPath
	}

	out := ""
	if u.Scheme != "" {
		out += u.Scheme + ":"
	}
	if u.Host != "" {
		out += "//" + u.Host
	}
	out += rawPath
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}

func (u *cdpURL) Hostname() string {
	if u == nil {
		return ""
	}
	return u.hostname
}

func (u *cdpURL) Port() string {
	if u == nil {
		return ""
	}
	return u.port
}

// NewCDPClient creates a new CDP client connected to the given debug URL.
// The debug URL should be the Chrome DevTools HTTP endpoint (e.g., http://localhost:9222).
func NewCDPClient(debugURL string) (*CDPClient, error) /* core.Result boundary */ {
	debugHTTPURL, err := parseDebugURL(debugURL)
	if err != nil {
		return nil, coreerr.E("CDPClient.New", "invalid debug URL", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), debugEndpointTimeout)
	defer cancel()

	targets, err := listTargetsAt(ctx, debugHTTPURL)
	if err != nil {
		return nil, coreerr.E("CDPClient.New", "failed to get targets", err)
	}

	// Find a page target
	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			wsURL, err = validateTargetWebSocketURL(debugHTTPURL, t.WebSocketDebuggerURL)
			if err != nil {
				return nil, coreerr.E("CDPClient.New", "invalid target WebSocket URL", err)
			}
			break
		}
	}

	if wsURL == "" {
		newTarget, err := createTargetAt(ctx, debugHTTPURL, "")
		if err != nil {
			return nil, coreerr.E("CDPClient.New", "no page targets found and failed to create new", err)
		}

		wsURL, err = validateTargetWebSocketURL(debugHTTPURL, newTarget.WebSocketDebuggerURL)
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
	conn.SetReadLimit(maxCDPMessageBytes)

	return newCDPClient(debugHTTPURL, wsURL, conn), nil
}

// Close closes the CDP connection.
func (c *CDPClient) Close() error /* core.Result boundary */ {
	c.close(errCDPClientClosed)
	<-c.done
	if c.closeErr != nil {
		return coreerr.E("CDPClient.Close", "failed to close WebSocket", c.closeErr)
	}
	return nil
}

// Call sends a CDP method call and waits for the response.
func (c *CDPClient) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) /* core.Result boundary */ {
	id := c.messageID.Add(1)

	msg := cdpMessage{
		ID:     id,
		Method: method,
		Params: cloneMapAny(params),
	}

	// Register response channel
	respCh := make(chan *cdpResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
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
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
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
			c.pendingMu.Lock()
			if ch, ok := c.pending[resp.ID]; ok {
				respCopy := resp
				select {
				case ch <- &respCopy:
				default:
				}
			}
			c.pendingMu.Unlock()
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
	c.handlersMu.RLock()
	handlers := slices.Clone(c.handlers[method])
	c.handlersMu.RUnlock()

	for _, handler := range handlers {
		// Call handler in goroutine to avoid blocking
		handlerParams := cloneMapAny(params)
		go handler(handlerParams)
	}
}

// Send sends a fire-and-forget CDP message (no response expected).
func (c *CDPClient) Send(method string, params map[string]any) error /* core.Result boundary */ {
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
func (c *CDPClient) NewTab(url string) (*CDPClient, error) /* core.Result boundary */ {
	ctx, cancel := context.WithTimeout(c.ctx, debugEndpointTimeout)
	defer cancel()

	target, err := createTargetAt(ctx, c.debugHTTPURL, url)
	if err != nil {
		return nil, coreerr.E("CDPClient.NewTab", "failed to create new tab", err)
	}

	if target.WebSocketDebuggerURL == "" {
		return nil, coreerr.E("CDPClient.NewTab", "no WebSocket URL for new tab", nil)
	}

	wsURL, err := validateTargetWebSocketURL(c.debugHTTPURL, target.WebSocketDebuggerURL)
	if err != nil {
		return nil, coreerr.E("CDPClient.NewTab", "invalid WebSocket URL for new tab", err)
	}

	// Connect to new tab
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, coreerr.E("CDPClient.NewTab", "failed to connect to new tab", err)
	}

	return newCDPClient(c.debugHTTPURL, wsURL, conn), nil
}

// CloseTab closes the current tab (target).
func (c *CDPClient) CloseTab() (err error) /* core.Result boundary */ {
	targetID, err := targetIDFromWebSocketURL(c.wsURL)
	if err != nil {
		return coreerr.E("CDPClient.CloseTab", "failed to determine target ID", err)
	}
	defer func() {
		if closeErr := c.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

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
	return nil
}

// ListTargets returns all available targets.
func ListTargets(debugURL string) ([]TargetInfo, error) /* core.Result boundary */ {
	debugHTTPURL, err := parseDebugURL(debugURL)
	if err != nil {
		return nil, coreerr.E("ListTargets", "invalid debug URL", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), debugEndpointTimeout)
	defer cancel()

	targets, err := listTargetsAt(ctx, debugHTTPURL)
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
func GetVersion(debugURL string) (map[string]string, error) /* core.Result boundary */ {
	debugHTTPURL, err := parseDebugURL(debugURL)
	if err != nil {
		return nil, coreerr.E("GetVersion", "invalid debug URL", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), debugEndpointTimeout)
	defer cancel()

	body, err := doDebugRequest(ctx, debugHTTPURL, "/json/version", "")
	if err != nil {
		return nil, coreerr.E("GetVersion", "failed to get version", err)
	}

	var version map[string]string
	if r := core.JSONUnmarshal(body, &version); !r.OK {
		return nil, coreerr.E("GetVersion", "failed to parse version", nil)
	}

	return version, nil
}

func newCDPClient(debugHTTPURL *cdpURL, wsURL string, conn *websocket.Conn) *CDPClient {
	ctx, cancel := context.WithCancel(context.Background())
	baseCopy := *debugHTTPURL
	conn.SetReadLimit(maxCDPMessageBytes)

	client := &CDPClient{
		conn:         conn,
		debugURL:     canonicalDebugURL(&baseCopy),
		debugHTTPURL: &baseCopy,
		wsURL:        wsURL,
		pending:      make(map[int64]chan *cdpResponse),
		handlers:     make(map[string][]func(map[string]any)),
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
	}

	go client.readLoop()

	return client
}

func parseDebugURL(raw string) (*cdpURL, error) /* core.Result boundary */ {
	debugURL, err := parseCoreURL(raw)
	if err != nil {
		return nil, err
	}
	if debugURL.Scheme != "http" && debugURL.Scheme != "https" {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL must use http or https", nil)
	}
	if debugURL.Host == "" {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL host is required", nil)
	}
	if debugURL.hasUser {
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
	if !isLoopbackHost(debugURL.Hostname()) {
		return nil, coreerr.E("CDPClient.parseDebugURL", "debug URL host must be localhost or loopback", nil)
	}
	return debugURL, nil
}

func parseCoreURL(raw string) (*cdpURL, error) /* core.Result boundary */ {
	r := core.URLParse(raw)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return nil, err
		}
		return nil, coreerr.E("CDPClient.parseCoreURL", "failed to parse URL", nil)
	}
	return cdpURLFromParsed(r.Value)
}

func cdpURLFromAny(value any) (*cdpURL, error) /* core.Result boundary */ {
	switch v := value.(type) {
	case *cdpURL:
		if v == nil {
			return nil, coreerr.E("CDPClient.cdpURLFromAny", "nil URL", nil)
		}
		return v, nil
	case cdpURL:
		return &v, nil
	default:
		return cdpURLFromParsed(value)
	}
}

func cdpURLFromParsed(value any) (*cdpURL, error) /* core.Result boundary */ {
	parsed, ok := value.(coreParsedURL)
	if !ok {
		return nil, coreerr.E("CDPClient.cdpURLFromParsed", "unsupported parsed URL type", nil)
	}

	return &cdpURL{
		Scheme:   urlFieldString(value, "Scheme"),
		Host:     urlFieldString(value, "Host"),
		Path:     urlFieldString(value, "Path"),
		RawPath:  urlFieldString(value, "RawPath"),
		RawQuery: urlFieldString(value, "RawQuery"),
		Fragment: urlFieldString(value, "Fragment"),
		hasUser:  urlFieldIsSet(value, "User"),
		hostname: parsed.Hostname(),
		port:     parsed.Port(),
	}, nil
}

func urlFieldString(value any, name string) string {
	field := urlField(value, name)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func urlFieldIsSet(value any, name string) bool {
	field := urlField(value, name)
	if !field.IsValid() {
		return false
	}

	switch field.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !field.IsNil()
	default:
		return !field.IsZero()
	}
}

func urlField(value any, name string) reflect.Value {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return reflect.Value{}
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	return v.FieldByName(name)
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if core.Lower(host) == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func canonicalDebugURL(debugURL any) string {
	u, err := cdpURLFromAny(debugURL)
	if err != nil {
		return ""
	}
	return core.TrimSuffix(u.String(), "/")
}

func doDebugRequest(ctx context.Context, debugHTTPURL any, endpoint, rawQuery string) (body []byte, err error) /* core.Result boundary */ {
	baseURL, err := cdpURLFromAny(debugHTTPURL)
	if err != nil {
		return nil, err
	}

	reqURL := *baseURL
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, coreerr.E("CDPClient.doDebugRequest", "debug endpoint returned "+resp.Status, nil)
	}

	body, err = io.ReadAll(io.LimitReader(resp.Body, maxDebugResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDebugResponseBytes {
		return nil, coreerr.E("CDPClient.doDebugRequest", "debug endpoint response too large", nil)
	}

	return body, nil
}

func listTargetsAt(ctx context.Context, debugHTTPURL any) ([]TargetInfo, error) /* core.Result boundary */ {
	body, err := doDebugRequest(ctx, debugHTTPURL, "/json", "")
	if err != nil {
		return nil, err
	}

	var targets []TargetInfo
	if r := core.JSONUnmarshal(body, &targets); !r.OK {
		return nil, coreerr.E("CDPClient.listTargetsAt", "failed to parse targets", nil)
	}

	return targets, nil
}

func createTargetAt(ctx context.Context, debugHTTPURL any, pageURL string) (*TargetInfo, error) /* core.Result boundary */ {
	if pageURL != "" {
		if err := validateNavigationURL(pageURL); err != nil {
			return nil, coreerr.E("CDPClient.createTargetAt", "invalid page URL", err)
		}
	}

	rawQuery := ""
	if pageURL != "" {
		rawQuery = core.URLEncode(pageURL)
	}

	body, err := doDebugRequest(ctx, debugHTTPURL, "/json/new", rawQuery)
	if err != nil {
		return nil, err
	}

	var target TargetInfo
	if r := core.JSONUnmarshal(body, &target); !r.OK {
		return nil, coreerr.E("CDPClient.createTargetAt", "failed to parse target", nil)
	}

	return &target, nil
}

func validateTargetWebSocketURL(debugHTTPURL any, raw string) (string, error) /* core.Result boundary */ {
	debugURL, err := cdpURLFromAny(debugHTTPURL)
	if err != nil {
		return "", err
	}

	wsURL, err := parseCoreURL(raw)
	if err != nil {
		return "", err
	}
	if wsURL.Scheme != "ws" && wsURL.Scheme != "wss" {
		return "", coreerr.E("CDPClient.validateTargetWebSocketURL", "target WebSocket URL must use ws or wss", nil)
	}
	if !sameEndpointHost(debugURL, wsURL) {
		return "", coreerr.E("CDPClient.validateTargetWebSocketURL", "target WebSocket URL must match debug URL host", nil)
	}
	return wsURL.String(), nil
}

func validateNavigationURL(raw string) error /* core.Result boundary */ {
	navigationURL, err := parseCoreURL(raw)
	if err != nil {
		return err
	}

	switch core.Lower(navigationURL.Scheme) {
	case "http", "https":
		if navigationURL.Host == "" {
			return coreerr.E("CDPClient.validateNavigationURL", "navigation URL host is required", nil)
		}
		if navigationURL.hasUser {
			return coreerr.E("CDPClient.validateNavigationURL", "navigation URL must not include credentials", nil)
		}
		return nil
	case "about":
		if raw == "about:blank" {
			return nil
		}
		return coreerr.E("CDPClient.validateNavigationURL", "only about:blank is permitted for non-http navigation", nil)
	default:
		return coreerr.E("CDPClient.validateNavigationURL", "navigation URL must use http, https, or about:blank", nil)
	}
}

func sameEndpointHost(httpURL, wsURL any) bool {
	httpEndpoint, err := cdpURLFromAny(httpURL)
	if err != nil {
		return false
	}
	wsEndpoint, err := cdpURLFromAny(wsURL)
	if err != nil {
		return false
	}
	return core.Lower(httpEndpoint.Hostname()) == core.Lower(wsEndpoint.Hostname()) && normalisedPort(httpEndpoint) == normalisedPort(wsEndpoint)
}

func normalisedPort(value any) string {
	u, err := cdpURLFromAny(value)
	if err != nil {
		return ""
	}

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

func targetIDFromWebSocketURL(raw string) (string, error) /* core.Result boundary */ {
	wsURL, err := parseCoreURL(raw)
	if err != nil {
		return "", err
	}

	targetID := core.PathBase(core.TrimSuffix(wsURL.Path, "/"))
	if targetID == "." || targetID == "/" || targetID == "" {
		return "", coreerr.E("CDPClient.targetIDFromWebSocketURL", "missing target ID in WebSocket URL", nil)
	}

	return targetID, nil
}

func (c *CDPClient) close(reason error) /* core.Result boundary */ {
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

func (c *CDPClient) failPending(err error) /* core.Result boundary */ {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

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
