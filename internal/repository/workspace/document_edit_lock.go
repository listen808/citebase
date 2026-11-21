package workspace

import (
	"errors"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const editLockTTL = 30 * time.Minute

// ErrEditLockHeldByOther 文档正被其他用户锁定（编辑冲突）
var ErrEditLockHeldByOther = errors.New("document is locked by another user")

type DocumentEditLockRepository struct {
	db *gorm.DB
}

func NewDocumentEditLockRepository() *DocumentEditLockRepository {
	return &DocumentEditLockRepository{db: repos.DB}
}

func NewDocumentEditLockRepositoryWithDB(db *gorm.DB) *DocumentEditLockRepository {
	return &DocumentEditLockRepository{db: db}
}

func (r *DocumentEditLockRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// Lock 抢占编辑锁
// 冲突校验：若文档已被其他用户锁定（且锁未过期），返回 ErrEditLockHeldByOther
// 同一用户重复加锁视为续约，允许覆盖
func (r *DocumentEditLockRepository) Lock(docID, userID int, username string, permanent bool) error {
	now := time.Now()
	expireTime := now.Add(-editLockTTL)

	perm := 0
	if permanent {
		perm = 1
	}

	lock := &workspacemodel.DocumentEditLock{
		DocumentID: docID,
		UserID:     userID,
		Username:   username,
		LockedAt:   now,
		Permanent:  perm,
	}

	// 使用事务 + SELECT FOR UPDATE 避免并发抢锁的竞态
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 先清理该文档的过期锁（仅清理非永久锁）
		if err := tx.Where("document_id = ? AND locked_at < ? AND permanent = 0", docID, expireTime).
			Delete(&workspacemodel.DocumentEditLock{}).Error; err != nil {
			return err
		}

		// 查询当前是否仍有其他用户持有的锁（行级锁防止并发竞态）
		// GORM v2 不再识别 v1 的 "gorm:query_option" 设置（会被静默忽略，导致并发抢锁竞态），
		// 必须用 clause.Locking 才会真正生成 SELECT ... FOR UPDATE，锁住锁行防止双写覆盖。
		var existing workspacemodel.DocumentEditLock
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("document_id = ?", docID).
			First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 存在其他用户持有的非过期锁 → 冲突
		if existing.DocumentID > 0 && existing.UserID != userID {
			// 永久锁或未过期的非永久锁，均视为被占用
			if existing.Permanent == 1 || existing.LockedAt.After(expireTime) {
				return ErrEditLockHeldByOther
			}
		}

		// 同一用户或无锁 → 写入/覆盖
		return tx.Save(lock).Error
	})
}

func (r *DocumentEditLockRepository) Unlock(docID, userID int) error {
	return r.db.Where("document_id = ? AND user_id = ?", docID, userID).
		Delete(&workspacemodel.DocumentEditLock{}).Error
}

func (r *DocumentEditLockRepository) Get(docID int) (*workspacemodel.DocumentEditLock, error) {
	// 先清理过期锁（仅清理非永久锁）
	expireTime := time.Now().Add(-editLockTTL)
	r.db.Where("document_id = ? AND locked_at < ? AND permanent = 0", docID, expireTime).
		Delete(&workspacemodel.DocumentEditLock{})

	var lock workspacemodel.DocumentEditLock
	if err := r.db.Where("document_id = ?", docID).First(&lock).Error; err != nil {
		return nil, err
	}
	return &lock, nil
}

func (r *DocumentEditLockRepository) ForceUnlock(docID int) error {
	return r.db.Where("document_id = ?", docID).
		Delete(&workspacemodel.DocumentEditLock{}).Error
}

// CleanupExpiredLocks 清理所有过期的编辑锁（跳过永久锁）
func (r *DocumentEditLockRepository) CleanupExpiredLocks() (int64, error) {
	expireTime := time.Now().Add(-editLockTTL)
	result := r.db.Where("locked_at < ? AND permanent = 0", expireTime).
		Delete(&workspacemodel.DocumentEditLock{})
	return result.RowsAffected, result.Error
}

// Heartbeat 刷新锁的时间，防止编辑过程中锁过期
func (r *DocumentEditLockRepository) Heartbeat(docID, userID int) error {
	return r.db.Model(&workspacemodel.DocumentEditLock{}).
		Where("document_id = ? AND user_id = ?", docID, userID).
		Update("locked_at", time.Now()).Error
}
