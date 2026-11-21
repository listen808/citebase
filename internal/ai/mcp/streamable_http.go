package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/version"
)

// StreamableHTTPTransport implements the MCP Streamable HTTP transport
// (protocol 2025-03-26). A single endpoint accepts JSON-RPC POST requests;
// the server may reply with either a plain JSON response or an SSE stream.
// Optional GET opens a standalone server->client notification stream.
type StreamableHTTPTransport struct {
	config     ServerConfig
	httpClient *http.Client
	tools      []ToolDef
	ready      bool

	mu          sync.RWMutex
	nextID      atomic.Int64
	pendingReqs map[int]chan json.RawMessage
	sessionID   string // MCP-Session-Id returned by server
	protoVer    string

	cancelStream context.CancelFunc
}

// NewStreamableHTTPTransport creates a Streamable HTTP transport.
// cfg.URL must point to the MCP endpoint (e.g. http://host:port/mcp).
func NewStreamableHTTPTransport(cfg ServerConfig) *StreamableHTTPTransport {
	return &StreamableHTTPTransport{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		pendingReqs: make(map[int]chan json.RawMessage),
		protoVer:    "2025-03-26",
	}
}

func (t *StreamableHTTPTransport) Connect(ctx context.Context) error {
	if t.config.URL == "" {
		return fmt.Errorf("Streamable HTTP URL not configured")
	}

	if err := t.initialize(ctx); err != nil {
		return err
	}
	if err := t.listTools(ctx); err != nil {
		return err
	}

	// Optionally open the standalone GET stream for server-initiated notifications.
	go t.maybeOpenNotificationStream()

	t.ready = true
	logx.Infof("[MCP/HTTP] Connected to %s, exposed %d tools (session=%s)", t.config.URL, len(t.tools), t.sessionID)
	return nil
}

// maybeOpenNotificationStream opens a GET SSE stream if the server supports it.
// Failures are non-fatal: notifications are best-effort.
func (t *StreamableHTTPTransport) maybeOpenNotificationStream() {
	req, err := http.NewRequestWithContext(context.Background(), "GET", t.config.URL, nil)
	if err != nil {
		return
	}
	t.applyHeaders(req)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.cancelStream = cancel
	t.mu.Unlock()

	go t.readNotificationStream(resp.Body, streamCtx)
}

func (t *StreamableHTTPTransport) readNotificationStream(body io.ReadCloser, ctx context.Context) {
	defer body.Close()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var data bytes.Buffer
	flush := func() {
		if data.Len() == 0 {
			return
		}
		var note jsonrpcResponse
		if err := json.Unmarshal(data.Bytes(), &note); err == nil && note.ID != 0 {
			t.mu.RLock()
			ch, ok := t.pendingReqs[note.ID]
			t.mu.RUnlock()
			if ok {
				select {
				case ch <- note.Result:
				default:
				}
			}
		}
		data.Reset()
	}
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			flush()
		case strings.HasPrefix(line, "data:"):
			d := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if strings.HasPrefix(d, ":") {
				continue
			}
			if data.Len() > 0 {
				data.WriteString("\n")
			}
			data.WriteString(d)
		case line == "":
			flush()
		}
	}
}

func (t *StreamableHTTPTransport) initialize(ctx context.Context) error {
	result, err := t.sendRequest(ctx, "initialize", initializeParams{
		ProtocolVersion: t.protoVer,
		Capabilities:    clientCapabilities{},
		ClientInfo:      clientInfo{Name: "citebase", Version: version.Version},
	})
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	var initResult initializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}
	// Server may negotiate a different protocol version.
	if initResult.ProtocolVersion != "" {
		t.protoVer = initResult.ProtocolVersion
	}
	logx.Infof("[MCP/HTTP] Server: %s (protocol %s)", initResult.ServerInfo.Name, t.protoVer)
	return t.sendNotification(ctx, "notifications/initialized", nil)
}

func (t *StreamableHTTPTransport) listTools(ctx context.Context) error {
	raw, err := t.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	var result toolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse tools list: %w", err)
	}
	t.tools = result.Tools
	return nil
}

func (t *StreamableHTTPTransport) CallTool(ctx context.Context, name string, args map[string]interface{}) (*toolCallResult, error) {
	if !t.ready {
		return nil, fmt.Errorf("transport not connected")
	}
	raw, err := t.sendRequest(ctx, "tools/call", toolCallParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	var result toolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tool call result: %w", err)
	}
	return &result, nil
}

func (t *StreamableHTTPTransport) ListTools(ctx context.Context) ([]ToolDef, error) {
	if !t.ready {
		return nil, fmt.Errorf("transport not connected")
	}
	return t.tools, nil
}

func (t *StreamableHTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancelStream != nil {
		t.cancelStream()
	}
	for id, ch := range t.pendingReqs {
		close(ch)
		delete(t.pendingReqs, id)
	}
	return nil
}

func (t *StreamableHTTPTransport) Tools() []ToolDef { return t.tools }

// applyHeaders sets Content-Type/Accept and any configured auth headers,
// plus the negotiated session id (if any).
func (t *StreamableHTTPTransport) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
	}
	t.mu.RLock()
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	t.mu.RUnlock()
}

func (t *StreamableHTTPTransport) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	reqID := int(t.nextID.Add(1))

	t.mu.Lock()
	respChan := make(chan json.RawMessage, 1)
	t.pendingReqs[reqID] = respChan
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pendingReqs, reqID)
		t.mu.Unlock()
	}()

	if err := t.doPost(ctx, method, reqID, params, respChan); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-respChan:
		return result, nil
	case <-time.After(90 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

// doPost sends the JSON-RPC request and routes the response. The server may
// respond with a plain JSON object, or an SSE stream carrying the result and
// possibly trailing notifications. We support both.
func (t *StreamableHTTPTransport) doPost(ctx context.Context, method string, reqID int, params interface{}, respChan chan json.RawMessage) error {
	body, _ := json.Marshal(jsonrpcRequest{JSONRPC: "2.0", ID: reqID, Method: method, Params: params})

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}
	t.applyHeaders(req)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST request failed: %w", err)
	}
	defer resp.Body.Close()

	// Capture session id for subsequent requests.
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.sessionID = sid
		t.mu.Unlock()
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
	default:
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST returned status %d: %s", resp.StatusCode, string(b))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		// SSE response: scan for the result event with matching id.
		return t.consumeSSEResponse(resp.Body, reqID, respChan)
	}

	// Plain JSON response.
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var r jsonrpcResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if r.Error != nil {
		return fmt.Errorf("json-rpc error %d: %s", r.Error.Code, r.Error.Message)
	}
	respChan <- r.Result
	return nil
}

// consumeSSEResponse reads an SSE stream until it finds the JSON-RPC response
// matching reqID, then delivers it on respChan. Trailing notifications are
// dispatched to any waiting channels but do not block termination.
func (t *StreamableHTTPTransport) consumeSSEResponse(body io.ReadCloser, reqID int, respChan chan json.RawMessage) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var data bytes.Buffer
	deliver := func() error {
		if data.Len() == 0 {
			return nil
		}
		var r jsonrpcResponse
		if err := json.Unmarshal(data.Bytes(), &r); err != nil {
			data.Reset()
			return nil
		}
		data.Reset()
		if r.ID != reqID {
			// Notification or out-of-order event: ignore for this request.
			return nil
		}
		if r.Error != nil {
			return fmt.Errorf("json-rpc error %d: %s", r.Error.Code, r.Error.Message)
		}
		// P3：Close 可能已关闭 channel，用 recover 避免 send on closed channel panic
		func() {
			defer func() { recover() }()
			respChan <- r.Result
		}()
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			_ = deliver()
		case strings.HasPrefix(line, "data:"):
			d := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if strings.HasPrefix(d, ":") {
				continue
			}
			if data.Len() > 0 {
				data.WriteString("\n")
			}
			data.WriteString(d)
		case line == "":
			if err := deliver(); err != nil {
				return err
			}
		}
	}
	return deliver()
}

func (t *StreamableHTTPTransport) sendNotification(ctx context.Context, method string, params interface{}) error {
	body, _ := json.Marshal(jsonrpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	req, err := http.NewRequestWithContext(ctx, "POST", t.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	t.applyHeaders(req)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send notification failed: %w", err)
	}
	defer resp.Body.Close()
	return nil
}
