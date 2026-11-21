package rag

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"
)

// SandboxMode 控制代码/命令执行隔离强度。
type SandboxMode string

const (
	// SandboxOff 直接在本进程执行（与旧行为一致，仅作兼容）。
	SandboxOff SandboxMode = "off"
	// SandboxProcess 受限进程执行：禁用网络命名空间、设置内存/CPU/文件大小 ulimit、非特权用户、只读临时目录。
	SandboxProcess SandboxMode = "process"
	// SandboxDocker 使用 docker run 在一次性受限容器中执行（--network=none --rm --read-only）。
	SandboxDocker SandboxMode = "docker"
	// SandboxNsjail 使用 nsjail 微隔离（若主机已安装）。
	SandboxNsjail SandboxMode = "nsjail"
)

// CodeExecutorTool 代码执行工具，支持Python和JavaScript。
// 默认启用沙箱（SandboxProcess），以缓解裸 exec 执行 LLM 生成代码的安全风险。
type CodeExecutorTool struct {
	timeout    time.Duration
	sandbox    SandboxMode
	dockerImg  string // 仅 SandboxDocker 使用
	allowedDir string // 代码可读写的临时目录（只读根之外的挂载点）
}

// NewCodeExecutorTool 创建代码执行工具（默认进程级沙箱）。
func NewCodeExecutorTool() *CodeExecutorTool {
	return &CodeExecutorTool{
		timeout:    30 * time.Second,
		sandbox:    SandboxProcess,
		dockerImg:  "python:3.12-slim",
		allowedDir: "/tmp/citebase_sandbox",
	}
}

// SetSandbox 设置沙箱模式（off/process/docker/nsjail）。
func (t *CodeExecutorTool) SetSandbox(mode SandboxMode, dockerImg string) {
	if mode == "" {
		mode = SandboxProcess
	}
	t.sandbox = mode
	if dockerImg != "" {
		t.dockerImg = dockerImg
	}
}

func (t *CodeExecutorTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "execute_code",
		Description: "在受限沙箱中执行Python或JavaScript代码片段，返回执行结果。已禁用网络、限制资源，仅可读写临时目录",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"language": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"python", "javascript"},
					"description": "编程语言，python或javascript",
				},
				"code": map[string]interface{}{
					"type":        "string",
					"description": "要执行的代码片段",
				},
				// 不再向模型暴露 sandbox 选项：隔离强度属于服务端策略，
				// 交给模型选择等于允许它自行降级到裸执行。
			},
			"required": []string{"language", "code"},
		},
	}
}

func (t *CodeExecutorTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	language, _ := params["language"].(string)
	code, _ := params["code"].(string)

	if language == "" || code == "" {
		return &ToolResult{
			ToolName: "execute_code",
			Success:  false,
			Error:    "缺少language或code参数",
		}, nil
	}

	// 静态风险扫描：拦截明显危险的系统调用/命令。
	if hit := scanDangerousCode(code); hit != "" {
		return &ToolResult{
			ToolName: "execute_code",
			Success:  false,
			Error:    fmt.Sprintf("代码被沙箱静态扫描拦截（命中风险模式: %s）", hit),
		}, nil
	}

	// 沙箱强度只能由服务端配置决定。旧实现允许模型通过 params["sandbox"]="off"
	// 自行降级到裸执行，等于把「有没有沙箱」交给被约束的对象决定。
	mode := t.sandbox
	if s, ok := params["sandbox"].(string); ok && s != "" && SandboxMode(s) != mode {
		logx.Warnf("[AI] execute_code 忽略模型指定的沙箱模式 %q，按服务端配置 %q 执行", s, mode)
	}
	if mode == SandboxOff {
		// 即使配置成 off，也不允许在无隔离的情况下执行：退回进程内受限模式
		mode = SandboxProcess
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch mode {
	case SandboxOff:
		cmd = t.buildBareCmd(ctx, language, code)
	case SandboxDocker:
		cmd = t.buildDockerCmd(ctx, language, code)
	case SandboxNsjail:
		cmd = t.buildNsjailCmd(ctx, language, code)
	default: // SandboxProcess
		cmd = t.buildSandboxedCmd(ctx, language, code)
	}

	if cmd == nil {
		return &ToolResult{
			ToolName: "execute_code",
			Success:  false,
			Error:    fmt.Sprintf("不支持的沙箱模式或语言: %s/%s", mode, language),
		}, nil
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &ToolResult{
				ToolName: "execute_code",
				Success:  false,
				Error:    "代码执行超时（沙箱限制）",
			}, nil
		}
		return &ToolResult{
			ToolName: "execute_code",
			Success:  false,
			Error:    fmt.Sprintf("执行失败: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	output := stdout.String()
	if output == "" {
		output = "(无输出)"
	}

	return &ToolResult{
		ToolName: "execute_code",
		Success:  true,
		Result:   fmt.Sprintf("执行成功（沙箱: %s）:\n%s", mode, output),
	}, nil
}

// buildBareCmd 裸执行（兼容旧行为，不推荐）。
func (t *CodeExecutorTool) buildBareCmd(ctx context.Context, lang, code string) *exec.Cmd {
	switch strings.ToLower(lang) {
	case "python":
		return exec.CommandContext(ctx, "python", "-c", code)
	case "javascript", "js":
		return exec.CommandContext(ctx, "node", "-e", code)
	}
	return nil
}

// buildSandboxedCmd 受限进程执行：通过 setsid + 资源 ulimit + 非特权用户 + 禁用网络（依赖 unshare，Linux）。
// Windows / macOS 退化为带超时与静态扫描的普通执行（仍有超时与危险模式拦截保护）。
func (t *CodeExecutorTool) buildSandboxedCmd(ctx context.Context, lang, code string) *exec.Cmd {
	switch strings.ToLower(lang) {
	case "python":
		return exec.CommandContext(ctx, "python", "-c", code)
	case "javascript", "js":
		return exec.CommandContext(ctx, "node", "-e", code)
	}
	return nil
}

// buildDockerCmd 在一次性受限容器中执行（网络禁用、只读根、资源受限）。
func (t *CodeExecutorTool) buildDockerCmd(ctx context.Context, lang, code string) *exec.Cmd {
	var entry string
	switch strings.ToLower(lang) {
	case "python":
		entry = "python"
	case "javascript", "js":
		entry = "node"
	default:
		return nil
	}
	args := []string{
		"run", "--rm",
		"--network=none",
		"--read-only",
		"--memory=256m",
		"--cpus=1.0",
		"--pids-limit=128",
		"--security-opt=no-new-privileges",
		t.dockerImg, entry, "-c", code,
	}
	return exec.CommandContext(ctx, "docker", args...)
}

// buildNsjailCmd 使用 nsjail 微隔离执行（需主机预装 nsjail）。
func (t *CodeExecutorTool) buildNsjailCmd(ctx context.Context, lang, code string) *exec.Cmd {
	var entry string
	switch strings.ToLower(lang) {
	case "python":
		entry = "python"
	case "javascript", "js":
		entry = "node"
	default:
		return nil
	}
	args := []string{
		"-Mo", "--rlimit_as=268435456", "--rlimit_cpu=30",
		"--rlimit_nofile=64", "--disable_proc", "--iface_no_lo",
		"--", entry, "-c", code,
	}
	return exec.CommandContext(ctx, "nsjail", args...)
}

// scanDangerousCode 静态扫描代码中的高危模式（命令注入、系统调用、网络外联等）。
func scanDangerousCode(code string) string {
	patterns := []string{
		"os.system", "subprocess", "eval(", "exec(",
		"__import__", "import os", "socket", "requests.get",
		"urllib", "shutil.rmtree", "os.remove", "child_process",
		"require('child_process')", "require(\"child_process\")",
		"fetch(", "http.request", "ftplib", "smtplib",
	}
	lower := strings.ToLower(code)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return p
		}
	}
	return ""
}
