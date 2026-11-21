package mcp

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"sync"
	"time"
)

// ServerEntry holds a connected MCP server and its transport.
type ServerEntry struct {
	Config    ServerConfig
	Transport Transport
	Tools     []ToolDef
	Connected bool
	Error     string

	lastHealthCheck time.Time
	reconnectCount  int
	stopHealthCheck chan struct{}
}

// Client manages connections to multiple MCP servers.
type Client struct {
	servers map[string]*ServerEntry
	mu      sync.RWMutex
}

// NewClient creates a new MCP client.
func NewClient() *Client {
	return &Client{
		servers: make(map[string]*ServerEntry),
	}
}

// AddServer connects to an MCP server and registers it.
func (c *Client) AddServer(ctx context.Context, cfg ServerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.servers[cfg.Name]; exists {
		return fmt.Errorf("mcp server %q already registered", cfg.Name)
	}

	transport := c.createTransport(cfg)
	if transport == nil {
		return fmt.Errorf("unsupported transport type: %q (want stdio|sse|streamable_http)", cfg.Transport)
	}

	entry := &ServerEntry{
		Config:    cfg,
		Transport: transport,
	}

	if err := transport.Connect(ctx); err != nil {
		entry.Error = err.Error()
		entry.Connected = false
		c.servers[cfg.Name] = entry
		return fmt.Errorf("connect to mcp server %q: %w", cfg.Name, err)
	}

	// 统一从 Transport 接口读取已暴露的工具（不再依赖具体类型断言）。
	entry.Tools = transport.Tools()
	entry.Connected = true

	c.servers[cfg.Name] = entry
	c.startHealthCheck(cfg.Name)
	logx.Infof("[MCP] Server %q added with %d tools", cfg.Name, len(entry.Tools))
	return nil
}

// createTransport builds a Transport from the given config.
// Must be called while holding c.mu (or on a freshly-created entry).
func (c *Client) createTransport(cfg ServerConfig) Transport {
	switch cfg.Transport {
	case "stdio":
		return NewStdioTransport(cfg)
	case "sse":
		return NewSSETransport(cfg)
	case "streamable_http", "http":
		return NewStreamableHTTPTransport(cfg)
	default:
		return nil
	}
}

// startHealthCheck launches a background goroutine that pings the server every 30s.
// Must be called while holding c.mu so that stopHealthCheck channel setup is race-free.
func (c *Client) startHealthCheck(name string) {
	entry, ok := c.servers[name]
	if !ok {
		return
	}
	entry.stopHealthCheck = make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-entry.stopHealthCheck:
				return
			case <-ticker.C:
				c.healthCheck(name)
			}
		}
	}()
}

// healthCheck pings the server via ListTools; on failure it marks the server
// disconnected and triggers an auto-reconnect.
func (c *Client) healthCheck(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.mu.RLock()
	entry, ok := c.servers[name]
	c.mu.RUnlock()
	if !ok || !entry.Connected || entry.Transport == nil {
		return
	}
	transport := entry.Transport

	if _, err := transport.ListTools(ctx); err != nil {
		logx.Infof("[MCP] Health check failed for %q: %v", name, err)
		c.mu.Lock()
		entry.Connected = false
		entry.Error = fmt.Sprintf("health check failed: %v", err)
		c.mu.Unlock()

		if err := c.reconnect(ctx, name); err != nil {
			logx.Infof("[MCP] Auto-reconnect failed for %q: %v", name, err)
		}
		return
	}

	c.mu.Lock()
	entry.lastHealthCheck = time.Now()
	c.mu.Unlock()
}

// reconnect attempts to re-establish the connection to a server.
// Up to 3 retries with exponential backoff (1s, 2s, 4s).
func (c *Client) reconnect(ctx context.Context, name string) error {
	c.mu.Lock()
	entry, ok := c.servers[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("server %q not found", name)
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		backoff := time.Duration(1<<uint(i)) * time.Second
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}

		c.mu.Lock()
		if entry.Transport != nil {
			entry.Transport.Close()
		}
		cfg := entry.Config
		c.mu.Unlock()

		transport := c.createTransport(cfg)
		if transport == nil {
			c.mu.Lock()
			entry.reconnectCount++
			c.mu.Unlock()
			continue
		}

		if err := transport.Connect(ctx); err != nil {
			c.mu.Lock()
			entry.reconnectCount++
			c.mu.Unlock()
			continue
		}

		c.mu.Lock()
		entry.Transport = transport
		entry.Tools = transport.Tools()
		entry.Connected = true
		entry.Error = ""
		entry.reconnectCount = 0
		c.mu.Unlock()

		logx.Infof("[MCP] Server %q reconnected successfully", name)
		return nil
	}

	c.mu.Lock()
	entry.Connected = false
	entry.Error = "reconnect failed after 3 retries"
	c.mu.Unlock()

	logx.Infof("[MCP] Server %q reconnect failed after %d retries", name, maxRetries)
	return fmt.Errorf("reconnect failed for server %q", name)
}

// RemoveServer disconnects and removes an MCP server.
func (c *Client) RemoveServer(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.servers[name]
	if !ok {
		return fmt.Errorf("mcp server %q not found", name)
	}

	if entry.stopHealthCheck != nil {
		close(entry.stopHealthCheck)
		entry.stopHealthCheck = nil
	}

	if entry.Transport != nil {
		entry.Transport.Close()
	}

	delete(c.servers, name)
	logx.Infof("[MCP] Server %q removed", name)
	return nil
}

// GetServer returns the entry for a specific server.
func (c *Client) GetServer(name string) (*ServerEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.servers[name]
	return entry, ok
}

// ListServers returns all registered server entries.
func (c *Client) ListServers() []*ServerEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*ServerEntry, 0, len(c.servers))
	for _, entry := range c.servers {
		result = append(result, entry)
	}
	return result
}

// GetAllTools returns tools from all connected servers, prefixed with server name.
func (c *Client) GetAllTools() []ToolDef {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allTools []ToolDef
	for _, entry := range c.servers {
		if !entry.Connected {
			continue
		}
		for _, tool := range entry.Tools {
			// Prefix tool name with server name to avoid collisions.
			prefixed := tool
			prefixed.Name = entry.Config.Name + ":" + tool.Name
			allTools = append(allTools, prefixed)
		}
	}
	return allTools
}

// CallTool calls a tool on the specified server.
// The toolName can be in "serverName:toolName" format or just "toolName" if serverName is provided.
// If the server is disconnected or the call fails, it attempts one reconnect + retry.
func (c *Client) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (*toolCallResult, error) {
	c.mu.RLock()
	entry, ok := c.servers[serverName]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("mcp server %q not found", serverName)
	}
	if !entry.Connected {
		if err := c.reconnect(ctx, serverName); err != nil {
			return nil, fmt.Errorf("mcp server %q is not connected: %w", serverName, err)
		}
	}

	c.mu.RLock()
	entry, ok = c.servers[serverName]
	c.mu.RUnlock()
	if !ok || entry.Transport == nil {
		return nil, fmt.Errorf("mcp server %q is not connected", serverName)
	}
	transport := entry.Transport

	result, err := transport.CallTool(ctx, toolName, args)
	if err != nil {
		logx.Infof("[MCP] Tool call failed on %q, attempting reconnect: %v", serverName, err)
		if reconnectErr := c.reconnect(ctx, serverName); reconnectErr == nil {
			c.mu.RLock()
			entry, ok = c.servers[serverName]
			c.mu.RUnlock()
			if ok && entry.Transport != nil {
				return entry.Transport.CallTool(ctx, toolName, args)
			}
		}
		return nil, err
	}
	return result, nil
}

// CallToolByFullName calls a tool using the "serverName:toolName" format.
func (c *Client) CallToolByFullName(ctx context.Context, fullName string, args map[string]interface{}) (*toolCallResult, error) {
	serverName, toolName, err := ParseToolName(fullName)
	if err != nil {
		return nil, err
	}
	return c.CallTool(ctx, serverName, toolName, args)
}

// ParseToolName splits a "serverName:toolName" into its parts.
func ParseToolName(fullName string) (serverName, toolName string, err error) {
	for i, ch := range fullName {
		if ch == ':' {
			return fullName[:i], fullName[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("tool name %q does not follow 'serverName:toolName' format", fullName)
}

// RouteTool 根据工具全名选择最优服务器并执行调用。
// 当工具名包含 "serverName:toolName" 前缀时按前缀精确路由；
// 否则在所有已连接服务器中查找同名工具并按优先级（配置顺序）选择第一个。
func (c *Client) RouteTool(ctx context.Context, fullName string, args map[string]interface{}) (*toolCallResult, error) {
	serverName, toolName, err := ParseToolName(fullName)
	if err == nil {
		return c.CallTool(ctx, serverName, toolName, args)
	}

	// 无前缀：在所有服务器中查找同名工具，按注册顺序返回首个匹配。
	// P1 修复（清单 56）：锁内只解析 transport 引用，远程 CallTool 移到锁外执行。
	// 旧实现 defer RUnlock 后在持读锁期间阻塞整个工具调用（stdio/HTTP 可达 60-90s），
	// 挂起的写者（healthCheck/AddServer/RemoveServer）会连带阻塞后续所有读者。
	var transport Transport
	c.mu.RLock()
	for _, entry := range c.servers {
		if !entry.Connected {
			continue
		}
		for _, tool := range entry.Tools {
			if tool.Name == fullName {
				transport = entry.Transport
				break
			}
		}
		if transport != nil {
			break
		}
	}
	c.mu.RUnlock()
	if transport == nil {
		return nil, fmt.Errorf("mcp tool %q not found on any connected server", fullName)
	}
	return transport.CallTool(ctx, fullName, args)
}

// Close shuts down all MCP server connections.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, entry := range c.servers {
		if entry.stopHealthCheck != nil {
			close(entry.stopHealthCheck)
			entry.stopHealthCheck = nil
		}
		if entry.Transport != nil {
			if err := entry.Transport.Close(); err != nil {
				logx.Infof("[MCP] Error closing server %q: %v", name, err)
			}
		}
	}
	c.servers = make(map[string]*ServerEntry)
	logx.Info("[MCP] All servers closed")
}

// ---------------------------------------------------------------------------
// MCPRouter — keyword-based intelligent query routing
// ---------------------------------------------------------------------------

// ToolCategory represents a category of tools that can be matched by keywords.
type ToolCategory string

const (
	CategorySearch   ToolCategory = "search"
	CategoryDatabase ToolCategory = "database"
	CategoryFile     ToolCategory = "file"
	CategoryNetwork  ToolCategory = "network"
	CategoryAI       ToolCategory = "ai"
	CategoryCode     ToolCategory = "code"
	CategoryGeneric  ToolCategory = "generic"
)

// RoutingRule maps a keyword pattern to a tool category and preferred server.
type RoutingRule struct {
	Keywords        []string     `json:"keywords"`
	Category        ToolCategory `json:"category"`
	PreferredServer string       `json:"preferred_server,omitempty"`
	Priority        int          `json:"priority"` // higher = checked first
}

// MCPRouter performs keyword-based classification of user queries and routes
// tool calls to the most appropriate MCP server.
type MCPRouter struct {
	client *Client
	rules  []RoutingRule
	mu     sync.RWMutex
}

// NewMCPRouter creates a router with default routing rules.
func NewMCPRouter(client *Client) *MCPRouter {
	router := &MCPRouter{
		client: client,
	}
	router.loadDefaultRules()
	return router
}

func (r *MCPRouter) loadDefaultRules() {
	r.rules = []RoutingRule{
		{
			Keywords: []string{"搜索", "查找", "查询", "search", "find", "query", "检索", "lookup"},
			Category: CategorySearch,
			Priority: 10,
		},
		{
			Keywords: []string{"数据库", "sql", "表", "数据", "database", "db", "table", "record", "查询数据"},
			Category: CategoryDatabase,
			Priority: 9,
		},
		{
			Keywords: []string{"文件", "读取", "写入", "上传", "下载", "file", "read", "write", "upload", "download", "文档"},
			Category: CategoryFile,
			Priority: 8,
		},
		{
			Keywords: []string{"网络", "网页", "url", "http", "api", "请求", "fetch", "crawl", "网页抓取"},
			Category: CategoryNetwork,
			Priority: 7,
		},
		{
			Keywords: []string{"分析", "总结", "摘要", "翻译", "生成", "analyze", "summarize", "translate", "generate", "ai", "llm"},
			Category: CategoryAI,
			Priority: 6,
		},
		{
			Keywords: []string{"代码", "编程", "code", "compile", "run", "执行", "函数", "function"},
			Category: CategoryCode,
			Priority: 5,
		},
	}
}

// Classify analyzes a user query and returns the best matching tool category
// and recommended server name.
func (r *MCPRouter) Classify(query string) (ToolCategory, string) {
	lower := strings.ToLower(query)

	// Score each rule against the query
	bestScore := 0
	bestCategory := CategoryGeneric
	bestServer := ""

	for _, rule := range r.rules {
		score := 0
		for _, kw := range rule.Keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		score += rule.Priority
		if score > bestScore {
			bestScore = score
			bestCategory = rule.Category
			bestServer = rule.PreferredServer
		}
	}

	// If no preferred server set, find a connected server with matching tools
	if bestServer == "" && bestCategory != CategoryGeneric {
		bestServer = r.findServerForCategory(bestCategory)
	}

	return bestCategory, bestServer
}

// findServerForCategory searches connected servers for one that has tools
// matching the given category.
func (r *MCPRouter) findServerForCategory(cat ToolCategory) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.client == nil {
		return ""
	}

	r.client.mu.RLock()
	defer r.client.mu.RUnlock()

	categoryKeywords := map[ToolCategory][]string{
		CategorySearch:   {"search", "find", "query", "lookup", "检索"},
		CategoryDatabase: {"database", "sql", "db", "table", "数据"},
		CategoryFile:     {"file", "read", "write", "upload", "文件"},
		CategoryNetwork:  {"http", "url", "web", "fetch", "api", "网络"},
		CategoryAI:       {"ai", "llm", "generate", "summarize", "翻译"},
		CategoryCode:     {"code", "compile", "run", "exec", "代码"},
	}

	keywords := categoryKeywords[cat]
	for _, entry := range r.client.servers {
		if !entry.Connected {
			continue
		}
		for _, tool := range entry.Tools {
			toolLower := strings.ToLower(tool.Name + " " + tool.Description)
			for _, kw := range keywords {
				if strings.Contains(toolLower, kw) {
					return entry.Config.Name
				}
			}
		}
	}
	return ""
}

// RouteQuery classifies a query and executes the appropriate tool call.
// It automatically selects the best server and tool based on the query content.
func (r *MCPRouter) RouteQuery(ctx context.Context, query string, args map[string]interface{}) (*toolCallResult, error) {
	category, serverName := r.Classify(query)

	if serverName == "" {
		return nil, fmt.Errorf("no suitable MCP server found for query category %q", category)
	}

	// Find the best matching tool on the selected server
	r.client.mu.RLock()
	entry, ok := r.client.servers[serverName]
	r.client.mu.RUnlock()
	if !ok || !entry.Connected {
		return nil, fmt.Errorf("MCP server %q not connected", serverName)
	}

	toolName := r.findBestTool(entry.Tools, query, category)
	if toolName == "" {
		return nil, fmt.Errorf("no matching tool found on server %q for category %q", serverName, category)
	}

	return r.client.CallTool(ctx, serverName, toolName, args)
}

// findBestTool selects the most appropriate tool from a list based on the query.
func (r *MCPRouter) findBestTool(tools []ToolDef, query string, cat ToolCategory) string {
	lower := strings.ToLower(query)
	bestScore := 0
	bestName := ""

	for _, tool := range tools {
		score := 0
		toolText := strings.ToLower(tool.Name + " " + tool.Description)
		words := strings.Fields(lower)
		for _, w := range words {
			if len(w) >= 3 && strings.Contains(toolText, w) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestName = tool.Name
		}
	}

	// If no keyword match, return the first tool (fallback)
	if bestName == "" && len(tools) > 0 {
		bestName = tools[0].Name
	}
	return bestName
}

// AddRule adds a custom routing rule.
func (r *MCPRouter) AddRule(rule RoutingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = append(r.rules, rule)
}

// Rules returns all current routing rules.
func (r *MCPRouter) Rules() []RoutingRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RoutingRule, len(r.rules))
	copy(out, r.rules)
	return out
}
