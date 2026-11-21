package workspace

import (
	"strconv"
	"strings"
)

// ============================================================
// 附件 / 图片模型
// 对应表：aw_attachment
// ============================================================

// Attachment 附件模型
type Attachment struct {
	ID         int     `gorm:"column:attachment_id;primaryKey;autoIncrement" json:"id"`
	UserID     int     `gorm:"column:user_id" json:"user_id"`
	DocumentID int     `gorm:"column:document_id" json:"document_id"`
	Name       string  `gorm:"column:name;size:255" json:"name"`
	Path       string  `gorm:"column:path;size:100;uniqueIndex:uk_attachment_path" json:"path"`
	Source     int     `gorm:"column:source" json:"source"` // 0 附件 1 图片 2 音频 3 视频
	Size       int64   `gorm:"column:size;default:0" json:"size"`
	MediaType  string  `gorm:"column:media_type;size:32;default:''" json:"media_type"`       // S18: mime type
	Duration   float64 `gorm:"column:duration;type:decimal(10,2);default:0" json:"duration"` // S18: 音视频时长(秒)
	CreateTime int     `gorm:"column:create_time" json:"create_time"`
	UpdateTime int     `gorm:"column:update_time" json:"update_time"`

	// P2 附件分块入库：storage_kind=1 时二进制在 aw_attachment_chunk，=2 仍在磁盘
	ChunkCount  int    `gorm:"column:chunk_count;not null;default:0" json:"chunk_count"`
	SHA256      string `gorm:"column:sha256;size:64;not null;default:''" json:"sha256"`
	StorageKind int    `gorm:"column:storage_kind;not null;default:2" json:"storage_kind"`
}

// 附件二进制存放位置
const (
	AttachmentStorageDB   = 1 // aw_attachment_chunk
	AttachmentStorageDisk = 2 // 磁盘（迁移期回退）
)

// AttachmentChunk 附件分块：4MB/块，(attachment_id, seq) 复合主键。
// 434MB 历史大件拆 109 块，单块远低于 max_allowed_packet，备份单行不超 dbdump 缓冲。
type AttachmentChunk struct {
	AttachmentID int    `gorm:"column:attachment_id;primaryKey;autoIncrement:false" json:"attachment_id"`
	Seq          int    `gorm:"column:seq;primaryKey;autoIncrement:false" json:"seq"`
	Len          int    `gorm:"column:len;not null" json:"len"` // 本块实际字节数（末块可不足 4MB）
	Data         []byte `gorm:"column:data;type:mediumblob;not null" json:"-"`
}

// TableName 指定表名
func (AttachmentChunk) TableName() string {
	return "aw_attachment_chunk"
}

// TableName 指定表名
func (Attachment) TableName() string {
	return "aw_attachment"
}

// AttachmentSource 附件来源常量
const (
	AttachmentSourceFile  = 0 // 普通附件
	AttachmentSourceImage = 1 // 图片
	AttachmentSourceAudio = 2 // S18: 音频
	AttachmentSourceVideo = 3 // S18: 视频
)

var audioExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".m4a": true,
}

var videoExtensions = map[string]bool{
	".mp4": true, ".webm": true, ".mkv": true, ".avi": true, ".mov": true, ".flv": true,
}

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".webp": true,
}

// ClassifyAttachmentByExt 根据文件扩展名判定附件分类
// 返回 source（0=附件 1=图片 2=音频 3=视频）与 mediaType（MIME 类型）
func ClassifyAttachmentByExt(filename string) (source int, mediaType string) {
	ext := strings.ToLower(filename[strings.LastIndex(filename, "."):])
	if imageExtensions[ext] {
		return AttachmentSourceImage, "image/" + ext[1:]
	}
	if audioExtensions[ext] {
		return AttachmentSourceAudio, "audio/" + ext[1:]
	}
	if videoExtensions[ext] {
		return AttachmentSourceVideo, "video/" + ext[1:]
	}
	return AttachmentSourceFile, ""
}

// AttachmentResponse 附件响应
type AttachmentResponse struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	DocumentID  int    `json:"document_id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Source      int    `json:"source"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`          // 访问 URL
	DownloadURL string `json:"download_url"` // 下载 URL（带原始文件名）
	CreateTime  int    `json:"create_time"`
}

// ToResponse 转换为响应对象
// URL 前缀由文件实际存储路径决定（attachment/ → /attachments/，images/ → /images/）
// 避免图片被标记为 Source=Image 但文件存放在 attachmentDir 却生成 /images/ URL 的不一致问题
func (a *Attachment) ToResponse() AttachmentResponse {
	downloadPrefix := "/api/v1/attachments/download/"
	pathWithoutPrefix := a.Path
	urlPrefix := "/attachments/"
	if strings.HasPrefix(pathWithoutPrefix, "attachment/") {
		pathWithoutPrefix = strings.TrimPrefix(pathWithoutPrefix, "attachment/")
		urlPrefix = "/attachments/"
	} else if strings.HasPrefix(pathWithoutPrefix, "images/") {
		pathWithoutPrefix = strings.TrimPrefix(pathWithoutPrefix, "images/")
		urlPrefix = "/images/"
	}

	return AttachmentResponse{
		ID:          a.ID,
		UserID:      a.UserID,
		DocumentID:  a.DocumentID,
		Name:        a.Name,
		Path:        a.Path,
		Source:      a.Source,
		Size:        a.Size,
		URL:         urlPrefix + pathWithoutPrefix,
		DownloadURL: downloadPrefix + strconv.Itoa(a.ID),
		CreateTime:  a.CreateTime,
	}
}
