package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/version"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Transport is the interface for communicating with an MCP server.
type Transport interface {
	// Connect starts the connection and performs the MCP initialize handshake.
	Connect(ctx context.Context) error
	// CallTool invokes a tool on the server and returns the raw result.
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*toolCallResult, error)
	// ListTools returns the tools exposed by this server.
	ListTools(ctx context.Context) ([]ToolDef, error)
	// Close shuts down the transport and the underlying process/connection.
	Close() error
	// Tools returns the most recently fetched tool list (cached after Connect).
	Tools() []ToolDef
}

// StdioTransport communicates with an MCP server over stdin/stdout using JSON-RPC 2.0.
type StdioTransport struct {
	config ServerConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	nextID atomic.Int64
	tools  []ToolDef
	ready  bool

	// P1 修复（清单 57）：响应读取移出 t.mu 临界区。旧实现在持有 t.mu 的整个
	// sendRequest 期间阻塞式 t.stdout.Scan()：子进程收到请求后挂起不回行时，
	// ctx 取消/超时完全失效（Scan 不感知 context），且 t.mu 一直被持有，
	// 该 transport 上所有其它调用串行卡死。现在由 readLoop 后台解析响应行
	// 投递到 respChan，sendRequest 用 select 等待并尊重 ctx。
	respChan chan jsonrpcResponse
	readDone chan struct{}

	// P2-#39: 进程退出监控
	// closed 在 Close() 时置 true，让监控 goroutine 区分主动关闭与异常退出
	// done 在监控 goroutine 退出时关闭，Close() 等待它以避免 cmd.Wait() 被重复调用
	closed atomic.Bool
	done   chan struct{}
}

// jsonrpcResponse 需要可导出字段供 channel 传递（结构体本身已在本文件定义）

// readLoop 后台读取子进程 stdout，把逐行 JSON-RPC 响应投递到 respChan。
func (t *StdioTransport) readLoop() {
	defer close(t.readDone)
	for t.stdout.Scan() {
		line := strings.TrimSpace(t.stdout.Text())
		if line == "" {
			continue
		}
		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			logx.Infof("[MCP] Failed to parse response line: %s, error: %v", line, err)
			continue
		}
		select {
		case t.respChan <- resp:
		case <-time.After(2 * time.Second):
			// 无人消费（请求方已取消/缓冲区满）时丢弃，避免 readLoop 卡死
			logx.Infof("[MCP] response channel full, dropping message id=%d", resp.ID)
		}
	}
	if err := t.stdout.Err(); err != nil {
		logx.Infof("[MCP/Stdio] stdout read error: %v", err)
	}
}

// NewStdioTransport creates a new stdio-based transport.
func NewStdioTransport(cfg ServerConfig) *StdioTransport {
	return &StdioTransport{
		config: cfg,
	}
}

func (t *StdioTransport) Connect(ctx context.Context) error {
	t.cmd = exec.CommandContext(ctx, t.config.Command, t.config.Args...) //nolint:gosec // G204: MCP stdio transport requires subprocess
	if len(t.config.Env) > 0 {
		env := os.Environ()
		for k, v := range t.config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		t.cmd.Env = env
	}

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	t.cmd.Stderr = os.Stderr

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start mcp server: %w", err)
	}

	// P2-#39: 启动进程监控 goroutine，子进程异常退出时立即标记 transport 不可用
	// 由现有 30s 健康检查触发重连，避免最长 30s 的检测延迟
	t.done = make(chan struct{})
	go t.monitorProcess()

	t.stdout = bufio.NewScanner(stdoutPipe)
	t.stdout.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

	// P1 修复（清单 57）：后台读取响应 + 请求级 select 等待，见 readLoop 注释
	t.respChan = make(chan jsonrpcResponse, 32)
	t.readDone = make(chan struct{})
	go t.readLoop()

	// Perform initialize handshake.
	initResult, err := t.sendRequest(ctx, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    clientCapabilities{},
		ClientInfo: clientInfo{
			Name:    "citebase",
			Version: version.Version,
		},
	})
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}

	var result initializeResult
	if err := json.Unmarshal(initResult, &result); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	logx.Infof("[MCP] Connected to server: %s (protocol %s)", result.ServerInfo.Name, result.ProtocolVersion)

	// Send initialized notification.
	if err := t.sendNotification("notifications/initialized", nil); err != nil {
		return fmt.Errorf("send initialized notification: %w", err)
	}

	// Fetch tool list.
	tools, err := t.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	t.tools = tools
	t.ready = true

	logx.Infof("[MCP] Server %s exposes %d tools", t.config.Name, len(tools))
	return nil
}

func (t *StdioTransport) CallTool(ctx context.Context, name string, args map[string]interface{}) (*toolCallResult, error) {
	if !t.ready {
		return nil, fmt.Errorf("transport not connected")
	}

	raw, err := t.sendRequest(ctx, "tools/call", toolCallParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	var result toolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tool call result: %w", err)
	}
	return &result, nil
}

func (t *StdioTransport) ListTools(ctx context.Context) ([]ToolDef, error) {
	raw, err := t.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result toolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tools list: %w", err)
	}
	return result.Tools, nil
}

func (t *StdioTransport) Close() error {
	t.closed.Store(true)
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	// P2-#39: 等待监控 goroutine 完成 cmd.Wait()，避免重复调用 Wait
	if t.done != nil {
		<-t.done
	}
	// P1（清单 57）：等 readLoop 因 stdout EOF 退出，避免 goroutine 泄漏
	if t.readDone != nil {
		select {
		case <-t.readDone:
		case <-time.After(5 * time.Second):
		}
	}
	return nil
}

// monitorProcess 监控子进程退出状态。
// 子进程异常退出时（非 Close() 触发）：
//   - 立即标记 t.ready = false，使后续 CallTool 快速失败
//   - 由现有 30s 健康检查触发自动重连
//
// 主动关闭时（Close() 触发）：仅完成 cmd.Wait() 释放资源
func (t *StdioTransport) monitorProcess() {
	defer close(t.done)
	err := t.cmd.Wait()
	if t.closed.Load() {
		return
	}
	t.mu.Lock()
	t.ready = false
	t.mu.Unlock()
	logx.Warnf("[MCP/Stdio] Server %q exited unexpectedly: %v", t.config.Name, err)
}

func (t *StdioTransport) Tools() []ToolDef {
	return t.tools
}

// sendRequest sends a JSON-RPC request and waits for the matching response.
// P1 修复（清单 57）：不再在持锁期间阻塞式 Scan；响应由 readLoop 投递到 respChan，
// 这里用 select 等待并尊重 ctx 取消，另加 90s 兜底超时防止无 deadline 的调用者卡死。
func (t *StdioTransport) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := int(t.nextID.Add(1))
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := fmt.Fprintf(t.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	// Read dispatched responses until we find the one matching our ID.
	for {
		select {
		case <-waitCtx.Done():
			return nil, waitCtx.Err()
		case <-t.done:
			return nil, fmt.Errorf("mcp server process exited")
		case resp, ok := <-t.respChan:
			if !ok {
				return nil, fmt.Errorf("mcp server closed stdout")
			}

			// Skip notifications (no ID).
			if resp.ID == 0 {
				continue
			}

			if resp.ID != id {
				logx.Infof("[MCP] Received response for unexpected ID %d (expected %d)", resp.ID, id)
				continue
			}

			if resp.Error != nil {
				return nil, fmt.Errorf("json-rpc error %d: %s", resp.Error.Code, resp.Error.Message)
			}

			return resp.Result, nil
		}
	}
}

// sendNotification sends a JSON-RPC notification (no ID, no response expected).
func (t *StdioTransport) sendNotification(method string, params interface{}) error {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	_, err = fmt.Fprintf(t.stdin, "%s\n", data)
	return err
}

// SSETransport implements MCP over HTTP Server-Sent Events.
// It uses HTTP GET for the SSE event stream and HTTP POST for JSON-RPC requests.
type SSETransport struct {
	config       ServerConfig
	httpClient   *http.Client
	tools        []ToolDef
	ready        bool
	eventChan    chan string
	pendingReqs  map[int]chan sseResponse
	mu           sync.RWMutex
	nextID       atomic.Int64
	cancelStream context.CancelFunc
}

// NewSSETransport creates a new SSE-based transport.
func NewSSETransport(cfg ServerConfig) *SSETransport {
	return &SSETransport{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		eventChan:   make(chan string, 100),
		pendingReqs: make(map[int]chan sseResponse),
	}
}

func (t *SSETransport) Connect(ctx context.Context) error {
	sseURL := t.config.URL
	if sseURL == "" {
		sseURL = t.config.Endpoint
	}
	if sseURL == "" {
		return fmt.Errorf("SSE URL not configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}

	if len(t.config.Headers) > 0 {
		for k, v := range t.config.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := t.httpClient.Do(req) //nolint:bodyclose // body closed in readSSEStream goroutine
	if err != nil {
		return fmt.Errorf("SSE connection failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	t.cancelStream = cancel

	go t.readSSEStream(resp.Body, streamCtx)

	if err := t.initialize(ctx); err != nil {
		cancel()
		return err
	}

	if err := t.listTools(ctx); err != nil {
		cancel()
		return err
	}

	t.ready = true
	logx.Infof("[MCP/SSE] Connected to %s, exposed %d tools", sseURL, len(t.tools))
	return nil
}

// sseResponse 分发协程投递给等待方的响应单元（P1 清单 7）
type sseResponse struct {
	result json.RawMessage
	err    error
}

func (t *SSETransport) readSSEStream(body io.ReadCloser, ctx context.Context) {
	defer body.Close()

	// P1 修复（清单 7）：SSE 传输此前只把事件写进 eventChan，没有任何消费者，
	// pendingReqs 的 respChan 永远无人投递 —— 所有 sendRequest 必然挂满 60s 超时，
	// transport="sse" 的 MCP server 彻底不可用。启动按 jsonrpc id 分发的协程，
	// 把响应路由回对应的等待方。
	go t.dispatchEvents(ctx)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventData bytes.Buffer

	flushEvent := func() {
		if eventData.Len() > 0 {
			select {
			case t.eventChan <- eventData.String():
			case <-ctx.Done():
				return
			}
		}
		eventData.Reset()
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			flushEvent()
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if strings.HasPrefix(data, ":") {
				continue
			}
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(data)
		} else if line == "" {
			flushEvent()
		}
	}

	if err := scanner.Err(); err != nil {
		logx.Infof("[MCP/SSE] Stream error: %v", err)
	}
}

// dispatchEvents 消费 eventChan，将 JSON-RPC 响应按 id 投递给 pendingReqs 中的等待者。
// ctx 为 SSE 流的生命周期上下文：流断开/Close 时随之退出。
func (t *SSETransport) dispatchEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-t.eventChan:
			var resp jsonrpcResponse
			if err := json.Unmarshal([]byte(data), &resp); err != nil {
				// 非 JSON-RPC 事件（如 endpoint 通告、心跳）忽略
				continue
			}
			if resp.ID == 0 {
				// 通知（无 id），当前无订阅处理需求
				continue
			}
			t.mu.RLock()
			ch, ok := t.pendingReqs[resp.ID]
			t.mu.RUnlock()
			if !ok {
				continue
			}
			out := sseResponse{}
			if resp.Error != nil {
				out.err = fmt.Errorf("json-rpc error %d: %s", resp.Error.Code, resp.Error.Message)
			} else {
				out.result = resp.Result
			}
			// respChan 带 1 缓冲且请求方可能已超时离开：非阻塞投递，投不去即丢弃
			select {
			case ch <- out:
			default:
			}
		}
	}
}

func (t *SSETransport) initialize(ctx context.Context) error {
	result, err := t.sendRequest(ctx, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    clientCapabilities{},
		ClientInfo: clientInfo{
			Name:    "citebase",
			Version: version.Version,
		},
	})
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}

	var initResult initializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	logx.Infof("[MCP/SSE] Server: %s (protocol %s)", initResult.ServerInfo.Name, initResult.ProtocolVersion)

	return t.sendNotification(ctx, "notifications/initialized", nil)
}

func (t *SSETransport) listTools(ctx context.Context) error {
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

func (t *SSETransport) CallTool(ctx context.Context, name string, args map[string]interface{}) (*toolCallResult, error) {
	if !t.ready {
		return nil, fmt.Errorf("transport not connected")
	}

	raw, err := t.sendRequest(ctx, "tools/call", toolCallParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	var result toolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tool call result: %w", err)
	}
	return &result, nil
}

func (t *SSETransport) ListTools(ctx context.Context) ([]ToolDef, error) {
	if !t.ready {
		return nil, fmt.Errorf("transport not connected")
	}
	return t.tools, nil
}

func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancelStream != nil {
		t.cancelStream()
	}

	t.ready = false

	for id := range t.pendingReqs {
		// 只摘除登记，不 close(channel)：分发协程可能仍在非阻塞投递，
		// close 会引入 send-on-closed panic；等待方自会超时退出
		delete(t.pendingReqs, id)
	}

	return nil
}

func (t *SSETransport) Tools() []ToolDef {
	return t.tools
}

func (t *SSETransport) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	reqID := int(t.nextID.Add(1))

	t.mu.Lock()
	respChan := make(chan sseResponse, 1)
	t.pendingReqs[reqID] = respChan
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pendingReqs, reqID)
		t.mu.Unlock()
	}()

	if err := t.doPostRequest(ctx, method, reqID, params); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.err != nil {
			return nil, resp.err
		}
		return resp.result, nil
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

func (t *SSETransport) doPostRequest(ctx context.Context, method string, reqID int, params interface{}) error {
	postURL := t.config.URL
	if postURL == "" {
		return fmt.Errorf("POST URL not configured")
	}

	reqBody := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", postURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if len(t.config.Headers) > 0 {
		for k, v := range t.config.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (t *SSETransport) sendNotification(ctx context.Context, method string, params interface{}) error {
	postURL := t.config.URL
	if postURL == "" {
		return fmt.Errorf("POST URL not configured")
	}

	reqBody := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", postURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create notification request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if len(t.config.Headers) > 0 {
		for k, v := range t.config.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send notification failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
