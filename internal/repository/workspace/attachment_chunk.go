package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/utils/config"
	"gorm.io/gorm"
)

// ChunkSize 附件分块大小：与 dbdump 单条语句缓冲上限一致（docs/citebase-落盘改落库重构分析.md §3.1）。
const ChunkSize = 4 * 1024 * 1024

// UseDB 附件来源是否为数据库（storage.attachment_source=db）。
func UseDB() bool {
	return config.AppConfig.Storage.AttachmentSource == "db"
}

// WriteAttachmentChunks 把 r 按 ChunkSize 切块写入 aw_attachment_chunk（须在事务内调用），
// 并回填 att 的 Size/ChunkCount/SHA256/StorageKind，由调用方连同元数据行一起提交。
// 边读边写边算 sha256，内存峰值一个块，不整读文件。
func WriteAttachmentChunks(tx *gorm.DB, att *workspacemodel.Attachment, r io.Reader) error {
	h := sha256.New()
	buf := make([]byte, ChunkSize)
	var seq int
	var total int64
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			if e := tx.Create(&workspacemodel.AttachmentChunk{
				AttachmentID: att.ID,
				Seq:          seq,
				Len:          n,
				Data:         buf[:n],
			}).Error; e != nil {
				return e
			}
			h.Write(buf[:n])
			total += int64(n)
			seq++
		}
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
	}
	att.Size = total
	att.ChunkCount = seq
	att.SHA256 = hex.EncodeToString(h.Sum(nil))
	att.StorageKind = workspacemodel.AttachmentStorageDB
	// 回写元数据行：Create 发生在读文件之前（需要 ID 建块行），
	// 那时 Size/SHA256/ChunkCount 还是未知值，必须在这里补上，否则回读走错分支
	return tx.Model(&workspacemodel.Attachment{}).
		Where("attachment_id = ?", att.ID).
		Updates(map[string]interface{}{
			"size":         att.Size,
			"chunk_count":  att.ChunkCount,
			"sha256":       att.SHA256,
			"storage_kind": workspacemodel.AttachmentStorageDB,
		}).Error
}

// DeleteAttachmentChunksTx 事务内删除一个附件的全部块。
func DeleteAttachmentChunksTx(tx *gorm.DB, attachmentID int) error {
	return tx.Where("attachment_id = ?", attachmentID).
		Delete(&workspacemodel.AttachmentChunk{}).Error
}

// StreamAttachmentChunks 按 seq 顺序把附件二进制写到 w；内存峰值一个块。
// 必须用 Raw：Table().Select().Rows() 会带出全部模型列，Scan 列数不匹配。
func (r *AttachmentRepository) StreamAttachmentChunks(attachmentID int, w io.Writer) error {
	rows, err := r.db.Raw("SELECT data FROM aw_attachment_chunk WHERE attachment_id = ? ORDER BY seq", attachmentID).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ChunkIntegrity 校验一个附件的分块完整性：块数 == chunk_count 且 SUM(len) == size。
func (r *AttachmentRepository) ChunkIntegrity(attachmentID, chunkCount int, size int64) error {
	var cnt int64
	var total int64
	err := r.db.Model(&workspacemodel.AttachmentChunk{}).
		Select("COUNT(*)", "COALESCE(SUM(len),0)").
		Where("attachment_id = ?", attachmentID).
		Row().Scan(&cnt, &total)
	if err != nil {
		return err
	}
	if int(cnt) != chunkCount || total != size {
		return &ChunkIntegrityError{
			AttachmentID: attachmentID,
			WantChunks:   chunkCount, GotChunks: int(cnt),
			WantSize: size, GotSize: total,
		}
	}
	return nil
}

// ChunkIntegrityError 分块完整性校验失败
type ChunkIntegrityError struct {
	AttachmentID          int
	WantChunks, GotChunks int
	WantSize, GotSize     int64
}

func (e *ChunkIntegrityError) Error() string {
	return fmt.Sprintf("attachment %d chunk mismatch: chunks %d/%d, bytes %d/%d",
		e.AttachmentID, e.GotChunks, e.WantChunks, e.GotSize, e.WantSize)
}
