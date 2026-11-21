package main

// migrate_content —— P1 正文回填：把 data/markdowns 的正文按 document_id 写入 aw_document_content。
//
// 用法（仓库根目录）：
//
//	go run ./tools/migrate_content                 # dry-run：只算映射与统计，不写库
//	go run ./tools/migrate_content -apply          # 真正写库（分批 100 篇一个事务）
//	go run ./tools/migrate_content -dir ./data/markdowns
//	go run ./tools/migrate_content -dsn "root:pw@tcp(h:3306)/citebase"
//
// 规则（与 docs/citebase-割接方案与演练报告.md 一致）：
//   - 映射主键是 document_id，不做目录遍历，避免同名/软删/改名错配；
//   - 先按现有路径算法（workspace.ComputePageFilePath）精确命中，记 exact；
//   - 未命中再按「原始名称、不净化 /」回退，记 raw_name（doc 339/343/489）；
//   - 页面类型缺正文 → 中止（除非 -allow-missing），不允许写空正文；
//   - 目录类型缺 README → 允许空正文，记 dir_no_readme；
//   - 正文按字节原样入库，不 trim、不改换行。

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
	"gorm.io/gorm"
)

type result struct {
	DocID     int
	Name      string
	MatchType string
	PageFile  string
	Size      int
	SHA256    string
	Content   string
}

func main() {
	apply := flag.Bool("apply", false, "真正写库；缺省 dry-run")
	dsn := flag.String("dsn", "", "覆盖 configs/config.yaml 的 DSN")
	docDir := flag.String("dir", "", "覆盖 storage.document_dir")
	allowMissing := flag.Bool("allow-missing", false, "页面缺正文时仍继续（默认中止）")
	flag.Parse()

	config.InitConfig()
	if *dsn != "" {
		config.AppConfig.Database.DSN = *dsn
	}
	if *docDir != "" {
		config.AppConfig.Storage.DocumentDir = *docDir
	}
	if config.AppConfig.Storage.DocumentDir == "" {
		config.AppConfig.Storage.DocumentDir = "./data/markdowns"
	}
	repos.InitDB()

	docRepo := workspacerepo.NewDocumentRepository()
	spaceRepo := workspacerepo.NewSpaceRepository()
	// 必须含目录：raw_name 回退要按父目录原始名称拼路径，目录本身也要落 dir_no_readme 行。
	docs, err := docRepo.FindAllDocumentsIncludeDirs()
	if err != nil {
		fail("load documents: %v", err)
	}
	store := utils.NewDocumentStorageWithDir(config.AppConfig.Storage.DocumentDir)

	byID := make(map[int]*workspacemodel.Document, len(docs))
	for _, d := range docs {
		byID[d.ID] = d
	}
	spaceNames := make(map[int]string)
	for _, d := range docs {
		if _, ok := spaceNames[d.SpaceID]; ok {
			continue
		}
		name := fmt.Sprintf("space_%d", d.SpaceID)
		if space, err := spaceRepo.FindByIDCached(d.SpaceID); err == nil && space != nil {
			name = space.Name
		}
		spaceNames[d.SpaceID] = name
	}

	var rows []result
	var missing []*workspacemodel.Document
	counts := map[string]int{}
	for _, doc := range docs {
		if doc.IsDelete == 1 {
			continue
		}
		r := resolve(doc, byID, spaceNames[doc.SpaceID], store)
		if r == nil {
			missing = append(missing, doc)
			continue
		}
		counts[r.MatchType]++
		rows = append(rows, *r)
	}

	totalBytes := 0
	for _, r := range rows {
		totalBytes += r.Size
	}
	fmt.Printf("在线文档 %d 篇：exact=%d raw_name=%d dir_no_readme=%d，缺正文 %d，共 %d 字节\n",
		len(rows)+len(missing), counts["exact"], counts["raw_name"], counts["dir_no_readme"], len(missing), totalBytes)
	for _, r := range rows {
		if r.MatchType != "exact" {
			fmt.Printf("  [%s] doc %d %s -> %s (%d 字节)\n", r.MatchType, r.DocID, r.Name, r.PageFile, r.Size)
		}
	}
	for _, d := range missing {
		fmt.Printf("  [missing_page] doc %d %s type=%d path=%s\n", d.ID, d.Name, d.Type, d.Path)
	}

	if len(missing) > 0 && !*allowMissing {
		fail("存在 %d 篇页面读不到正文，已中止（确认后用 -allow-missing 继续）", len(missing))
	}

	if !*apply {
		fmt.Println("dry-run：未写库。确认无误后加 -apply。")
		return
	}

	if err := writeRows(rows); err != nil {
		fail("write: %v", err)
	}
	if err := verify(rows); err != nil {
		fail("verify: %v", err)
	}
	fmt.Printf("已写入并校验 %d 篇正文\n", len(rows))
}

// resolve 依次尝试 exact / raw_name / dir_no_readme；页面未命中返回 nil。
func resolve(doc *workspacemodel.Document, byID map[int]*workspacemodel.Document, spaceName string, store *utils.DocumentStorage) *result {
	candidates := []struct {
		matchType string
		pageFile  string
	}{
		{"exact", workspacesvc.ComputePageFilePath(doc)},
		{"raw_name", rawPageFile(doc, byID, spaceName)},
	}
	for _, c := range candidates {
		if c.pageFile == "" {
			continue
		}
		abs := store.GetAbsPageFile(c.pageFile)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		return newResult(doc, c.matchType, c.pageFile, data)
	}
	if doc.Type == workspacemodel.DocumentTypeDir {
		return newResult(doc, "dir_no_readme", workspacesvc.ComputePageFilePath(doc), nil)
	}
	return nil
}

// rawPageFile 按「原始名称」拼路径：安全算法会把名称里的 / 净化成 _，而历史文件是真的嵌套目录。
func rawPageFile(doc *workspacemodel.Document, byID map[int]*workspacemodel.Document, spaceName string) string {
	segments := []string{spaceName}
	if doc.Path != "0" && doc.Path != "" {
		parts := strings.Split(doc.Path, ",")
		skipFirst := true
		for _, part := range parts {
			if part == "" || part == "0" {
				continue
			}
			if skipFirst {
				skipFirst = false
				continue
			}
			id, _ := strconv.Atoi(part)
			if p, ok := byID[id]; ok {
				segments = append(segments, p.Name)
			}
		}
	}
	if doc.Type == workspacemodel.DocumentTypePage {
		segments = append(segments, doc.Name+"."+"md")
	} else {
		segments = append(segments, doc.Name, "README.md")
	}
	return filepath.Join(segments...)
}

func newResult(doc *workspacemodel.Document, matchType, pageFile string, data []byte) *result {
	sum := sha256.Sum256(data)
	return &result{
		DocID:     doc.ID,
		Name:      doc.Name,
		MatchType: matchType,
		PageFile:  pageFile,
		Size:      len(data),
		SHA256:    hex.EncodeToString(sum[:]),
		Content:   string(data),
	}
}

func writeRows(rows []result) error {
	repo := workspacerepo.NewDocumentContentRepository()
	for start := 0; start < len(rows); start += 100 {
		end := start + 100
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]
		if err := repos.DB.Transaction(func(tx *gorm.DB) error {
			for _, r := range batch {
				if err := repo.UpsertTx(tx, r.DocID, r.Content); err != nil {
					return fmt.Errorf("doc %d: %w", r.DocID, err)
				}
			}
			return nil
		}); err != nil {
			return err
		}
		fmt.Printf("已写 %d/%d\n", end, len(rows))
	}
	return nil
}

// verify 回读逐篇比对 sha256 与字节数。
func verify(rows []result) error {
	repo := workspacerepo.NewDocumentContentRepository()
	sort.Slice(rows, func(i, j int) bool { return rows[i].DocID < rows[j].DocID })
	for _, r := range rows {
		content, err := repo.Get(r.DocID)
		if err != nil {
			return fmt.Errorf("doc %d read back: %w", r.DocID, err)
		}
		sum := sha256.Sum256([]byte(content))
		if len(content) != r.Size || hex.EncodeToString(sum[:]) != r.SHA256 {
			return fmt.Errorf("doc %d hash mismatch: want %d/%s got %d/%s",
				r.DocID, r.Size, r.SHA256, len(content), hex.EncodeToString(sum[:]))
		}
	}
	return nil
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}
