package workspace

import (
	"bytes"
	"errors"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
	"github.com/listen808/citebase/internal/utils/safego"
	"gorm.io/gorm"
)

var (
	ErrInvalidDocumentID    = errors.New("invalid document id")
	ErrInvalidFile          = errors.New("no file uploaded")
	ErrInvalidFilename      = errors.New("invalid filename")
	ErrUnsupportedFileType  = errors.New("unsupported file type")
	ErrFileTooLarge         = errors.New("file too large")
	ErrFileValidationFailed = errors.New("file validation failed")
	ErrAttachmentNotFound   = errors.New("attachment not found")
	ErrSpaceNotFound        = errors.New("space not found")
	ErrNoAccessPermission   = errors.New("no permission to access this attachment")
	ErrNoEditPermission     = errors.New("no permission to edit in this space")
	ErrFileNotFound         = errors.New("file not found on disk")
)

var allowedAttachmentExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".webp": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".txt": true,
	".zip": true, ".rar": true, ".7z": true,
	// 音视频：ClassifyAttachmentByExt 已有 2/3 分类，白名单此前漏配导致永远传不上来
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".m4a": true,
	".mp4": true, ".webm": true, ".mkv": true, ".avi": true, ".mov": true, ".flv": true,
}

const (
	maxAttachmentSize = 100 * 1024 * 1024
)

type AttachmentService struct {
	attachmentRepo *workspacerepo.AttachmentRepository
	docRepo        *workspacerepo.DocumentRepository
	spaceRepo      *workspacerepo.SpaceRepository
	docService     *DocumentService
	spaceService   *SpaceService
}

func NewAttachmentService() *AttachmentService {
	return &AttachmentService{
		attachmentRepo: workspacerepo.NewAttachmentRepository(),
		docRepo:        workspacerepo.NewDocumentRepository(),
		spaceRepo:      workspacerepo.NewSpaceRepository(),
		docService:     NewDocumentService(),
		spaceService:   NewSpaceService(),
	}
}

type UploadAttachmentResult struct {
	ID         int    `json:"id"`
	DocumentID int    `json:"document_id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Path       string `json:"path"`
	URL        string `json:"url"`
	CreateTime int    `json:"create_time"`
}

func (s *AttachmentService) UploadAttachment(documentID int, file multipart.File, header *multipart.FileHeader, userID int) (*UploadAttachmentResult, error) {
	logx.Infof("[UploadService] 开始处理: documentID=%d, userID=%d, filename=%s, size=%d", documentID, userID, header.Filename, header.Size)

	doc, err := s.docRepo.FindByID(documentID)
	if err != nil {
		logx.Errorf("[UploadService] 查找文档失败: documentID=%d, err=%v", documentID, err)
		return nil, err
	}
	if doc == nil {
		logx.Warnf("[UploadService] 文档不存在: documentID=%d", documentID)
		return nil, ErrDocumentNotFound
	}
	spaceID := doc.SpaceID
	logx.Infof("[UploadService] 文档存在: spaceID=%d", spaceID)

	// 校验用户在该空间是否有编辑权限（privilege >= 1）
	privilege, err := s.spaceService.GetUserSpacePrivilege(spaceID, userID)
	if err != nil {
		logx.Errorf("[UploadService] 获取用户权限失败: spaceID=%d, userID=%d, err=%v", spaceID, userID, err)
		return nil, err
	}
	logx.Infof("[UploadService] 用户权限: privilege=%d (要求 >= 1)", privilege)
	if privilege < 1 {
		logx.Warnf("[UploadService] 权限不足: spaceID=%d, userID=%d, privilege=%d", spaceID, userID, privilege)
		return nil, ErrNoEditPermission
	}

	safeName, err := utils.SanitizeFilename(header.Filename)
	if err != nil {
		logx.Warnf("[UploadService] 文件名清洗失败: %s, err=%v", header.Filename, err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidFilename, err)
	}
	logx.Infof("[UploadService] 文件名清洗: %s -> %s", header.Filename, safeName)
	header.Filename = safeName

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedAttachmentExts[ext] {
		logx.Warnf("[UploadService] 不支持的文件类型: ext=%s, filename=%s", ext, header.Filename)
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFileType, ext)
	}
	logx.Infof("[UploadService] 文件类型校验通过: ext=%s", ext)

	if header.Size > maxAttachmentSize {
		logx.Warnf("[UploadService] 文件过大: size=%d, max=%d", header.Size, maxAttachmentSize)
		return nil, fmt.Errorf("%w (max %dMB)", ErrFileTooLarge, maxAttachmentSize/(1024*1024))
	}
	logx.Infof("[UploadService] 文件大小校验通过: size=%d", header.Size)

	magicTypes := []string{"jpg", "jpeg", "png", "gif", "pdf", "zip", "rar", "7z", "docx", "xlsx", "pptx"}
	if err := utils.ValidateFileMagicNumber(header, magicTypes); err != nil {
		logx.Warnf("[UploadService] magic number 校验失败: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrFileValidationFailed, err)
	}
	logx.Infof("[UploadService] magic number 校验通过")

	timestamp := time.Now().Unix()
	randomStr := strconv.FormatInt(time.Now().UnixNano()%100000, 10)
	filename := fmt.Sprintf("%d_%s%s", timestamp, randomStr, ext)

	// 根据扩展名自动识别附件分类（图片/音频/视频/普通附件）
	source, mediaType := workspacemodel.ClassifyAttachmentByExt(header.Filename)

	relativePath := fmt.Sprintf("attachment/%d/%d/%s", spaceID, documentID, filename)
	accessURL := "/attachments/" + strings.TrimPrefix(relativePath, "attachment/")
	attachment := &workspacemodel.Attachment{
		UserID:     userID,
		DocumentID: documentID,
		Path:       relativePath,
		Source:     source,
		MediaType:  mediaType,
		Name:       header.Filename,
		Size:       header.Size,
	}

	// P2：db 模式把二进制按 4MB 切块写 aw_attachment_chunk（元数据行+块行同一事务）；
	// file 模式保持原磁盘行为。任一块失败整体回滚，不留半条附件。
	if workspacerepo.UseDB() {
		logx.Infof("[UploadService] 切块入库: path=%s", relativePath)
		err := s.attachmentRepo.Transaction(func(tx *gorm.DB) error {
			attachment.CreateTime = int(time.Now().Unix())
			attachment.UpdateTime = attachment.CreateTime
			if err := tx.Create(attachment).Error; err != nil {
				return err
			}
			return workspacerepo.WriteAttachmentChunks(tx, attachment, file)
		})
		if err != nil {
			logx.Errorf("[UploadService] 块写入失败: err=%v", err)
			return nil, err
		}
		logx.Infof("[UploadService] 入库成功: attachmentID=%d, chunks=%d, size=%d", attachment.ID, attachment.ChunkCount, attachment.Size)
	} else {
		attachmentDir := config.AppConfig.Storage.AttachmentDir
		if attachmentDir == "" {
			attachmentDir = "./data/attachment"
		}
		dirPath := filepath.Join(attachmentDir, fmt.Sprintf("%d", spaceID), fmt.Sprintf("%d", documentID))
		logx.Infof("[UploadService] 创建目录: %s", dirPath)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			logx.Errorf("[UploadService] 创建目录失败: %s, err=%v", dirPath, err)
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		filePath := filepath.Join(dirPath, filename)
		logx.Infof("[UploadService] 目标文件路径: %s", filePath)

		dst, err := os.Create(filePath)
		if err != nil {
			logx.Errorf("[UploadService] 创建文件失败: %s, err=%v", filePath, err)
			return nil, fmt.Errorf("failed to create file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			logx.Errorf("[UploadService] 写入文件失败: %s, err=%v", filePath, err)
			os.Remove(filePath)
			return nil, fmt.Errorf("failed to save file: %w", err)
		}
		logx.Infof("[UploadService] 文件写入成功")

		logx.Infof("[UploadService] 写入数据库: name=%s, source=%d, mediaType=%s", attachment.Name, source, mediaType)
		if err := s.attachmentRepo.Create(attachment); err != nil {
			logx.Errorf("[UploadService] 数据库写入失败: err=%v", err)
			os.Remove(filePath)
			return nil, err
		}
	}
	logx.Infof("[UploadService] 数据库写入成功: attachmentID=%d", attachment.ID)

	return &UploadAttachmentResult{
		ID:         attachment.ID,
		DocumentID: attachment.DocumentID,
		Name:       attachment.Name,
		Size:       attachment.Size,
		Path:       relativePath,
		URL:        accessURL,
		CreateTime: attachment.CreateTime,
	}, nil
}

func (s *AttachmentService) GetDocumentAttachments(documentID int, source int) ([]workspacemodel.AttachmentResponse, error) {
	var attachments []*workspacemodel.Attachment
	var err error

	if source == -1 {
		attachments, err = s.attachmentRepo.FindByDocumentID(documentID)
	} else {
		attachments, err = s.attachmentRepo.FindByDocumentIDAndSource(documentID, source)
	}
	if err != nil {
		return nil, err
	}

	responses := make([]workspacemodel.AttachmentResponse, len(attachments))
	for i, a := range attachments {
		responses[i] = a.ToResponse()
	}
	return responses, nil
}

// GetRawAttachmentsByDocumentID 返回附件原始模型列表（供导出等功能使用）
func (s *AttachmentService) GetRawAttachmentsByDocumentID(documentID int) ([]*workspacemodel.Attachment, error) {
	return s.attachmentRepo.FindByDocumentID(documentID)
}

// CopyAttachmentData 把附件二进制写到 w（P3 导出路径）：
// storage_kind=1 走块表流式（内存峰值一个块）；storage_kind=2 与历史未迁移记录读磁盘。
func (s *AttachmentService) CopyAttachmentData(att *workspacemodel.Attachment, w io.Writer) error {
	if att.StorageKind == workspacemodel.AttachmentStorageDB {
		return s.attachmentRepo.StreamAttachmentChunks(att.ID, w)
	}
	fullPath := ResolveAttachmentPath(att.Path)
	if fullPath == "" {
		return ErrFileNotFound
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

// ReadAttachmentBytes 按存储路径（aw_attachment.path 形式：images/... 或 attachment/...）读附件全部字节。
// db 块表优先，磁盘兜底；供导出内联（HTML/Markdown/Word base64）使用。
func (s *AttachmentService) ReadAttachmentBytes(storedPath string) ([]byte, error) {
	storedPath = strings.SplitN(normalizeStoredPath(storedPath), "?", 2)[0]
	storedPath = strings.TrimPrefix(storedPath, "/")
	if storedPath == "" || strings.Contains(storedPath, "..") {
		return nil, ErrAttachmentNotFound
	}
	if att, err := s.attachmentRepo.FindByPath(storedPath); err == nil && att != nil &&
		workspacerepo.UseDB() && att.StorageKind == workspacemodel.AttachmentStorageDB {
		var buf bytes.Buffer
		if err := s.attachmentRepo.StreamAttachmentChunks(att.ID, &buf); err == nil {
			return buf.Bytes(), nil
		}
		// 块读取失败回磁盘兜底，磁盘也没有才报错
	}
	fullPath := ResolveAttachmentPath(storedPath)
	if fullPath == "" {
		return nil, ErrAttachmentNotFound
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, ErrAttachmentNotFound
	}
	return data, nil
}

func (s *AttachmentService) DeleteAttachment(attachmentID int, userID int) error {
	attachment, err := s.attachmentRepo.FindByID(attachmentID)
	if err != nil || attachment == nil {
		return ErrAttachmentNotFound
	}

	// 鉴权必须是 fail-closed：旧实现「查得到文档才校验权限」，
	// 而软删文档会让 FindByID 返回 ErrDocumentNotFound —— 把附件所属文档丢进回收站，
	// 任何登录用户就能凭 attachment_id 删除他人附件。解析不到文档时仅允许上传者本人删除。
	if attachment.DocumentID > 0 {
		doc, derr := s.docRepo.FindByID(attachment.DocumentID)
		if derr == nil && doc != nil {
			privilege, perr := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
			if perr != nil {
				return perr
			}
			if privilege < 1 {
				return ErrNoEditPermission
			}
		} else if attachment.UserID == 0 || attachment.UserID != userID {
			return ErrNoAccessPermission
		}
		s.removeAttachmentRefFromDoc(attachment)
	} else if attachment.UserID == 0 || attachment.UserID != userID {
		// 无主文档的全局/历史附件同样不得被任意用户删除
		return ErrNoAccessPermission
	}

	// P2：元数据行 + 块行同一事务删除；kind=1 记录本无磁盘文件，Remove 报 not-exist 直接忽略
	if err := s.attachmentRepo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&workspacemodel.Attachment{}, attachmentID).Error; err != nil {
			return err
		}
		return workspacerepo.DeleteAttachmentChunksTx(tx, attachmentID)
	}); err != nil {
		return err
	}
	if fullPath := ResolveAttachmentPath(attachment.Path); fullPath != "" {
		if rmErr := os.Remove(fullPath); rmErr != nil && !os.IsNotExist(rmErr) {
			logx.Warnf("删除附件文件失败 %s: %v", fullPath, rmErr)
		}
	}
	return nil
}

// DownloadAttachment 返回只读流 + 原始文件名 + 大小。
// db 模式且 storage_kind=1：从 aw_attachment_chunk 按块流式读取（经 io.Pipe，内存峰值一个块）；
// file 模式及未迁移的 kind=2 历史记录仍读磁盘。调用方负责关闭返回的 reader。
func (s *AttachmentService) DownloadAttachment(attachmentID int, userID int) (rc io.ReadCloser, fileName string, size int64, err error) {
	attachment, err := s.attachmentRepo.FindByID(attachmentID)
	if err != nil || attachment == nil {
		return nil, "", 0, ErrAttachmentNotFound
	}

	if attachment.DocumentID > 0 {
		doc, err := s.docRepo.FindByID(attachment.DocumentID)
		if err != nil || doc == nil {
			return nil, "", 0, ErrDocumentNotFound
		}

		space, err := s.spaceRepo.FindByID(doc.SpaceID)
		if err != nil || space == nil {
			return nil, "", 0, ErrSpaceNotFound
		}

		if !space.IsPublic() && !(userID > 0 && space.IsLoggedInVisible()) {
			spaceUser, err := s.spaceRepo.FindSpaceUser(doc.SpaceID, userID)
			if err != nil {
				return nil, "", 0, err
			}
			if spaceUser == nil {
				return nil, "", 0, ErrNoAccessPermission
			}
		}
	}

	if workspacerepo.UseDB() && attachment.StorageKind == workspacemodel.AttachmentStorageDB {
		pr, pw := io.Pipe()
		// safego 兜 panic（仓库协程守卫要求）；defer Close 保证 panic 时读端不挂死
		safego.Go("StreamAttachmentChunks", func() {
			defer pw.Close()
			pw.CloseWithError(s.attachmentRepo.StreamAttachmentChunks(attachmentID, pw))
		})
		return pr, attachment.Name, attachment.Size, nil
	}

	fullPath := ResolveAttachmentPath(attachment.Path)
	if fullPath == "" {
		return nil, "", 0, ErrFileNotFound
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, "", 0, ErrFileNotFound
	}
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		f.Close()
		return nil, "", 0, ErrFileNotFound
	}

	return f, attachment.Name, info.Size(), nil
}

func (s *AttachmentService) removeAttachmentRefFromDoc(attachment *workspacemodel.Attachment) {
	content, err := s.docService.GetDocumentContent(attachment.DocumentID)
	if err != nil || content == "" {
		return
	}

	attPath := attachment.Path
	escapedPath := regexp.QuoteMeta(attPath)

	var re *regexp.Regexp
	if attachment.Source == workspacemodel.AttachmentSourceImage {
		re = regexp.MustCompile(`(?m)!\[[^\]]*\]\(<?/?` + escapedPath + `>?\)[^\n]*`)
	} else {
		urlPath := regexp.QuoteMeta(strings.TrimPrefix(attPath, "attachment/"))
		re = regexp.MustCompile(`(?m)(\[[^\]]*\]\(<?/?` + escapedPath + `>?\)|\[[^\]]*\]\(<?/?attachments/` + urlPath + `>?\))`)
	}

	newContent := re.ReplaceAllString(content, "")
	newContent = regexp.MustCompile(`\n{3,}`).ReplaceAllString(newContent, "\n\n")

	if newContent != content {
		doc, err := s.docRepo.FindByID(attachment.DocumentID)
		if err != nil || doc == nil {
			return
		}
		if err := s.docService.UpdateDocumentContent(doc, newContent); err != nil {
			logx.Warnf("Failed to remove attachment ref from doc %d: %v", attachment.DocumentID, err)
		}
	}
}

func GetAttachmentFilenameForDownload(name string) string {
	return url.PathEscape(name)
}

// normalizeStoredPath 统一 aw_attachment.path 里的分隔符。
// 历史数据中存在 `attachment\4\486\a.png` 这种反斜杠形态（Windows 上传时写入），
// 而 filepath.ToSlash 只在 Windows 上把 `\` 换成 `/`，Linux 上原样返回 ——
// 同一个库在两种系统上会解析到不同目录，所以这里显式替换，与宿主平台无关。
func normalizeStoredPath(p string) string {
	return strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
}

// ResolveAttachmentPath 把 aw_attachment.path 解析为磁盘路径。
// 库中存在两种历史前缀：images/（旧图片目录）与 attachment/（统一附件目录），
// 下载/删除/导出此前各自按单一目录拼路径，导致旧图片与图片类附件找不到文件。
func ResolveAttachmentPath(storedPath string) string {
	storedPath = normalizeStoredPath(storedPath)
	if storedPath == "" || strings.Contains(storedPath, "..") {
		return ""
	}

	attachmentDir := config.AppConfig.Storage.AttachmentDir
	if attachmentDir == "" {
		attachmentDir = "./data/attachment"
	}
	imageDir := config.AppConfig.Storage.ImageDir
	if imageDir == "" {
		imageDir = "./data/images"
	}

	switch {
	case strings.HasPrefix(storedPath, "attachment/"):
		return filepath.Join(attachmentDir, strings.TrimPrefix(storedPath, "attachment/"))
	case strings.HasPrefix(storedPath, "images/"):
		return filepath.Join(imageDir, strings.TrimPrefix(storedPath, "images/"))
	}

	// 无前缀的历史记录：附件目录优先，其次旧图片目录
	candidate := filepath.Join(attachmentDir, storedPath)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filepath.Join(imageDir, storedPath)
}
