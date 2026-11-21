package main

// aidump —— Citebase 整库导出工具（纯 Go，不依赖 mysqldump/mariadb-dump 外部二进制）。
//
// 为什么要有它：internal/service/admin/backup.go 原先走 exec.LookPath("mysqldump")，
// 而只装了数据库客户端的机器（例如 chocolatey 的 mysql-cli 只有 mysql.exe）上根本没有
// 那个二进制，"系统管理 → 备份" 就必然失败。真正干活的引擎在 internal/utils/dbdump，
// 备份服务与本工具共用同一份实现，这里只负责命令行与配置解析。
//
// 用法（在仓库根目录）：
//
//	go run ./tools/aidump -out citebase.sql                  # DSN 取自 configs/config.yaml
//	go run ./tools/aidump -db citebase_legacy -out legacy.sql
//	go run ./tools/aidump -tables aw_user,aw_document -schema-only -quiet
//	go run ./tools/aidump -out citebase.sql -add-drop-table  # 产出可直接覆盖回灌的脚本
//	AIWIKI_DB_DSN="user:pw@tcp(h:3306)/db" go run ./tools/aidump -out x.sql
//
// DSN 解析优先级：-dsn > AIDUMP_DSN > 仓库配置加载器（configs/config.yaml 或 AIWIKI_DB_DSN）。
// 输出走文件时进度与汇总打到 stderr，stdout 只留 SQL，方便 `go run ./tools/aidump | mysql db`。
// 默认不写 DROP TABLE：导出文件回灌到已有库时，应当是"补行"而不是"清空"。

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/listen808/citebase/internal/utils/config"
	"github.com/listen808/citebase/internal/utils/dbdump"
)

type options struct {
	dsn          string
	schema       string
	out          string
	tables       string
	schemaOnly   bool
	noCreateInfo bool
	addDrop      bool
	chunk        int
	quiet        bool
}

func main() {
	opt := parseFlags()
	if err := run(opt); err != nil {
		fmt.Fprintln(os.Stderr, "✗", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	var o options
	flag.StringVar(&o.dsn, "dsn", os.Getenv("AIDUMP_DSN"), "go-sql-driver 格式 DSN；缺省读 AIDUMP_DSN 环境变量，再退回 configs/config.yaml")
	flag.StringVar(&o.schema, "db", "", "要导出的库名（缺省从 DSN 解析）")
	flag.StringVar(&o.out, "out", "-", "输出文件路径；'-' 表示标准输出")
	flag.StringVar(&o.tables, "tables", "", "只导这些表（逗号分隔，支持 aw_* 形式的前缀通配）；缺省全库")
	flag.BoolVar(&o.schemaOnly, "schema-only", false, "只导结构，不导数据")
	flag.BoolVar(&o.noCreateInfo, "no-create-info", false, "只导数据，不导 CREATE TABLE")
	flag.BoolVar(&o.addDrop, "add-drop-table", false, "在每张表的 CREATE 前加 DROP TABLE IF EXISTS（回灌会先删表）")
	flag.IntVar(&o.chunk, "chunk", dbdump.DefaultChunk, "每条 INSERT 最多多少行")
	flag.BoolVar(&o.quiet, "quiet", false, "不打印逐表进度")
	flag.Parse()

	if o.chunk < 1 {
		fmt.Fprintln(os.Stderr, "✗ -chunk 必须 >= 1")
		os.Exit(2)
	}
	if o.schemaOnly && o.noCreateInfo {
		fmt.Fprintln(os.Stderr, "✗ -schema-only 与 -no-create-info 互斥")
		os.Exit(2)
	}
	return o
}

func run(o options) error {
	dsn, err := resolveDSN(o.dsn)
	if err != nil {
		return err
	}

	w, closeFn, err := openOut(o.out)
	if err != nil {
		return err
	}
	defer func() { _ = closeFn() }()

	schema := o.schema
	if schema == "" {
		if schema, err = dbdump.SchemaFromDSN(dsn); err != nil {
			return err
		}
	}

	st, err := dbdump.Dump(dsn, w, dbdump.Options{
		Tables:       splitTables(o.tables),
		Schema:       schema,
		SchemaOnly:   o.schemaOnly,
		NoCreateInfo: o.noCreateInfo,
		AddDropTable: o.addDrop,
		Chunk:        o.chunk,
		Progress: func(table string, n int) {
			if !o.quiet {
				fmt.Fprintf(os.Stderr, "  %-36s %8d 行\n", table, n)
			}
		},
		Warn: func(msg string) { fmt.Fprintln(os.Stderr, "!", msg) },
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "完成：%s —— %d 张表，%d 行，%.2f MB\n",
		schema, st.Tables, st.Rows, float64(st.Bytes)/1024/1024)
	return nil
}

// resolveDSN 按 -dsn > AIDUMP_DSN > 应用配置的顺序取 DSN。
// 复用应用的配置加载器：读同一个 configs/config.yaml，也认 AIWIKI_DB_DSN，
// 免得工具自己再写一份 YAML 解析、与应用读到不同的库。
func resolveDSN(flagDSN string) (string, error) {
	if flagDSN != "" {
		return flagDSN, nil
	}
	config.InitConfig()
	if config.AppConfig.Database.DSN == "" {
		return "", errors.New("拿不到 DSN：给 -dsn、设 AIDUMP_DSN，或准备 configs/config.yaml")
	}
	return config.AppConfig.Database.DSN, nil
}

func splitTables(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func openOut(path string) (io.Writer, func() error, error) {
	if path == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("创建输出文件失败: %w", err)
	}
	return f, f.Close, nil
}
