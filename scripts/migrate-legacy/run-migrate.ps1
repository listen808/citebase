<#
.SYNOPSIS
    Citebase 旧库(mw_) -> 新库(aw_) 迁移编排：体检 -> 备份 -> 还原历史 dump -> 执行迁移 SQL -> 汇总 -> 清理。

.DESCRIPTION
    数据搬运全在 migrate-from-v1.sql 里，本脚本只负责把它安全地跑起来：
      1) 历史 dump 还原到独立库 citebase_legacy，绝不进生产库（否则 86 张 mw_* 表会落进 citebase）。
      2) 动手前先 mysqldump 备份目标库，出问题可回滚。
      3) 口令只写进临时 defaults-file，不出现在任何命令行参数里。
      4) 每条 SQL 先落成文件再执行（含容器内），绕开 PowerShell 5.1 的原生命令引号与
         GBK 重编码两个坑；中文报告由 mysql 客户端直接输出。
    Docker 模式下 mysql/mysqldump 都在容器内执行，本机无需装客户端。

.PARAMETER Mode
    Docker（默认，Citebase 由 docker compose 起）或 Local（本机 MySQL）。

.PARAMETER DbHost / DbPort
    仅 Local 模式生效：MySQL 服务端地址与端口（留空则用客户端默认，通常是 127.0.0.1:3306）。
    目标库和历史库必须在同一个实例上——迁移 SQL 用的是跨库 INSERT，Docker 模式忽略这两项。

.PARAMETER DumpFile
    历史库 mysqldump 文件；缺省取 databases\ 下最新的 dump-citebase-*.sql。

.EXAMPLE
    ./run-migrate.ps1
    # Docker 模式，root 口令读自仓库根目录 .env 的 MYSQL_ROOT_PASSWORD

.EXAMPLE
    ./run-migrate.ps1 -Mode Local -DbHost 192.0.2.10 -DbUser root -DumpFile .\databases\dump-citebase-202609271553.sql
    # 连远端 MySQL：库名/口令/主机端口全部写进临时 my.cnf，命令行上看不到密码

.EXAMPLE
    ./run-migrate.ps1 -Mode Local -MysqlBin 'C:\tools\mysql\current\bin\mysql.exe' -DbUser root -DropLegacy
    # 本机 MySQL，核对报告后删除 citebase_legacy

.EXAMPLE
    ./run-migrate.ps1 -DryRun
    # 只打印解析到的参数与步骤，不连数据库

.NOTES
    正文(.md)/图片/附件在旧服务器磁盘上，本脚本不处理；搬迁清单见 migrate-from-v1.sql 第 7 节。
    目标库必须已建好 91 张 aw_* 表（先跑过 Citebase 或导入过 databases/table.sql）。
#>
[CmdletBinding()]
param(
    [ValidateSet('Docker', 'Local')] [string] $Mode = 'Docker',
    [string] $Container = 'citebase-mysql',
    [string] $MysqlBin = 'mysql',
    [string] $DumpBin = 'mysqldump',
    [string] $DbUser = 'root',
    [string] $DbPassword,
    [string] $DbHost = '',
    [string] $DbPort = '',
    [string] $TargetDb = 'citebase',
    [string] $LegacyDb = 'citebase_legacy',
    [string] $DumpFile = '',
    [string] $BackupDir = '',
    [switch] $SkipBackup,
    [switch] $DropLegacy,
    [switch] $DryRun,
    [switch] $Force
)

$ErrorActionPreference = 'Stop'
$NoBom = New-Object Text.UTF8Encoding($false)
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent (Split-Path -Parent $ScriptDir)
$MigrateSql = Join-Path $ScriptDir 'migrate-from-v1.sql'
$Stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$Work = Join-Path $env:TEMP "citebase-migrate-$Stamp"
New-Item -ItemType Directory -Path $Work -Force | Out-Null
$script:Seq = 0
$script:Remote = @()

function Write-Step([string]$msg) { Write-Host "`n==== $msg ====" -ForegroundColor Cyan }
function Write-Dim([string]$msg) { Write-Host "       $msg" -ForegroundColor DarkGray }

# PS 5.1 会把原生命令的 stderr 变成终止错误（git/docker 常踩），这里统一降级并回收退出码
function Invoke-Native {
    param([string]$Exe, [string[]]$Argv)
    $saved = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $raw = & $Exe @Argv 2>&1
        $code = $LASTEXITCODE
    }
    finally { $ErrorActionPreference = $saved }
    $lines = @()
    foreach ($r in $raw) { $lines += ([string]$r) }
    return [pscustomobject]@{ Code = $code; Out = ($lines -join "`n") }
}

# ------------------------------------------------------------ 1. 参数与凭据
if (-not (Test-Path -LiteralPath $MigrateSql)) { throw "找不到迁移 SQL: $MigrateSql" }

if (-not $DumpFile) {
    $DumpFile = Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'databases') -Filter 'dump-citebase-*.sql' |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1 -ExpandProperty FullName
    if (-not $DumpFile) { throw "未指定 -DumpFile，且 databases\ 下没有 dump-citebase-*.sql" }
}
$DumpFile = (Resolve-Path -LiteralPath $DumpFile).Path

function Read-Secret([string]$prompt) {
    $sec = Read-Host $prompt -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
}

# 从 configs/config.yaml 的 dsn 反解连接参数：user:pass@tcp(host:port)/db?params
function Read-Dsn([string]$path) {
    if (-not (Test-Path -LiteralPath $path)) { return $null }
    $hit = Select-String -LiteralPath $path -Pattern '^\s*dsn:\s*"([^"]*@[^"]*)"' | Select-Object -First 1
    if (-not $hit) { return $null }
    $dsn = $hit.Matches[0].Groups[1].Value
    $at = $dsn.LastIndexOf('@')
    if ($at -lt 1) { return $null }
    $cred = $dsn.Substring(0, $at)
    $tail = $dsn.Substring($at + 1)
    $m = [regex]::Match($tail, '^tcp\(([^):]+)(?::(\d+))?\)/([^?;]+)')
    if (-not $m.Success) { return $null }
    $cu = $cred.Split(':')
    $pw = ''
    if ($cu.Count -gt 1) { $pw = ($cu[1..($cu.Count - 1)] -join ':') }
    return [pscustomobject]@{
        User     = $cu[0]
        Password = $pw
        Host     = $m.Groups[1].Value
        Port     = $m.Groups[2].Value
        Database = $m.Groups[3].Value
        Line     = $hit.LineNumber
    }
}

$dsnFile = Join-Path $RepoRoot 'configs\config.yaml'
if (Test-Path -LiteralPath $dsnFile) {
    $dsn = Read-Dsn $dsnFile
    if ($dsn) {
        $portShow = '3306'
        if ($dsn.Port) { $portShow = $dsn.Port }
        Write-Host "已解析连接参数: $($dsn.User)@$($dsn.Host):$portShow/$($dsn.Database)  <- configs\config.yaml:$($dsn.Line)" -ForegroundColor DarkGray
        if (-not $PSBoundParameters.ContainsKey('TargetDb')) { $TargetDb = $dsn.Database }
        if ($Mode -eq 'Local') {
            if (-not $DbHost) { $DbHost = $dsn.Host }
            if (-not $DbPort -and $dsn.Port) { $DbPort = $dsn.Port }
            if (-not $PSBoundParameters.ContainsKey('DbUser')) { $DbUser = $dsn.User }
            if (-not $DbPassword -and $dsn.Password) { $DbPassword = $dsn.Password }
        }
    }
    else {
        Write-Host '提示: configs\config.yaml 的 dsn 没解析出来，请手工传 -DbHost/-DbPort/-DbUser' -ForegroundColor Yellow
    }
}

# DryRun 只看计划：环境不满足时降级成警告，不阻断、也不生成口令文件
$Warn = @()
if ($Mode -eq 'Docker') {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        if ($DryRun) { $Warn += '本机没有 docker 命令：DryRun 照常出计划，实跑请改 -Mode Local' }
        else { throw 'Docker 模式需要 docker 在 PATH 中' }
    }
    elseif ((Invoke-Native 'docker' @('inspect', $Container)).Code -ne 0) {
        if ($DryRun) { $Warn += "容器 $Container 当前不可达：DryRun 不阻断" }
        else { throw "容器 $Container 不存在或未运行（用 -Container 指定实际容器名，docker ps 可查）" }
    }
    if (-not $DbPassword) {
        $envFile = Join-Path $RepoRoot '.env'
        if (Test-Path -LiteralPath $envFile) {
            $hit = Select-String -LiteralPath $envFile -Pattern '^\s*MYSQL_ROOT_PASSWORD\s*=' | Select-Object -First 1
            if ($hit) { $DbPassword = ($hit.Line -split '=', 2)[1].Trim().Trim('"') }
        }
        if (-not $DbPassword) {
            if ($DryRun) { $Warn += "口令未提供：实跑会读 .env 的 MYSQL_ROOT_PASSWORD，读不到则交互输入" }
            else { $DbPassword = Read-Secret "容器 $Container 内 MySQL root 口令" }
        }
    }
}
else {
    if (-not (Get-Command $MysqlBin -ErrorAction SilentlyContinue)) {
        if ($DryRun) { $Warn += "本机找不到 $MysqlBin：实跑需 -MysqlBin 给出 mysql.exe 路径" }
        else { throw "找不到 $MysqlBin：用 -MysqlBin 给出 mysql.exe 完整路径（mysqldump 同目录，或用 -DumpBin 指定）" }
    }
    # chocolatey 的 mysql-cli 包只有客户端没有 mysqldump，缺它就没法自动备份
    if (-not $SkipBackup -and -not (Get-Command $DumpBin -ErrorAction SilentlyContinue)) {
        if ($DryRun) { $Warn += "找不到 $DumpBin：实跑要么给 -DumpBin 完整路径，要么加 -SkipBackup（目标库是空库时备份没意义）" }
        else { throw "找不到 $DumpBin，无法备份目标库：给 -DumpBin 指定 mysqldump 路径，或确认目标库无需备份后加 -SkipBackup" }
    }
    if (-not $DbPassword) {
        if ($DryRun) { $Warn += '口令未提供：DryRun 跳过输入' }
        else { $DbPassword = Read-Secret "用户 $DbUser 的 MySQL 口令" }
    }
}
if (-not $BackupDir) { $BackupDir = $Work }
if (-not (Test-Path -LiteralPath $BackupDir)) { New-Item -ItemType Directory -Path $BackupDir -Force | Out-Null }

$CnfHost = Join-Path $Work 'my.cnf'
if (-not $DryRun) {
    # host/port 写进 defaults-file，命令行参数保持干净（mysql 与 mysqldump 都读 [client] 组）
    $cnf = "[client]`nuser=$DbUser`npassword=$DbPassword`ndefault-character-set=utf8mb4`n"
    if ($DbHost) { $cnf += "host=$DbHost`n" }
    if ($DbPort) { $cnf += "port=$DbPort`n" }
    [IO.File]::WriteAllText($CnfHost, $cnf, $NoBom)
    $DbPassword = $null   # 已落盘，内存副本清掉
}

# 库名替换：只动标识符形式（反引号/单引号包裹），保留注释里的 dump 文件名。
# citebase_legacy 作为整体先替换，剩下的 `citebase`/'citebase' 才是目标库。
$bt = [string][char]96
$sqlText = [IO.File]::ReadAllText($MigrateSql)
$sqlText = $sqlText.Replace('citebase_legacy', $LegacyDb)
$sqlText = $sqlText.Replace($bt + 'citebase' + $bt, $bt + $TargetDb + $bt)
$sqlText = $sqlText.Replace("'" + 'citebase' + "'", "'" + $TargetDb + "'")
$MigrateLocal = Join-Path $Work 'migrate-from-v1.sql'
[IO.File]::WriteAllText($MigrateLocal, $sqlText, $NoBom)

$CnfRemote = "/tmp/citebase-migrate-$Stamp.cnf"
$RunnerRemote = "/tmp/citebase-migrate-$Stamp.sh"
$DumperRemote = "/tmp/citebase-dump-$Stamp.sh"
$BackupRemote = "/tmp/citebase-backup-$Stamp.sql"

$prevEnc = $null
try { $prevEnc = [Console]::OutputEncoding; [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch { }

Write-Host 'Citebase 旧库 -> 新库 数据迁移' -ForegroundColor Green
Write-Dim "模式      : $Mode$(if ($Mode -eq 'Docker') { " (容器 $Container)" })"
if ($Mode -eq 'Local') {
    $ep = $(if ($DbHost) { $DbHost } else { 'localhost（客户端默认）' })
    if ($DbPort) { $ep += ":$DbPort" }
    Write-Dim "连接      : $ep  用户 $DbUser"
}
elseif ($DbHost -or $DbPort) {
    Write-Dim "说明      : Docker 模式在容器内连本机 socket，-DbHost/-DbPort 被忽略"
}
Write-Dim "历史 dump : $DumpFile"
Write-Dim "还原到库  : $LegacyDb"
Write-Dim "目标库    : $TargetDb"
Write-Dim "备份目录  : $BackupDir"
Write-Dim "临时目录  : $Work"
Write-Dim "步骤      : 体检 -> 备份 -> 还原 -> 迁移 -> 汇总 -> 清理"

if ($DryRun) {
    # 计划直接从迁移 SQL 现算，不写死说明文字：改了 SQL，DryRun 输出跟着变
    $live = ($sqlText -split "`n" | Where-Object { $_ -notmatch '^\s*--' }) -join "`n"
    $ins = @([regex]::Matches($live, 'INSERT IGNORE INTO `[^`]+`\.`(aw_[a-z0-9_]+)`') | ForEach-Object { $_.Groups[1].Value })
    $upd = @([regex]::Matches($live, '(?m)^UPDATE `[a-zA-Z0-9_]+`\.`(aw_[a-z0-9_]+)`') | ForEach-Object { $_.Groups[1].Value })
    $opt = @([regex]::Matches($sqlText, '(?m)^-- INSERT IGNORE INTO `[^`]+`\.`(aw_[a-z0-9_]+)`') | ForEach-Object { $_.Groups[1].Value })
    $mb = [math]::Round((Get-Item -LiteralPath $DumpFile).Length / 1MB, 1)

    Write-Host "`n==== 执行计划（DryRun，未连接数据库） ====" -ForegroundColor Yellow
    Write-Host "1/6 体检    目标库 $TargetDb 需已存在且 >= 91 张 aw_* 表；$LegacyDb 必须不存在"
    if ($SkipBackup) { Write-Host '2/6 备份    已用 -SkipBackup 关闭' }
    else { Write-Host "2/6 备份    mysqldump $TargetDb -> aw-backup-$TargetDb-$Stamp.sql（用同一个 my.cnf 可回滚）" }
    Write-Host "3/6 还原    CREATE DATABASE $LegacyDb; 再灌入 dump（$mb MB，预期 86 张 mw_* 表）"
    Write-Host "4/6 迁移    $($ins.Count) 张表写入 $TargetDb：$($ins -join ', ')"
    if ($upd.Count) { Write-Host "            $($upd.Count) 条 UPDATE 跨库回填：$(($upd | Sort-Object -Unique) -join ', ')（配置值 / 同名账号沿用旧密码）" }
    if ($opt.Count) { Write-Host "            默认跳过（SQL 里注释掉的可选段）：$($opt -join ', ')" }
    Write-Host "5/6 汇总    逐表打印 旧库行数 / 已写入 / 未写入，再打印 $TargetDb 计数与配置终态"
    if ($DropLegacy) { Write-Host "6/6 清理    核对后删除 $LegacyDb（-DropLegacy 会再问一次）" }
    else { Write-Host "6/6 清理    保留 $LegacyDb，核对报告后自行 DROP DATABASE" }
    Write-Host '不迁移    mw_privilege / mw_role / mw_role_privilege / mw_ai_prompt_template（与新库种子等价，见 SQL 第 7 节）'
    Write-Host '待人工    正文 .md / images / attachment 三个目录仍在旧服务器磁盘上，SQL 搬不到（见 SQL 第 7 节清单）'
    if ($Warn.Count) {
        Write-Host "`n---- 需要注意 ----" -ForegroundColor Magenta
        foreach ($w in $Warn) { Write-Host "  ! $w" -ForegroundColor Magenta }
    }
    Write-Host "`n临时目录  $Work（实跑时才会写 my.cnf）" -ForegroundColor DarkGray
    return
}
if (-not $Force) {
    if ((Read-Host "`n确认开始迁移？会向 $TargetDb 写入数据 [y/N]") -notmatch '^[yY]') { Write-Host '已取消。'; return }
}

# ------------------------------------------------------------ 2. SQL 执行原语
function New-SqlFile([string]$body) {
    $script:Seq++
    $p = Join-Path $Work ('q{0:D3}.sql' -f $script:Seq)
    [IO.File]::WriteAllText($p, "SET NAMES utf8mb4;`n$body`n", $NoBom)
    return $p
}

# 所有 SQL 都以文件形式执行：容器内用 sh 脚本做重定向，本机用 Start-Process 重定向 stdin
function Invoke-SqlFile {
    param([string]$Db, [string]$Path, [switch]$Table)
    $fmt = $(if ($Table) { 'table' } else { 'raw' })
    if ($Mode -eq 'Docker') {
        $leaf = 'citebase-in-' + $Stamp + '-' + $script:Seq + '.sql'
        $remote = "/tmp/$leaf"
        $cp = Invoke-Native 'docker' @('cp', $Path, "${Container}:$remote")
        if ($cp.Code -ne 0) { throw "docker cp 失败: $Path`n$($cp.Out)" }
        $script:Remote += $remote
        $r = Invoke-Native 'docker' @('exec', $Container, 'sh', $RunnerRemote, $remote, $(if ($Db) { $Db } else { '-' }), $fmt)
        if ($r.Code -ne 0) { throw "SQL 执行失败 (exit=$($r.Code)): $Path`n$($r.Out)" }
        return $r.Out
    }
    $script:Seq++
    $stdout = Join-Path $Work ('out{0:D3}.txt' -f $script:Seq)
    $a = @("--defaults-extra-file=$CnfHost", '--default-character-set=utf8mb4')
    if (-not $Table) { $a += '-N' }
    if ($Db) { $a += @('-D', $Db) }
    $p = Start-Process -FilePath $MysqlBin -ArgumentList $a -RedirectStandardInput $Path `
        -RedirectStandardOutput $stdout -RedirectStandardError "${stdout}.err" -NoNewWindow -Wait -PassThru
    if ($p.ExitCode -ne 0) {
        throw "SQL 执行失败 (exit=$($p.ExitCode)): $Path`n" + [IO.File]::ReadAllText("${stdout}.err")
    }
    return [IO.File]::ReadAllText($stdout, [Text.Encoding]::UTF8)
}

function Invoke-Sql {
    param([string]$Db, [string]$Sql, [switch]$Table)
    return Invoke-SqlFile -Db $Db -Path (New-SqlFile $Sql) -Table:$Table
}

function Get-Scalar([string]$Db, [string]$Sql) {
    $v = (Invoke-Sql -Db $Db -Sql $Sql) -replace '[^0-9]', ''
    $n = 0
    if (-not [int]::TryParse($v, [ref]$n)) { throw "标量结果解析失败 ('$v') <- $Sql" }
    return $n
}

$prevEnc = $null
try {
    if ($Mode -eq 'Docker') {
        # 容器内执行脚本：用 here-string 直写，避免 @() 元素拼接被解析成两个元素
        $runner = @"
#!/bin/sh
set -e
CNF=$CnfRemote
FILE="`$1"
DB="`$2"
FMT="`$3"
if [ "`$DB" = "-" ]; then DB=""; fi
OPT="-N"
if [ "`$FMT" = "table" ]; then OPT="--table"; fi
if [ -n "`$DB" ]; then
  mysql --defaults-extra-file="`$CNF" `$OPT --default-character-set=utf8mb4 -D "`$DB" < "`$FILE"
else
  mysql --defaults-extra-file="`$CNF" `$OPT --default-character-set=utf8mb4 < "`$FILE"
fi
"@
        [IO.File]::WriteAllText((Join-Path $Work 'run-sql.sh'), $runner + "`n", $NoBom)
        $dumper = @"
#!/bin/sh
set -e
mysqldump --defaults-extra-file=$CnfRemote --single-transaction --quick --default-character-set=utf8mb4 "`$1" > "`$2"
"@
        [IO.File]::WriteAllText((Join-Path $Work 'run-dump.sh'), $dumper + "`n", $NoBom)
        foreach ($pair in @(
                @($CnfHost, $CnfRemote),
                @((Join-Path $Work 'run-sql.sh'), $RunnerRemote),
                @((Join-Path $Work 'run-dump.sh'), $DumperRemote))) {
            $cp = Invoke-Native 'docker' @('cp', $pair[0], "${Container}:$($pair[1])")
            if ($cp.Code -ne 0) { throw "docker cp 失败: $($pair[0])`n$($cp.Out)" }
            $script:Remote += $pair[1]
        }
    }

    # ---- 1/6 体检
    Write-Step '1/6 目标库体检'
    $ver = (Invoke-Sql '' 'SELECT VERSION()') -replace '[\r\n]', ''
    Write-Host "服务端版本: $ver"
    if ($ver -like '5.*') {
        Write-Host "注意: MySQL 5.x 上跑 Citebase 属于降级使用（官方目标 8.0）；迁移 SQL 本身不含 8.0 专属语法，可继续。" -ForegroundColor Yellow
    }
    if ((Get-Scalar '' "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='$TargetDb'") -lt 1) {
        throw "目标库 $TargetDb 不存在：先 CREATE DATABASE，再启动 Citebase 或导入 databases/table.sql"
    }
    $aw = Get-Scalar '' "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$TargetDb' AND LEFT(table_name,3)='aw_'"
    if ($aw -lt 91) { throw "目标库只有 $aw 张 aw_* 表（预期 91）：请先启动 Citebase 完成 AutoMigrate，或导入 databases/table.sql" }
    Write-Host "目标库就绪：$aw 张 aw_* 表"
    if ((Get-Scalar '' "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='$LegacyDb'") -ge 1) {
        throw "历史库 $LegacyDb 已存在，为避免误删请先处理：DROP DATABASE $LegacyDb，或用 -LegacyDb 换名"
    }
    Write-Host "历史库 $LegacyDb 不存在，可安全还原"

    # ---- 2/6 备份
    if ($SkipBackup) {
        Write-Step '2/6 跳过备份（-SkipBackup）'
    }
    else {
        Write-Step '2/6 备份目标库'
        $BackupLocal = Join-Path $BackupDir "aw-backup-$TargetDb-$Stamp.sql"
        if ($Mode -eq 'Docker') {
            $d = Invoke-Native 'docker' @('exec', $Container, 'sh', $DumperRemote, $TargetDb, $BackupRemote)
            if ($d.Code -ne 0) { throw "mysqldump 失败，未改动任何数据，终止。`n$($d.Out)" }
            $cp = Invoke-Native 'docker' @('cp', "${Container}:$BackupRemote", $BackupLocal)
            if ($cp.Code -ne 0) { throw "docker cp 取回备份失败`n$($cp.Out)" }
            $script:Remote += $BackupRemote
        }
        else {
            $a = @("--defaults-extra-file=$CnfHost", '--single-transaction', '--quick', '--default-character-set=utf8mb4', $TargetDb)
            $p = Start-Process -FilePath $DumpBin -ArgumentList $a -RedirectStandardOutput $BackupLocal `
                -RedirectStandardError "${BackupLocal}.err" -NoNewWindow -Wait -PassThru
            if ($p.ExitCode -ne 0) { throw "mysqldump 失败 (exit=$($p.ExitCode))，终止。`n" + [IO.File]::ReadAllText("${BackupLocal}.err") }
        }
        $sizeMb = [math]::Round((Get-Item -LiteralPath $BackupLocal).Length / 1MB, 2)
        Write-Host "备份完成：$sizeMb MB"
        Write-Dim "备份文件: $BackupLocal"
        Write-Dim '回滚方法: 用同一个 my.cnf 把备份灌回目标库（见临时目录 my.cnf）'
    }

    # ---- 3/6 还原历史库
    Write-Step "3/6 还原历史 dump 到 $LegacyDb"
    Invoke-Sql '' "CREATE DATABASE ``$LegacyDb`` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci" | Out-Null
    $watch = [Diagnostics.Stopwatch]::StartNew()
    Invoke-SqlFile -Db $LegacyDb -Path $DumpFile | Out-Null
    Write-Host ("还原完成（{0:N0}s）" -f $watch.Elapsed.TotalSeconds)
    $mw = Get-Scalar '' "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$LegacyDb' AND LEFT(table_name,3)='mw_'"
    Write-Host "历史库表数量: $mw （预期 86）"
    if ($mw -lt 86) { throw "历史库只有 $mw 张表，dump 不完整或库名不对 —— 终止，目标库尚未被写入" }

    # ---- 4/6 迁移
    Write-Step '4/6 执行 migrate-from-v1.sql'
    $r = Invoke-SqlFile -Db $LegacyDb -Path $MigrateLocal -Table
    Write-Host $r

    # ---- 5/6 汇总
    Write-Step '5/6 目标库汇总'
    $sum = @(
        "SELECT 'users' AS item, COUNT(*) AS num FROM ``aw_user``",
        "UNION ALL SELECT 'spaces', COUNT(*) FROM ``aw_space``",
        "UNION ALL SELECT 'documents', COUNT(*) FROM ``aw_document``",
        "UNION ALL SELECT 'documents_alive', COUNT(*) FROM ``aw_document`` WHERE ``is_delete`` = 0",
        "UNION ALL SELECT 'versions', COUNT(*) FROM ``aw_document_version``",
        "UNION ALL SELECT 'attachments', COUNT(*) FROM ``aw_attachment``",
        "UNION ALL SELECT 'comments', COUNT(*) FROM ``aw_comment``",
        "UNION ALL SELECT 'collections', COUNT(*) FROM ``aw_collection``",
        "UNION ALL SELECT 'follows', COUNT(*) FROM ``aw_follow``",
        "UNION ALL SELECT 'members', COUNT(*) FROM ``aw_space_user``"
    ) -join "`n"
    Write-Host (Invoke-Sql -Db $TargetDb -Sql $sum -Table)

    # ---- 6/6 清理
    Write-Step '6/6 清理'
    if ($DropLegacy) {
        if ((Read-Host "确认删除历史库 $LegacyDb？请先核对上面的报告 [y/N]") -match '^[yY]') {
            Invoke-Sql '' "DROP DATABASE ``$LegacyDb``" | Out-Null
            Write-Host "已删除 $LegacyDb"
        }
        else { Write-Host '保留历史库。' }
    }
    else {
        Write-Host "历史库 $LegacyDb 已保留。核对报告后自行清理：DROP DATABASE ``$LegacyDb``;  或重跑本脚本加 -DropLegacy" -ForegroundColor Yellow
    }

    Write-Host "`n完成。别忘了 migrate-from-v1.sql 第 7 节：markdowns/images/attachment 目录搬迁、重启重建全文索引、重跑 AI 分析。" -ForegroundColor Green
}
finally {
    if ($prevEnc) { try { [Console]::OutputEncoding = $prevEnc } catch { } }
    if ($Mode -eq 'Docker' -and $script:Remote.Count -gt 0) {
        $rm = 'rm -f ' + ($script:Remote -join ' ')
        Invoke-Native 'docker' @('exec', $Container, 'sh', '-c', $rm) | Out-Null
    }
    Remove-Item -LiteralPath $CnfHost -Force -ErrorAction SilentlyContinue
    Write-Host "临时目录: $Work （口令文件 my.cnf 已删除，其余 SQL/输出保留备查）" -ForegroundColor DarkGray
}
