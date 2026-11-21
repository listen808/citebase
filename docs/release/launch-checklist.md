# 上线检查清单

> 按顺序过一遍；每一项都写"怎么验"，不要只看"配了没有"。
> 安装包场景同时看 [windows-packaging.md](windows-packaging.md)。

## 1. 运行环境

- [ ] Go 不需要装在目标机（单二进制）；但反向代理、TLS、MySQL 8 要先就位。
- [ ] 端口：默认 `3001`。**它只能改 `configs/config.yaml` 的 `server.port`，环境变量覆盖不到**（viper 只绑定白名单键），改了端口记得同步反代与健康检查。
- [ ] 时区与日志目录：`data/logs` 必须在运行账号下可写；轮转是 `lumberjack`，确认磁盘余量。

## 2. 凭据与环境变量

| 变量 | 要求 | 不配的后果 |
|---|---|---|
| `CITEBASE_JWT_SECRET`（旧名 `AIWIKI_JWT_SECRET`） | ≥32 字节随机串，**每次部署唯一** | 用示例值等于令牌可伪造 |
| 数据库 `MYSQL_*` | 必须显式给，代码里**没有默认口令** | 起不来（这是有意的 fail-fast） |
| `CITEBASE_ENCRYPT_KEY` | 多副本时必须显式且一致 | 各副本解不开对方的加密字段 |
| `CITEBASE_ENV=production` | 生产建议显式 | 默认按开发模式放宽检查 |

`.env` 只给 Docker Compose 用；用二进制部署时写 `configs/config.yaml` 或系统环境变量。**不要把真实凭据写进 `.env.example` / `config_sample.yaml` / 镜像层。**

## 3. 数据库

- [ ] 库是**空的**：`databases/table.sql` 每条 `CREATE TABLE` 前带 `DROP TABLE`，对已有库执行会清数据；已有库升级只靠启动时 AutoMigrate。
- [ ] 首次启动会跑迁移并打印一次 admin 初始密码（**只打印一次**，去日志里找）。
- [ ] 多副本：只让一个实例跑迁移（其余设 `CITEBASE_SKIP_MIGRATE=true`），并确认关键列核对通过（它不受 SKIP_MIGRATE 影响）。
- [ ] 跑完立刻做一次备份：`data/backups` 或 `internal/utils/dbdump` 的导出；确认能回灌到另一个空库。

## 4. 反向代理（最容易漏的一项）

- [ ] **必须配 `server.trusted_proxies`**（反代/负载均衡的真实网段）。不配时 Gin 不信任 `X-Forwarded-For`，所有请求都记成同一个 IP：全局限流 300/分钟变成**全站共享**，一个人刷接口会把所有人打成 429。
- [ ] 转发头：`X-Forwarded-For`、`X-Forwarded-Proto`、`Host`。
- [ ] WebSocket：`/api/v1/collab/ws` 需要 `Upgrade`/`Connection` 透传；反代要关掉对该路径的 buffering。
- [ ] 上传体积：反代的 `client_max_body_size` 与后端 multipart 限制对齐（默认按 50 MB 量级配置）。
- [ ] 超时：AI 流式接口（SSE）需要较长的读超时，别用默认 60s 掐断长回答。
- [ ] CORS：`server.allowed_origins` 填**实际域名**（同源请求自动放行）；不要写内网 IP 段，避免拓扑泄漏。

## 5. 安全收口

- [ ] 首次登录后改掉 admin 密码；安装向导的三个接口（`/api/v1/install/state`、`/install/init-db`、`/install/init-system`）在初始化完成后应返回"已初始化"，确认它不是可重复执行的状态。
- [ ] 关掉不需要的开放能力：`swagger_open`、外部 API Key（`/open/api-keys`）、SSO/OIDC 配置。
- [ ] TLS 终结在反代；后端只监听内网。
- [ ] 确认日志里不出现口令/令牌/连接串（抽查启动日志与一次登录）。

## 6. 可观测性

- [ ] `GET /api/v1/health` 返回 200（反代健康检查指它，不要指 `/`）。
- [ ] `/metrics` 已接入采集（若有 Prometheus）；确认它没有暴露到公网。
- [ ] 日志级别：生产用 `info`，排障临时升 `debug`（`CITEBASE_LOG_LEVEL`），事后改回。

## 7. 发布前门禁

```bash
make ci                                  # lint + test + build + 前端构建 + check-docs
cd web && npm run ci                     # lint → type-check → check:style(7) → i18n-msg → vitest → build
node tools/validate-doc-links.mjs        # 文档索引不指空（make check-docs 已含）
```

没有远端 CI 时，这三条就是唯一强制（见 [../decision-log/2026-10-07-ci-and-module-path.md](../decision-log/2026-10-07-ci-and-module-path.md)）。

## 8. 冒烟（上线后 10 分钟）

- [ ] 登录 / 退出 / 刷新令牌（`/auth/refresh` 有独立的 30/分钟限流，别用压测打它）；
- [ ] 建空间 → 建文档 → 编辑保存 → 历史版本可见；
- [ ] 搜索：中文关键词有结果（分词词典缺失时是**静默空结果**，这里必须人眼验）；
- [ ] AI 问答：流式返回、引用可点、超时能优雅报错；
- [ ] 上传附件 → 预览 → 下载；
- [ ] 协作编辑：两个浏览器同时改同一段，能看到对方光标；
- [ ] 备份 → 回灌到测试库 → 抽查三张表行数；
- [ ] 移动端（<641px）：侧栏抽屉、表格横向滚动、按钮不重叠。

## 9. 回滚

- [ ] 保留上一个版本的二进制与 `configs/`（配置变更经常是故障源）；
- [ ] 数据库只前进不后退：回滚二进制前确认它兼容当前表结构（新列已 AutoMigrate 时，旧二进制忽略新列通常可回，但**删列/改类型不可回**）；
- [ ] 回滚后立刻验证 `/api/v1/health` 与登录。

## 10. 上线后一周

- [ ] 看 429 比例：偏高说明限流档或 `trusted_proxies` 有问题；
- [ ] 看 5xx 与 `error_leak_test` 类日志：有无 SQL 原文漏到响应；
- [ ] 看磁盘：`data/index`、`data/logs`、`data/backups` 的增长速度；
- [ ] 把本次上线踩到的坑补进 `docs/decision-log/`（带日期），别只留在聊天记录里。
