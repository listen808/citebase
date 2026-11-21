package main

// migrate_attachment —— P2 附件回填：把 data/attachment、data/images 的二进制按 4MB 切块写入 aw_attachment_chunk。
//
// 用法（仓库根目录）：
//
//	go run ./tools/migrate_attachment              # dry-run：只统计，不写库
//	go run ./tools/migrate_attachment -apply       # 真正写库（单附件一个事务）
//	go run ./tools/migrate_attachment -dsn "..."   # 覆盖 configs/config.yaml 的 DSN
//	go run ./tools/migrate_attachment -no-orphans  # 不为磁盘孤儿文件补建记录
//
// 规则（与 docs/citebase-割接方案与演练报告.md 一致）：
//   - 逐条 os.Stat 重算 size（修复 284 条 0 值），计算 sha256/chunk_count，成功置 storage_kind=1；
//   - 文件缺失的记录保持 storage_kind=2 并列出，不阻塞其余记录；
//   - 磁盘孤儿文件（无库记录）按原 path 补建 aw_attachment 记录后一并入库（path 不改，否则正文引用断链）；
//     补建前提：path 中的 document_id 能解析且文档存在，否则只报告不导入；
//   - 回填后逐条从库回读比对 sha256 + 分块完整性校验。

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils/config"
	"gorm.io/gorm"
)

// job 一条待导入的附件：att 是库行（孤儿文件内存中构造，ID=0），full 是磁盘绝对路径
type job struct {
	att  *workspacemodel.Attachment
	full string
}

func main() {
	apply := flag.Bool("apply", false, "真正写库；缺省 dry-run")
	dsn := flag.String("dsn", "", "覆盖 configs/config.yaml 的 DSN")
	noOrphans := flag.Bool("no-orphans", false, "不为磁盘孤儿文件补建记录")
	verifyOnly := flag.Bool("verify-only", false, "只对全部 storage_kind=1 记录回读校验，不导入")
	flag.Parse()

	config.InitConfig()
	if *dsn != "" {
		config.AppConfig.Database.DSN = *dsn
	}
	repos.InitDB() // AutoMigrate 建 aw_attachment_chunk 并为 aw_attachment 补三列 + path 唯一索引

	// 1. 现有记录
	var atts []workspacemodel.Attachment
	if err := repos.DB.Find(&atts).Error; err != nil {
		fail("load attachments: %v", err)
	}
	existing := make(map[string]bool, len(atts))
	for i := range atts {
		existing[atts[i].Path] = true
	}

	if *verifyOnly {
		verifyAll(atts)
		return
	}

	// 2. 孤儿扫描：磁盘有文件、库里无记录
	var orphans []workspacemodel.Attachment
	if !*noOrphans {
		orphans = scanOrphans(existing)
	}
	fmt.Printf("库记录 %d 条，磁盘孤儿 %d 条\n", len(atts), len(orphans))

	// 3. 组装待导入清单（未迁移的 kind=2 + 补建的孤儿）
	missing := 0
	var jobs []job
	orphans = buildOrphanRecords(orphans, &missing)
	for i := range atts {
		a := &atts[i]
		if a.StorageKind == workspacemodel.AttachmentStorageDB {
			continue
		}
		full := workspacesvc.ResolveAttachmentPath(a.Path)
		info, err := os.Stat(full)
		if err != nil {
			missing++
			fmt.Printf("[missing] id=%d path=%s（保持 storage_kind=2，不阻塞）\n", a.ID, a.Path)
			continue
		}
		if info.Size() != a.Size {
			fmt.Printf("[size-fix] id=%d %s: %d -> %d\n", a.ID, a.Path, a.Size, info.Size())
		}
		jobs = append(jobs, job{att: a, full: full})
	}
	for i := range orphans {
		a := &orphans[i]
		full := workspacesvc.ResolveAttachmentPath(a.Path)
		if info, err := os.Stat(full); err != nil {
			missing++
			fmt.Printf("[orphan-missing] path=%s\n", a.Path)
			continue
		} else {
			a.Size = info.Size()
		}
		jobs = append(jobs, job{att: a, full: full})
	}

	var totalBytes int64
	for _, j := range jobs {
		totalBytes += j.att.Size
	}
	fmt.Printf("待导入 %d 条 / %d 字节（%.2fMB），缺失 %d 条\n",
		len(jobs), totalBytes, float64(totalBytes)/(1024*1024), missing)

	if !*apply {
		fmt.Println("dry-run：未写库。确认无误后加 -apply。")
		return
	}

	// 4. 逐条导入：单附件一个事务（元数据行 + 全部块行）
	var imported []workspacemodel.Attachment
	for i := range jobs {
		j := &jobs[i]
		if err := importOne(j.att, j.full); err != nil {
			fail("import id=%d %s: %v", j.att.ID, j.att.Path, err)
		}
		imported = append(imported, *j.att)
		if (i+1)%20 == 0 || i+1 == len(jobs) {
			fmt.Printf("已导入 %d/%d\n", i+1, len(jobs))
		}
	}

	// 5. 校验：回读 sha256 + 分块完整性
	bad := 0
	for i := range imported {
		a := &imported[i]
		if err := verifyOne(a); err != nil {
			bad++
			fmt.Printf("[verify-fail] id=%d %s: %v\n", a.ID, a.Path, err)
		}
	}
	fmt.Printf("校验 %d 条：失败 %d\n", len(imported), bad)
	if bad > 0 {
		os.Exit(1)
	}
	fmt.Printf("P2 附件回填完成：%d 条已入 aw_attachment_chunk，缺失/未入 %d 条保持磁盘读\n", len(imported), missing)
}

// scanOrphans 遍历两个磁盘目录，找库里没有 path 记录的文件
func scanOrphans(existing map[string]bool) []workspacemodel.Attachment {
	var out []workspacemodel.Attachment
	dirs := []struct{ prefix, dir string }{
		{"images/", config.AppConfig.Storage.ImageDir},
		{"attachment/", config.AppConfig.Storage.AttachmentDir},
	}
	for _, d := range dirs {
		if d.dir == "" {
			continue
		}
		root := filepath.FromSlash(d.dir)
		_ = filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return nil
			}
			stored := d.prefix + filepath.ToSlash(rel)
			if existing[stored] {
				return nil
			}
			out = append(out, workspacemodel.Attachment{Path: stored, Name: entry.Name()})
			return nil
		})
	}
	return out
}

// buildOrphanRecords 给孤儿文件补建记录：path 原样（否则正文引用断链），
// document_id 从 path 解析且必须存在，解析不了/文档不存在的只报告不导入。
func buildOrphanRecords(orphans []workspacemodel.Attachment, missing *int) []workspacemodel.Attachment {
	var ok []workspacemodel.Attachment
	now := int(time.Now().Unix())
	for _, o := range orphans {
		parts := strings.Split(o.Path, "/")
		var docID int
		if len(parts) >= 4 {
			fmt.Sscanf(parts[2], "%d", &docID)
		}
		if docID <= 0 {
			fmt.Printf("[orphan-skip] path=%s（解析不到 document_id）\n", o.Path)
			*missing++
			continue
		}
		if err := repos.DB.First(&workspacemodel.Document{}, docID).Error; err != nil {
			fmt.Printf("[orphan-skip] path=%s（文档 %d 不存在）\n", o.Path, docID)
			*missing++
			continue
		}
		source, mediaType := workspacemodel.ClassifyAttachmentByExt(o.Name)
		o.DocumentID = docID
		o.Source = source
		o.MediaType = mediaType
		o.CreateTime = now
		o.UpdateTime = now
		ok = append(ok, o)
		fmt.Printf("[orphan-add] path=%s doc=%d\n", o.Path, docID)
	}
	return ok
}

// verifyAll 对库内全部 kind=1 记录回读 sha256 + 分块完整性（-verify-only 验收用）
func verifyAll(atts []workspacemodel.Attachment) {
	bad := 0
	total := 0
	var totalBytes int64
	for i := range atts {
		a := &atts[i]
		if a.StorageKind != workspacemodel.AttachmentStorageDB {
			continue
		}
		total++
		totalBytes += a.Size
		if err := verifyOne(a); err != nil {
			bad++
			fmt.Printf("[verify-fail] id=%d %s: %v\n", a.ID, a.Path, err)
		}
	}
	fmt.Printf("校验 %d 条（%.2fMB）：失败 %d\n", total, float64(totalBytes)/(1024*1024), bad)
	if bad > 0 {
		os.Exit(1)
	}
}

// importOne 单附件一个事务：孤儿先插元数据行，块行写入与元数据回写都由 WriteAttachmentChunks 完成
func importOne(a *workspacemodel.Attachment, full string) error {
	return repos.DB.Transaction(func(tx *gorm.DB) error {
		f, err := os.Open(full)
		if err != nil {
			return err
		}
		defer f.Close()
		if a.ID == 0 {
			if err := tx.Create(a).Error; err != nil {
				return err
			}
		}
		return workspacerepo.WriteAttachmentChunks(tx, a, f)
	})
}

// verifyOne 回读整文件比对 sha256 + 分块完整性
func verifyOne(a *workspacemodel.Attachment) error {
	repo := workspacerepo.NewAttachmentRepository()
	h := sha256.New()
	if err := repo.StreamAttachmentChunks(a.ID, h); err != nil {
		return fmt.Errorf("stream: %w", err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != a.SHA256 {
		return fmt.Errorf("sha256 mismatch: want %s got %s", a.SHA256, got)
	}
	return repo.ChunkIntegrity(a.ID, a.ChunkCount, a.Size)
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}
