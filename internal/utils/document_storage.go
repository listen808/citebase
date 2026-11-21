package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	DocumentTypePage    = 1 // 文档页
	DocumentTypeDir     = 2 // 目录
	DocumentDefaultName = "README"
	DocumentPageSuffix  = ".md"
)

// DocumentStorage 文档文件系统存储
type DocumentStorage struct {
	BaseDir string // 基础目录，如 ./data
	DocDir  string // 文档目录，如 ./data/markdowns
	lock    sync.RWMutex
}

// NewDocumentStorage 创建文档存储实例（使用默认路径）
func NewDocumentStorage(baseDir string) *DocumentStorage {
	if baseDir == "" {
		baseDir = "./data"
	}
	return &DocumentStorage{
		BaseDir: baseDir,
		DocDir:  filepath.Join(baseDir, "markdowns"),
	}
}

// NewDocumentStorageWithDir 创建文档存储实例（使用完整路径）
func NewDocumentStorageWithDir(docDir string) *DocumentStorage {
	return &DocumentStorage{
		DocDir: docDir,
	}
}

// GetPageFile 获取文档页文件路径 (相对于 DocDir)
// 例如: space_name/doc_name.md 或 space_name/folder_name/README.md
//
// 空间名与文档名可能来自用户输入或 AI 生成内容，必须先净化成安全的单段路径分量，
// 否则「../x」之类的名称会直接拼出可逃逸存储根目录的路径。
func (s *DocumentStorage) GetPageFile(spaceName string, docName string, docType int) string {
	spaceName = SafePathSegment(spaceName)
	if docType == DocumentTypePage {
		return filepath.Join(spaceName, SafePathSegment(docName)+DocumentPageSuffix)
	}
	return filepath.Join(spaceName, SafePathSegment(docName), DocumentDefaultName+DocumentPageSuffix)
}

// SafePathSegment 把任意名称净化为可安全用于文件路径的单个分量。
// 丢弃路径分隔符、`..`、`.`、NUL 与其他控制字符，并限制长度。
func SafePathSegment(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\x00':
			b.WriteRune('_')
		case '.', '-', '_':
			b.WriteRune(r)
		default:
			if r < 0x20 || r == 0x7f {
				b.WriteRune('_')
				continue
			}
			b.WriteRune(r)
		}
	}
	cleaned := strings.Trim(b.String(), ". ")
	if cleaned == "" {
		cleaned = "untitled"
	}
	if r := []rune(cleaned); len(r) > 120 {
		cleaned = string(r[:120])
	}
	return cleaned
}

// GetPageFileByPath 根据文档路径获取页文件（用于生成新文档）
// docPath 格式: "0,1,2" 表示 根目录 -> 文档1 -> 文档2
// 注意: 这个方法只用于创建文档时获取路径，实际读取需要使用 GetPageFileByDocInfo
func (s *DocumentStorage) GetPageFileByPath(spaceName string, docPath string, docName string, docType int) string {
	if docPath == "0" || docPath == "" {
		return s.GetPageFile(spaceName, docName, docType)
	}

	// 解析路径，构建相对目录
	// docPath 格式: "1,2,3" (父文档ID列表)
	pathParts := strings.Split(docPath, ",")
	var relativeDir string
	for _, part := range pathParts {
		if part != "" && part != "0" {
			relativeDir = filepath.Join(relativeDir, part)
		}
	}

	if relativeDir == "" {
		return s.GetPageFile(spaceName, docName, docType)
	}

	// 对于子文档，需要返回相对路径
	// 注意: 实际路径需要在 service 层根据父文档名称构建
	return filepath.Join(relativeDir, docName)
}

// GetAbsPageFile 把存储内相对路径解析为绝对路径，并保证结果一定落在 DocDir 之内。
//
// pageFile 可能来自数据库中历史写入的名称、AI 生成的标题或直接传入的路径，
// 因此这里做第二层防护：丢弃 `..`、`.` 与绝对路径前缀，杜绝逃逸 DocDir 的读写删。
func (s *DocumentStorage) GetAbsPageFile(pageFile string) string {
	return filepath.Join(s.DocDir, SafeRelativePath(pageFile))
}

// SafeRelativePath 净化相对路径：逐段丢弃 `..`、`.`、空段与控制字符。
func SafeRelativePath(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" {
		return ""
	}
	segments := strings.Split(p, "/")
	cleaned := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		cleaned = append(cleaned, seg)
	}
	return filepath.FromSlash(strings.Join(cleaned, "/"))
}

// GetContent 读取文档内容
func (s *DocumentStorage) GetContent(pageFile string) (string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	absFile := s.GetAbsPageFile(pageFile)
	content, err := os.ReadFile(absFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // 文件不存在返回空内容
		}
		return "", err
	}
	return string(content), nil
}

// SetContent 写入文档内容
func (s *DocumentStorage) SetContent(pageFile string, content string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	absFile := s.GetAbsPageFile(pageFile)
	absDir := filepath.Dir(absFile)

	// 确保目录存在
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(absFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Delete 删除文档文件
func (s *DocumentStorage) Delete(pageFile string, docType int) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	absFile := s.GetAbsPageFile(pageFile)

	if docType == DocumentTypePage {
		return os.Remove(absFile)
	}

	// 目录类型删除整个目录
	dir := filepath.Dir(absFile)
	return os.RemoveAll(dir)
}

// Rename 重命名文档
func (s *DocumentStorage) Rename(oldPageFile string, newSpaceName string, newDocName string, newDocType int) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	oldAbsFile := s.GetAbsPageFile(oldPageFile)
	newPageFile := s.GetPageFile(newSpaceName, newDocName, newDocType)
	newAbsFile := s.GetAbsPageFile(newPageFile)

	if newDocType == DocumentTypeDir {
		// 目录类型：重命名整个目录
		oldDir := filepath.Dir(oldAbsFile)
		newDir := filepath.Dir(newAbsFile)

		// 如果旧目录存在，则重命名
		if _, err := os.Stat(oldDir); err == nil {
			// 确保新父目录存在
			newParentDir := filepath.Dir(newDir)
			if err := os.MkdirAll(newParentDir, 0755); err != nil {
				return err
			}
			return os.Rename(oldDir, newDir)
		}
	} else {
		// 页面类型：重命名单个文件
		// 如果文件存在，则移动/重命名
		if _, err := os.Stat(oldAbsFile); err == nil {
			// 确保新目录存在
			newDir := filepath.Dir(newAbsFile)
			if err := os.MkdirAll(newDir, 0755); err != nil {
				return err
			}
			return os.Rename(oldAbsFile, newAbsFile)
		}
	}

	return nil
}

// RenameByPath 使用完整路径重命名文档
func (s *DocumentStorage) RenameByPath(oldPageFile string, newPageFile string, docType int) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	oldAbsFile := s.GetAbsPageFile(oldPageFile)
	newAbsFile := s.GetAbsPageFile(newPageFile)

	if docType == DocumentTypeDir {
		// 目录类型：重命名整个目录
		oldDir := filepath.Dir(oldAbsFile)
		newDir := filepath.Dir(newAbsFile)

		// 如果旧目录存在，则重命名
		if _, err := os.Stat(oldDir); err == nil {
			// 确保新父目录存在
			newParentDir := filepath.Dir(newDir)
			if err := os.MkdirAll(newParentDir, 0755); err != nil {
				return err
			}
			return os.Rename(oldDir, newDir)
		}
	} else {
		// 页面类型：重命名单个文件
		// 如果文件存在，则移动/重命名
		if _, err := os.Stat(oldAbsFile); err == nil {
			// 确保新目录存在
			newDir := filepath.Dir(newAbsFile)
			if err := os.MkdirAll(newDir, 0755); err != nil {
				return err
			}
			return os.Rename(oldAbsFile, newAbsFile)
		}
	}

	return nil
}

// Exists 检查文件是否存在
func (s *DocumentStorage) Exists(pageFile string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	absFile := s.GetAbsPageFile(pageFile)
	_, err := os.Stat(absFile)
	return err == nil
}

// EnsureBaseDirs 确保基础目录存在
func (s *DocumentStorage) EnsureBaseDirs() error {
	return os.MkdirAll(s.DocDir, 0755)
}
