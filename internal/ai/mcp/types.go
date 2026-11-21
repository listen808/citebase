package mcp

import "encoding/json"

// ServerConfig defines the configuration for an MCP server.
type ServerConfig struct {
	Name      string            `yaml:"name" json:"name"`
	Transport string            `yaml:"transport" json:"transport"` // "stdio" or "sse"
	Command   string            `yaml:"command" json:"command"`
	Args      []string          `yaml:"args" json:"args"`
	Env       map[string]string `yaml:"env" json:"env"`
	Endpoint  string            `yaml:"endpoint" json:"endpoint"`
	URL       string            `yaml:"url" json:"url"`
	Headers   map[string]string `yaml:"headers" json:"headers"`
}

// ToolDef describes a tool exposed by an MCP server.
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"inputSchema"`
}

// JSON-RPC 2.0 types used by MCP protocol.

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP initialize request/response payloads.

type initializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    clientCapabilities `json:"capabilities"`
	ClientInfo      clientInfo         `json:"clientInfo"`
}

type clientCapabilities struct {
	// Currently empty; can be extended for sampling, roots, etc.
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// tools/list response.

type toolsListResult struct {
	Tools []ToolDef `json:"tools"`
}

// tools/call request/response.

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"` // "text", "image", etc.
	Text string `json:"text,omitempty"`
}
