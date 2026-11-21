# import_oracle_ddl 用法

解析 Oracle / MySQL DDL 文件，把每张表的建表语句导入为一篇 Citebase 文档，供 Text-to-SQL 做 schema 检索。

## 前提

- 后端服务已在 `localhost:3001` 运行
- 项目根目录存在 `data/.jwt_secret`（首次启动自动生成）
- 从项目根目录执行（依赖相对路径 `data/.jwt_secret`）

## 语法

```powershell
go build -o import_oracle_ddl.exe ./tools/import_oracle_ddl

.\import_oracle_ddl.exe <ddl_file> [space_name]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `ddl_file` | 是 | DDL 文件路径（支持 `.sql` / `.txt`） |
| `space_name` | 否 | 自定义空间名称。不传则自动从文件中检测 schema 名，拼接为 `{SCHEMA}数据字典` |

## 示例

```powershell
# 自动检测 schema 名作为空间名
.\import_oracle_ddl.exe .\my_schema.sql            # → MYSCHEMA数据字典

# 指定空间名
.\import_oracle_ddl.exe .\my_schema.sql 业务数据字典
```

> DDL 样本请自备。仓库不收录任何第三方生产库结构 —— 这类文件通常属于他人资产，
> 也不要提交到本仓库。

## DDL 文件格式要求

文件需包含带 schema 前缀的节头注释，正则匹配：`-- {schema}.{table} definition`

支持两种格式（大小写不敏感）：

**Oracle 格式**（`.sql`）：
```sql
-- MYSCHEMA.AGREEMENT definition

CREATE TABLE "MYSCHEMA"."AGREEMENT"
   ( "AGREEMENT_ID" NUMBER(32,0) NOT NULL ENABLE, ... )
```

**MySQL 格式**（`.txt`）：
```sql
-- myschema.audit_log definition

CREATE TABLE `audit_log` (
  `content_id` decimal(16,0) NOT NULL COMMENT '序列', ...
)
```

## 执行流程

1. 解析 DDL 文件，提取所有 `CREATE TABLE` 语句（表名统一大写，确保多文件一致性）
2. 自动检测 schema 名作为默认空间名（`{SCHEMA}数据字典`），可通过命令行参数覆盖
3. 在 API 中查找或创建目标空间（`visit_level: public`）
4. 在空间下创建父目录 `表结构DDL`
5. 逐表创建文档（每张表一个 Markdown 文档，包含完整 DDL）
6. 已存在的表自动更新内容
7. 遇到 409 冲突（跨文件导入同名表）自动查找并更新现有文档
8. 全部完成后刷新 Text-to-SQL 表缓存

## 注意事项

- 表名统一转为大写，避免 `.sql`（大写）和 `.txt`（小写）混合导入时的重复冲突
- 每张表之间间隔 300ms，防止 API 限流
- 遇到 429 限流自动等待 10 秒重试
- 遇到 409 冲突自动查找已有文档并更新（而非报错中断）
- 同一空间多次导入会**更新**已有文档，不会重复创建
