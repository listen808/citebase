package rag

import (
	"fmt"
	"math"
	"sync"
)

type ToolRegistry struct {
	tools map[string]ToolExecutor
	mu    sync.RWMutex
}

var globalToolRegistry *ToolRegistry

func init() {
	globalToolRegistry = &ToolRegistry{
		tools: make(map[string]ToolExecutor),
	}
}

func GetToolRegistry() *ToolRegistry {
	return globalToolRegistry
}

func (r *ToolRegistry) Register(name string, tool ToolExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = tool
}

// Unregister 删除已注册的工具，常用于 MCP 服务器卸载时清理以 serverName:toolName 前缀注册的工具
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

func (r *ToolRegistry) Get(name string) (ToolExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) GetAll() map[string]ToolExecutor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]ToolExecutor, len(r.tools))
	for k, v := range r.tools {
		result[k] = v
	}
	return result
}

func (r *ToolRegistry) RegisterAll(agent *AgentChat) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, tool := range r.tools {
		agent.RegisterTool(name, tool)
	}
}

// ValidateParams 按工具 Definition().Parameters 声明的 JSON Schema 做最小校验：
// required 必填 + string/integer/boolean/object 基本类型匹配。
// 工具参数全部由模型生成，不能当作可信输入：类型漂移（如把 document_id 写成字符串 "12"
// 或写成对象）会让工具内部的 `.(float64)` 断言静默拿到 0，进而绕过 space/document 守卫。
func ValidateParams(def ToolDefinition, params map[string]interface{}) error {
	schema := def.Parameters
	if len(schema) == 0 {
		return nil
	}

	for _, name := range requiredParamNames(schema) {
		if v, ok := params[name]; !ok || v == nil {
			return fmt.Errorf("缺少必填参数: %s", name)
		}
	}

	props, _ := schema["properties"].(map[string]interface{})
	for name, rawSchema := range props {
		v, ok := params[name]
		if !ok || v == nil {
			continue
		}
		ps, _ := rawSchema.(map[string]interface{})
		typ, _ := ps["type"].(string)
		if !matchesParamType(typ, v) {
			return fmt.Errorf("参数 %s 类型错误，期望 %s", name, typ)
		}
	}
	return nil
}

// requiredParamNames 兼容 []string（Go 侧声明）与 []interface{}（JSON 反序列化）两种写法。
func requiredParamNames(schema map[string]interface{}) []string {
	switch req := schema["required"].(type) {
	case []string:
		return req
	case []interface{}:
		names := make([]string, 0, len(req))
		for _, item := range req {
			if s, ok := item.(string); ok && s != "" {
				names = append(names, s)
			}
		}
		return names
	}
	return nil
}

// matchesParamType 只约束明确声明的类型，number/array/未声明一律放行，避免误伤 MCP 工具。
func matchesParamType(typ string, v interface{}) bool {
	switch typ {
	case "string":
		_, ok := v.(string)
		return ok
	case "integer":
		// encoding/json 会把所有 JSON 数字解码成 float64，这里同时要求确实没有小数部分。
		f, ok := v.(float64)
		return ok && !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Trunc(f)
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "object":
		_, ok := v.(map[string]interface{})
		return ok
	default:
		return true
	}
}
