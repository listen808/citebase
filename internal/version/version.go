// Package version 是后端版本号的唯一落点。
//
// 为什么要专门开一个包：迁移之前 "2.0.0" 这个字面量抄在 6 个地方
// （web/package.json、databases/data.sql 的种子行、internal/repository/db.go 的 defaultConfigs、
// internal/ai/mcp 的三个 initialize、docs/swagger 的 info.version），
// 而且 swagger 那份还写成了 "2.0" —— 没有任何一台工装会发现它们分叉，
// 因为每一处单独看都"像是对的"。现在只准这里写一次，其余要么引用它，要么由
// version_test.go 逐处比对（不一致即红）。
package version

import (
	"runtime"
	"strings"
)

// Version 必须与 web/package.json 的 version 字段完全一致（去掉前导 v）。
// 改版本时只改这一处 + package.json，其余由门禁校验。
const Version = "2.0.0"

// 下面三个由构建期 -ldflags -X 注入；不设时保留这里的默认值。
// 默认值刻意写成 "dev"/""，而不是抄一个真实 commit —— "没注入"必须一眼看得出来。
var (
	Commit    = "dev"
	BuildDate = ""
)

// Full 给日志与启动横幅用：2.0.0 (commit dev, built , go1.26.4)。
func Full() string {
	var b strings.Builder
	b.WriteString(Version)
	b.WriteString(" (commit ")
	b.WriteString(Commit)
	if BuildDate != "" {
		b.WriteString(", built ")
		b.WriteString(BuildDate)
	}
	b.WriteString(", ")
	b.WriteString(runtime.Version())
	b.WriteString(")")
	return b.String()
}

// Build 是构建元数据，只给管理端诊断接口用（不对外公开 commit）。
type Build struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date,omitempty"`
	GoVersion string `json:"go_version"`
}

func BuildInfo() Build {
	return Build{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
	}
}
