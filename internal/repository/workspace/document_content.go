package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DocumentContentRepository 文档正文数据访问层（aw_document_content）
type DocumentContentRepository struct {
	db *gorm.DB
}

func NewDocumentContentRepository() *DocumentContentRepository {
	return &DocumentContentRepository{db: repos.DB}
}

// Get 按文档 ID 读取正文；正文缺失返回空串（与旧「文件不存在返回空内容」语义一致）。
func (r *DocumentContentRepository) Get(documentID int) (string, error) {
	var row workspacemodel.DocumentContent
	err := r.db.Select("content").Where("document_id = ?", documentID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Content, nil
}

// Upsert 写入/覆盖正文
func (r *DocumentContentRepository) Upsert(documentID int, content string) error {
	return r.UpsertTx(r.db, documentID, content)
}

// UpsertTx 在指定事务句柄中写入正文（用于文档创建与文档行同事务）
func (r *DocumentContentRepository) UpsertTx(tx *gorm.DB, documentID int, content string) error {
	row := NewDocumentContentRow(documentID, content)
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "document_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"content", "content_hash", "content_size", "update_time",
		}),
	}).Create(row).Error
}

// Delete 删除正文
func (r *DocumentContentRepository) Delete(documentID int) error {
	return r.db.Where("document_id = ?", documentID).Delete(&workspacemodel.DocumentContent{}).Error
}

// NewDocumentContentRow 组装正文行（sha256 / 字节数 / 更新时间）
func NewDocumentContentRow(documentID int, content string) *workspacemodel.DocumentContent {
	sum := sha256.Sum256([]byte(content))
	return &workspacemodel.DocumentContent{
		DocumentID:  documentID,
		Content:     content,
		ContentHash: hex.EncodeToString(sum[:]),
		ContentSize: len(content),
		UpdateTime:  int(time.Now().Unix()),
	}
}
