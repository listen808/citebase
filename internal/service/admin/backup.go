package admin

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils/config"
	"github.com/listen808/citebase/internal/utils/dbdump"
)

// BackupService 备份与恢复服务
// 方案：dbdump（纯 Go）导出/回灌 SQL + zip 打包数据目录（markdowns/images/attachments），
// 全程不依赖 mysqldump / mysql 客户端二进制。
type BackupService struct {
	backupDir string
}

// BackupInfo 备份元信息
type BackupInfo struct {
	ID         string `json:"id"`          // 文件名（不含扩展名）
	FileName   string `json:"file_name"`   // 完整文件名
	Size       int64  `json:"size"`        // 字节数
	SizeMB     string `json:"size_mb"`     // 可读大小
	CreateTime int64  `json:"create_time"` // Unix 时间戳
	CreateAt   string `json:"create_at"`   // 可读时间
}

// NewBackupService 创建备份服务实例
func NewBackupService() *BackupService {
	backupDir := filepath.Join("data", "backups")
	// 确保目录存在
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logx.Errorf("[BackupService] failed to create backup dir: %v", err)
	}
	return &BackupService{backupDir: backupDir}
}

// CreateBackup 创建备份
// 步骤：1) dbdump 导出 SQL  2) zip 打包 SQL + 数据目录
func (s *BackupService) CreateBackup() (*BackupInfo, error) {
	dsn := config.AppConfig.Database.DSN
	if dsn == "" {
		return nil, fmt.Errorf("database DSN not configured")
	}

	now := time.Now()
	backupID := fmt.Sprintf("backup_%s", now.Format("20060102_150405"))
	sqlFile := filepath.Join(s.backupDir, backupID+".sql")
	zipFile := filepath.Join(s.backupDir, backupID+".zip")

	// 1. 导出数据库：用驱动直连，既不需要机器上有 mysqldump，
	//    也不会把口令拼进子进程命令行（进程列表里看得见）。
	dumpFh, err := os.Create(sqlFile)
	if err != nil {
		return nil, fmt.Errorf("create sql file failed: %v", err)
	}
	logx.Infof("[BackupService] dumping database for backup %s", backupID)
	stats, dumpErr := dbdump.Dump(dsn, dumpFh, dbdump.Options{
		Warn: func(msg string) { logx.Warnf("[BackupService] %s", msg) },
	})
	dumpFh.Close()
	if dumpErr != nil {
		os.Remove(sqlFile)
		return nil, fmt.Errorf("database dump failed: %v", dumpErr)
	}
	logx.Infof("[BackupService] dumped %d tables / %d rows", stats.Tables, stats.Rows)

	// 2. zip 打包 SQL + 数据目录
	zipFh, err := os.Create(zipFile)
	if err != nil {
		os.Remove(sqlFile)
		return nil, fmt.Errorf("create zip file failed: %v", err)
	}
	defer zipFh.Close()

	zipWriter := zip.NewWriter(zipFh)
	defer zipWriter.Close()

	// 加入 SQL 文件
	if err := addFileToZip(zipWriter, sqlFile, "database.sql"); err != nil {
		os.Remove(sqlFile)
		os.Remove(zipFile)
		return nil, fmt.Errorf("zip sql failed: %v", err)
	}

	// 加入数据目录
	dataDirs := s.getDataDirs()
	for _, dir := range dataDirs {
		if _, err := os.Stat(dir); err != nil {
			continue // 目录不存在跳过
		}
		relName := filepath.Base(dir)
		if err := addDirToZip(zipWriter, dir, relName); err != nil {
			logx.Infof("[BackupService] zip dir %s failed: %v", dir, err)
			// 不中断，继续其他目录
		}
	}

	zipWriter.Close()
	// 删除临时 SQL 文件
	os.Remove(sqlFile)

	// 返回备份信息
	info, err := os.Stat(zipFile)
	if err != nil {
		return nil, err
	}
	return &BackupInfo{
		ID:         backupID,
		FileName:   backupID + ".zip",
		Size:       info.Size(),
		SizeMB:     fmt.Sprintf("%.2f MB", float64(info.Size())/1024/1024),
		CreateTime: now.Unix(),
		CreateAt:   now.Format("2006-01-02 15:04:05"),
	}, nil
}

// ListBackups 列出所有备份（按时间降序）
func (s *BackupService) ListBackups() ([]*BackupInfo, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return []*BackupInfo{}, nil
	}
	var backups []*BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(name, ".zip")
		backups = append(backups, &BackupInfo{
			ID:         id,
			FileName:   name,
			Size:       info.Size(),
			SizeMB:     fmt.Sprintf("%.2f MB", float64(info.Size())/1024/1024),
			CreateTime: info.ModTime().Unix(),
			CreateAt:   info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	// 按时间降序
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreateTime > backups[j].CreateTime
	})
	return backups, nil
}

// DeleteBackup 删除备份
func (s *BackupService) DeleteBackup(id string) error {
	if !isValidBackupID(id) {
		return fmt.Errorf("invalid backup id")
	}
	zipFile := filepath.Join(s.backupDir, id+".zip")
	if _, err := os.Stat(zipFile); err != nil {
		return fmt.Errorf("backup not found")
	}
	return os.Remove(zipFile)
}

// GetBackupFilePath 获取备份文件路径（用于下载）
func (s *BackupService) GetBackupFilePath(id string) (string, error) {
	if !isValidBackupID(id) {
		return "", fmt.Errorf("invalid backup id")
	}
	zipFile := filepath.Join(s.backupDir, id+".zip")
	if _, err := os.Stat(zipFile); err != nil {
		return "", fmt.Errorf("backup not found")
	}
	return zipFile, nil
}

// ImportBackup 导入外部备份包：上传的 zip 流式落到 data/backups，之后与本机创建的
// 备份同列表、同恢复路径（RestoreBackup 按 ID 找文件，不区分来源）。
//
// 只认 database.sql：恢复以数据库为准，缺这一行的包救不回任何东西，
// 让它进列表只会误导管理员（纯数据目录不算备份）。
// 上传边收边写临时文件，校验通过才改名落位 —— 半途而废的请求不会留下能进列表的残包。
func (s *BackupService) ImportBackup(srcName string, r io.Reader) (*BackupInfo, error) {
	base := sanitizeBackupID(srcName)
	if base == "" {
		base = time.Now().Format("import_20060102_150405")
	}
	id := base
	target := filepath.Join(s.backupDir, id+".zip")
	for n := 1; ; n++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		id = fmt.Sprintf("%s_%d", base, n)
		target = filepath.Join(s.backupDir, id+".zip")
	}

	tmp := target + ".upload"
	fh, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("create upload file failed: %v", err)
	}
	written, copyErr := io.Copy(fh, r)
	closeErr := fh.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("写入上传文件失败: %v", copyErr)
	}
	if written < 22 { // zip 的最小尺寸是空归档的 EOCD 记录
		os.Remove(tmp)
		return nil, fmt.Errorf("上传内容为空")
	}
	if err := validateBackupZip(tmp); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("备份落位失败: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &BackupInfo{
		ID:         id,
		FileName:   id + ".zip",
		Size:       info.Size(),
		SizeMB:     fmt.Sprintf("%.2f MB", float64(info.Size())/1024/1024),
		CreateTime: now.Unix(),
		CreateAt:   now.Format("2006-01-02 15:04:05"),
	}, nil
}

// validateBackupZip 确认是合法 zip 且含根下的 database.sql
func validateBackupZip(path string) error {
	ra, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("不是有效的 zip 备份包: %v", err)
	}
	defer ra.Close()
	for _, f := range ra.File {
		if filepath.ToSlash(f.Name) == "database.sql" {
			return nil
		}
	}
	return fmt.Errorf("备份包缺少 database.sql")
}

// sanitizeBackupID 从上传文件名推导备份 ID：只留 isValidBackupID 认的字符
// （字母/数字/下划线），既防路径遍历，也保证后续 download/restore/delete 的 ID 校验能过。
func sanitizeBackupID(name string) string {
	name = strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".zip")
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	// 留 10 个字符给撞名时的 _N 后缀，否则拼完会超 isValidBackupID 的 100 上限
	if len(out) > 90 {
		out = out[:90]
	}
	return out
}

// RestoreBackup 从备份恢复
// 步骤：1) 解压 zip 到临时目录  2) 恢复数据目录  3) dbdump 逐条执行 SQL
func (s *BackupService) RestoreBackup(id string) error {
	if !isValidBackupID(id) {
		return fmt.Errorf("invalid backup id")
	}
	zipFile := filepath.Join(s.backupDir, id+".zip")
	if _, err := os.Stat(zipFile); err != nil {
		return fmt.Errorf("backup not found")
	}

	// 创建临时目录解压
	tmpDir, err := os.MkdirTemp("", "citebase_restore_*")
	if err != nil {
		return fmt.Errorf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 解压
	if err := unzipToDir(zipFile, tmpDir); err != nil {
		return fmt.Errorf("unzip failed: %v", err)
	}

	// 恢复数据目录（覆盖现有文件）
	dataDirs := s.getDataDirs()
	for _, dir := range dataDirs {
		relName := filepath.Base(dir)
		srcDir := filepath.Join(tmpDir, relName)
		if _, err := os.Stat(srcDir); err != nil {
			continue
		}
		// 备份当前目录（重命名为 .bak）
		bakDir := dir + ".bak"
		os.RemoveAll(bakDir)
		if _, err := os.Stat(dir); err == nil {
			_ = os.Rename(dir, bakDir)
		}
		// 移动解压的目录到目标位置（跨卷时 os.Rename 必然失败——临时目录由
		// os.MkdirTemp 落在系统盘，数据目录常配在另一块盘——退化为递归复制）
		if err := os.Rename(srcDir, dir); err != nil {
			if copyErr := copyDir(srcDir, dir); copyErr != nil {
				// 恢复失败时回滚
				if _, e := os.Stat(bakDir); e == nil {
					_ = os.Rename(bakDir, dir)
				}
				return fmt.Errorf("restore dir %s failed: %v (copy fallback: %v)", relName, err, copyErr)
			}
		}
		// 删除备份目录
		os.RemoveAll(bakDir)
	}

	// 恢复数据库
	sqlFile := filepath.Join(tmpDir, "database.sql")
	if _, err := os.Stat(sqlFile); err == nil {
		fh, err := os.Open(sqlFile)
		if err != nil {
			return fmt.Errorf("open sql file failed: %v", err)
		}
		defer fh.Close()

		logx.Infof("[BackupService] restoring database from %s", sqlFile)
		st, err := dbdump.Restore(config.AppConfig.Database.DSN, fh, dbdump.RestoreOptions{
			// 备份里的 CREATE TABLE 不带 DROP TABLE，遇到同名表先删掉，否则回灌到
			// 有数据的库上会撞 Error 1050；恢复的语义就是"回到备份那一刻"。
			ReplaceTables: true,
			Warn:          func(msg string) { logx.Warnf("[BackupService] %s", msg) },
		})
		if err != nil {
			return fmt.Errorf("database restore failed: %v", err)
		}
		logx.Infof("[BackupService] restored %d statements / %d tables", st.Statements, st.Tables)
	}

	return nil
}

// getDataDirs 获取需要备份的数据目录。
// P3：对应源已入库（content_source=db / attachment_source=db）时跳过该 media 目录 ——
// SQL dump 已含 aw_document_content / aw_attachment_chunk 完整副本，再打包磁盘是双份体积。
// 前提：迁移工具 -verify-only 全量通过（当前 421/421）；哪个源仍在 file 模式就继续备份哪个目录。
func (s *BackupService) getDataDirs() []string {
	var dirs []string
	if !doccontent.UseDB() {
		docDir := config.AppConfig.Storage.DocumentDir
		if docDir == "" {
			docDir = "./data/markdowns"
		}
		dirs = append(dirs, docDir)
	}
	if !workspacerepo.UseDB() {
		imageDir := config.AppConfig.Storage.ImageDir
		if imageDir == "" {
			imageDir = "./data/images"
		}
		attachmentDir := config.AppConfig.Storage.AttachmentDir
		if attachmentDir == "" {
			attachmentDir = "./data/attachment"
		}
		dirs = append(dirs, imageDir, attachmentDir)
	}
	return dirs
}

// isValidBackupID 校验备份 ID 格式（防止路径遍历）
func isValidBackupID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	// 只允许字母、数字、下划线
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// addFileToZip 将单个文件加入 zip
func addFileToZip(zipWriter *zip.Writer, filePath, zipName string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipName
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}

// addDirToZip 递归将目录加入 zip
func addDirToZip(zipWriter *zip.Writer, dirPath, zipPrefix string) error {
	return filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return err
		}
		zipPath := filepath.Join(zipPrefix, relPath)
		// 统一为正斜杠（zip 标准）
		zipPath = strings.ReplaceAll(zipPath, "\\", "/")

		if info.IsDir() {
			_, err := zipWriter.Create(zipPath + "/")
			return err
		}
		return addFileToZip(zipWriter, filePath, zipPath)
	})
}

// copyDir 递归复制目录，os.Rename 跨卷失败时的回退路径
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		return err
	})
}

// unzipToDir 解压 zip 到目录
func unzipToDir(zipFile, destDir string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		// 防止 Zip Slip 漏洞
		path := filepath.Join(destDir, file.Name) //nolint:gosec // G305: zip slip prevented by prefix check below
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry outside target dir: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			_ = os.MkdirAll(path, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		//nolint:gosec // G305,G110: path validated above; backups are created by the same app
		_, err = io.Copy(outFile, io.LimitReader(rc, 2<<30))
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
