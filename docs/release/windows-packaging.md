# Windows 打包：便携 zip 与安装器

- 入口：`scripts/package-windows.bat`（唯一构建链）
- 产物：`dist/citebase/`（暂存目录）→ 仓库根 `citebase-windows.zip` → 若装了 Inno Setup 6，再产出 `Citebase-Setup-<version>.exe`

## 1. 前提

- Go 1.26+、Node `^20.19.0 || >=22.12.0`（脚本开头会 `where go` / `where node`，缺一个就停）；
- 可选：Inno Setup 6。`ISCC.exe` 按顺序查找 `%ISCC_PATH%` → `%LOCALAPPDATA%\Programs\Inno Setup 6\` → `%ProgramFiles(x86)%\Inno Setup 6\` → `%ProgramFiles%\Inno Setup 6\`；找不到就只出 zip，不报错。

## 2. 七个步骤

| 步 | 做什么 |
|---|---|
| 0 | 检查 Go / Node |
| 1 | 清掉上一次的 `dist/` 与根目录 zip |
| 2 | 建暂存目录：`configs/`、`web/dist/`、`databases/`、`tools/import_oracle_ddl/`、`data/{index,dict/zh,markdowns,images,attachment}` |
| 3 | `go build -ldflags="-s -w -X .../internal/version.Commit=<short hash>"` → `citebase.exe`（无 git 时 Commit=`dev`，**不编一个假 hash**） |
| 4 | `cd web && npm install`（无 `node_modules` 时）+ `npm run build`，校验 `web/dist/index.html` 存在 |
| 5 | 拷资源：`web/dist`、`databases/table.sql` + `data.sql`、`configs/config_sample.yaml` → `config.yaml`、`build/DATABASE-SETUP.sql`、gse 中文词典、导入工具、生成 `start.bat` / `stop.bat` / `README.txt` |
| 6 | `Compress-Archive` → `citebase-windows.zip` |
| 7 | 从 `web/package.json` 的 **`version`** 取版本（`node -p "require(...).version"`，不用 findstr——dependencies 里也有 version 键），`ISCC /DStageDir=dist\citebase /DBuildVersion=<version> /Odist\Citebase-Setup-<version>.exe build\installer.iss` |

版本号只有一处：`web/package.json` 的 `version`，它必须与 `internal/version/version.go` 的 `Version` const 一致（`version_test.go` 钉住）。`package-windows.bat` 只注入 `Commit`，不注入 `Version`——`-X` 注入不进 const，会被静默忽略。

## 3. 包内容

```
citebase/
├── citebase.exe
├── start.bat / stop.bat
├── README.txt                 # 脚本生成
├── DATABASE-SETUP.sql         # 三步建库说明（build/ 下的模板，原样拷入）
├── configs/config.yaml        # 由 config_sample.yaml 拷来，需改 DSN
├── databases/table.sql        # 含 DROP TABLE，只能对空库执行
├── databases/data.sql
├── web/dist/**                # 前端构建产物
├── tools/import_oracle_ddl/*.go|*.md
└── data/{index,dict/zh,markdowns,images,attachment}
```

## 4. 五个必须知道的坑（都在代码注释里写着）

1. **中文分词词典必须随包走**。gse 的 `LoadDict()` 无参时解析的是**构建机** module cache 的绝对路径；换一台机器就找不到，症状是"搜索中文返回空结果"（静默降级，不报错）。脚本用 `go list -m -f "{{.Dir}}" github.com/go-ego/gse` 定位后拷 `s_1.txt` / `t_1.txt`；定位失败会 WARN 并提示手工补。
2. **快捷方式必须指向 `start.bat`，不能指 `citebase.exe`**。静态资源与前端是按 CWD 相对路径读的，exe 若被从桌面启动，CWD 是桌面而不是安装目录。`start.bat` 第一件事是 `cd /d %~dp0`，所以它成立。
3. **默认装到 `%LOCALAPPDATA%\Citebase`，不是 Program Files**。`data/` 下的日志、备份、索引、上传件与运行期密钥都是相对运行目录写入的，Program Files 默认不可写。安装器用 `PrivilegesRequired=lowest`（per-user，不需要 UAC）。
4. **卸载默认保留 `data/`**（索引、上传、日志），删除要用户显式确认。卸载器只关自己的进程，不碰用户的浏览器/杀软（`CloseApplications=yes` 限定范围）。
5. **Inno 脚本两个格式约束**：`build/installer.iss` 必须存为 **UTF-8 with BOM**；`[Code]` 段必须**全 ASCII**——Pascal Script 编译器对非 ASCII 解析不可靠（中文会报 `Unknown identifier`）。面向用户的中文说明由 bat 生成为包内 `DATABASE-SETUP.sql` 与 `README.txt`，安装器只引用文件路径。

## 5. 发布前自检

- [ ] `dist/citebase/citebase.exe` 能启动，`start.bat` 从任意目录双击都能起（验证第 2 条坑）；
- [ ] `web/dist/index.html` 存在，页面能打开且静态资源 200；
- [ ] `configs/config.yaml` 在包里且 DSN 为空/示例值（不要把开发机 DSN 打进去）；
- [ ] `data/dict/zh/s_1.txt`、`t_1.txt` 在包里，中文搜索有结果（验证第 1 条坑）；
- [ ] 空库执行 `DATABASE-SETUP.sql` 里的三步，首次启动日志里有 admin 初始密码（只打印一次）；
- [ ] zip 解压到另一台机器能跑（不依赖构建机的 module cache / Node）；
- [ ] 安装器：装到 `%LOCALAPPDATA%`、桌面快捷方式指向 `start.bat`、卸载后 `data/` 仍在。

## 6. 已知漂移（发布时顺手核对）

`build/DATABASE-SETUP.sql` 是手写模板，里面写"全部 **94** 张表"；而 `databases/table.sql` 当前实测 **96** 条 `CREATE TABLE`（97 条 `DROP TABLE`，`table.sql` 由 `go run ./tools/genddl` 生成）。发布前用 `grep -c "CREATE TABLE" databases/table.sql` 核一遍并同步模板里的数字——这类"文档里的数字"不会自己变红。
