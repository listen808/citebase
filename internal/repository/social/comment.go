package social

import (
	"database/sql"
	"errors"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrCommentNotFound = errors.New("comment not found")
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository() *CommentRepository {
	return &CommentRepository{db: repos.DB}
}

func (r *CommentRepository) Create(comment *socialmodel.Comment) error {
	return r.db.Create(comment).Error
}

func (r *CommentRepository) GetByDocumentID(documentID int) ([]*socialmodel.Comment, error) {
	var allComments []*socialmodel.Comment
	err := r.db.Table("aw_comment").
		Select("aw_comment.*, aw_user.given_name as creator_name").
		Joins("LEFT JOIN aw_user ON aw_comment.creator_id = aw_user.user_id").
		Where("aw_comment.document_id = ?", documentID).
		Order("aw_comment.create_time ASC").
		Scan(&allComments).Error
	if err != nil {
		return nil, err
	}

	// Build nested tree: parent -> replies
	commentMap := make(map[int]*socialmodel.Comment)
	for _, c := range allComments {
		commentMap[c.ID] = c
	}

	var roots []*socialmodel.Comment
	for _, c := range allComments {
		if c.ParentID == 0 {
			roots = append(roots, c)
		} else if parent, ok := commentMap[c.ParentID]; ok {
			parent.Replies = append(parent.Replies, c)
		} else {
			// Orphan reply — treat as top-level
			roots = append(roots, c)
		}
	}

	return roots, nil
}

func (r *CommentRepository) FindByID(id int) (*socialmodel.Comment, error) {
	var comment socialmodel.Comment
	err := r.db.First(&comment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}
	return &comment, nil
}

func (r *CommentRepository) Delete(id int) error {
	// Delete the comment and its replies
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete replies first
		if err := tx.Delete(&socialmodel.Comment{}, "parent_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&socialmodel.Comment{}, "id = ?", id).Error
	})
}

func (r *CommentRepository) Resolve(id int, resolved int) error {
	return r.db.Model(&socialmodel.Comment{}).
		Where("id = ?", id).
		Update("resolved", resolved).Error
}
