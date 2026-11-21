// Package envname 让"改名后的环境变量"与"旧名字"并存。
//
// 为什么不能一把把 AIWIKI_* 直接改成 CITEBASE_*：viper 与 os.Getenv 对认不出的键
// **一律静默丢弃**。现存部署里那些 .env 与 start.bat 会全部失效，而它们的启动日志
// 照样打印一句"配置加载成功" —— 这正是本仓库在 compose 那条链上刚查出来过的失效形态
// （见 docs/release/launch-checklist.md 第 9 条）。
// 所以新名优先、旧名兜底，等一个发布周期后再摘掉旧名。
package envname

import "os"

// Getenv 按顺序取第一个非空的变量值。
// 调用方写成 Getenv("CITEBASE_DB_DSN", "AIWIKI_DB_DSN")：新名在前，旧名在后。
func Getenv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}
