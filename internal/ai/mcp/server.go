package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/wiki"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	"github.com/listen808/citebase/internal/version"
)

// DocSearchFn 原始文档检索回调，由调用方注入（rag.Retriever.RetrieveScoped）。
// 用函数而非 *rag.Retriever 直接引用，避免 mcp ↔ rag 循环依赖
// （rag 已 import mcp 的客户端类型）。
type DocSearchFn func(ctx context.Context, question string, spaceID int, topK int) ([]Source, error)

// Source 原始文档检索返回的片段。字段名与 rag.Source 对齐，仅保留外部 Agent 需要的部分。
type Source struct {
	DocumentName  string  `json:"document_name,omitempty"`
	DocumentID    int     `json:"document_id,omitempty"`
	Snippet       string  `json:"snippet,omitempty"`
	HeadingPath   string  `json:"heading_path,omitempty"`
	CitationIndex int     `json:"citation_index,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

// Server 把知识库暴露给外部 Agent（Claude Code / Codex 等）的 MCP 服务端。
//
// 项目此前只有 MCP 客户端，知识库无法被外部 Agent 查询。这里补上服务端，
// 走 streamable HTTP（JSON-RPC 2.0 over HTTP），与客户端复用同一套协议类型。
type Server struct {
	tools       []ToolDef
	handlers    map[string]ToolHandler
	compiler    *wiki.Compiler
	retriever   *wiki.Retriever
	docSearchFn DocSearchFn // 原始文档检索（带 ctx 用户权限过滤）；nil 时该 tool 不可用
}

// ToolHandler 工具实现。参数已是校验后的 map。
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// NewWikiServer 注册知识库相关的 MCP 工具。
// docSearch 用于检索尚未编译成 wiki 页的原始文档；nil 时该 tool 不可用。
func NewWikiServer(p provider.Provider, docSearch DocSearchFn) *Server {
	compiler := wiki.NewCompiler(p)
	retriever := wiki.NewRetriever(p)

	s := &Server{
		handlers:    make(map[string]ToolHandler),
		compiler:    compiler,
		retriever:   retriever,
		docSearchFn: docSearch,
	}

	s.register(ToolDef{
		Name:        "wiki_search",
		Description: "在知识库中检索（目录导航 + 全文双路召回），返回候选知识页面",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"space_id": map[string]interface{}{"type": "integer", "description": "空间ID"},
				"question": map[string]interface{}{"type": "string", "description": "检索问题"},
				"top_k":    map[string]interface{}{"type": "integer", "description": "全文旁路补充的条数，默认 2"},
			},
			"required": []string{"space_id", "question"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		spaceID := intArg(args, "space_id")
		question, _ := args["question"].(string)
		topK := intArg(args, "top_k")
		return retriever.Retrieve(ctx, spaceID, question, topK)
	})

	s.register(ToolDef{
		Name:        "wiki_get_page",
		Description: "读取一个知识页面的完整正文",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"space_id": map[string]interface{}{"type": "integer"},
				"slug":     map[string]interface{}{"type": "string"},
			},
			"required": []string{"space_id", "slug"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		spaceID := intArg(args, "space_id")
		slug, _ := args["slug"].(string)
		if slug == "" {
			return nil, fmt.Errorf("slug 不能为空")
		}
		return s.compiler.GetPageForRead(spaceID, slug)
	})

	s.register(ToolDef{
		Name:        "wiki_list_pages",
		Description: "列出空间下的知识页面",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"space_id": map[string]interface{}{"type": "integer"},
				"status":   map[string]interface{}{"type": "string", "description": "planned/draft/published，留空为全部"},
			},
			"required": []string{"space_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		spaceID := intArg(args, "space_id")
		status, _ := args["status"].(string)
		return s.compiler.ListPagesForRead(spaceID, status)
	})

	s.register(ToolDef{
		Name:        "wiki_graph",
		Description: "返回知识页面的交叉引用边，用于图谱遍历",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"space_id": map[string]interface{}{"type": "integer"},
			},
			"required": []string{"space_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return s.compiler.LinksForRead(intArg(args, "space_id"))
	})

	s.register(ToolDef{
		Name:        "wiki_related",
		Description: "返回与指定页面最相关的页面（直接引用/同源/共同邻居/同类型 四信号加权）",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"space_id": map[string]interface{}{"type": "integer"},
				"slug":     map[string]interface{}{"type": "string"},
				"limit":    map[string]interface{}{"type": "integer"},
			},
			"required": []string{"space_id", "slug"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		slug, _ := args["slug"].(string)
		return compiler.RelatedPages(intArg(args, "space_id"), slug, intArg(args, "limit"))
	})

	// P5：待人工裁决的审核项（含深度研究纪要），外部 Agent 可据此协助补全知识
	s.register(ToolDef{
		Name:        "wiki_reviews",
		Description: "返回空间内待人工裁决的审核项（草稿确认/深度研究等），只读",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"space_id": map[string]interface{}{"type": "integer"},
			},
			"required": []string{"space_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return compiler.ListPendingReviews(intArg(args, "space_id"))
	})

	// 原始文档检索：wiki 层只覆盖已编译的知识页，未编译的文档要靠这条。
	// 走 RetrieveScoped —— 结果按 ctx 携带的用户身份做权限过滤，
	// 与 wiki 工具一样，外部 Agent 不能借检索越权读文档。
	if s.docSearchFn != nil {
		s.register(ToolDef{
			Name:        "doc_search",
			Description: "检索空间内原始文档片段（向量∪全文混合召回，含未编译进 wiki 的文档），返回带引用的候选段落",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"space_id": map[string]interface{}{"type": "integer", "description": "空间ID"},
					"question": map[string]interface{}{"type": "string", "description": "检索问题"},
					"top_k":    map[string]interface{}{"type": "integer", "description": "返回条数，默认 5，上限 20"},
				},
				"required": []string{"space_id", "question"},
			},
		}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			spaceID := intArg(args, "space_id")
			question, _ := args["question"].(string)
			topK := intArg(args, "top_k")
			return s.docSearchFn(ctx, question, spaceID, topK)
		})
	}

	return s
}

func (s *Server) register(def ToolDef, h ToolHandler) {
	s.tools = append(s.tools, def)
	s.handlers[def.Name] = h
}

// Handle 处理一条 JSON-RPC 请求，返回待序列化的响应。
func (s *Server) Handle(ctx context.Context, raw []byte) (interface{}, error) {
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC payload: %w", err)
	}

	switch req.Method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "citebase", "version": version.Version},
		}, nil

	case "tools/list":
		return map[string]interface{}{"tools": s.tools}, nil

	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &p); err != nil {
				return nil, fmt.Errorf("invalid tools/call params: %w", err)
			}
		}
		h, ok := s.handlers[p.Name]
		if !ok {
			return nil, fmt.Errorf("unknown tool: %s", p.Name)
		}
		out, err := h(ctx, p.Arguments)
		if err != nil {
			return map[string]interface{}{
				"content": []map[string]interface{}{{"type": "text", "text": "错误: " + err.Error()}},
				"isError": true,
			}, nil
		}
		text, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": string(text)}},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported method: %s", req.Method)
	}
}

// PublishedOnly 工具默认只暴露已发布页面，避免把未审核草稿泄漏给外部 Agent。
const PublishedOnly = string(knowledgemodel.WikiPageStatusPublished)

func intArg(args map[string]interface{}, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return 0
}
