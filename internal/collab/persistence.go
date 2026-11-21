package collab

import (
	"time"

	collabmodel "github.com/listen808/citebase/internal/model/collab"
	"gorm.io/gorm"
)

// PersistenceStore handles saving/loading Yjs document state to/from MySQL.
type PersistenceStore struct {
	db *gorm.DB
}

// NewPersistenceStore creates a new PersistenceStore.
func NewPersistenceStore(db *gorm.DB) *PersistenceStore {
	return &PersistenceStore{db: db}
}

// Load retrieves the document state from the database.
func (s *PersistenceStore) Load(docID uint) ([]byte, int, error) {
	var state collabmodel.DocumentCollabState
	err := s.db.Where("document_id = ?", docID).First(&state).Error
	if err != nil {
		return nil, 0, err
	}
	return state.State, state.Version, nil
}

// Save persists the document state to the database.
func (s *PersistenceStore) Save(docID uint, state []byte, version int) error {
	record := collabmodel.DocumentCollabState{
		DocumentID: docID,
		State:      state,
		Version:    version,
		UpdatedAt:  time.Now(),
	}
	// Use upsert: update if exists, create if not
	return s.db.Where("document_id = ?", docID).
		Assign(collabmodel.DocumentCollabState{
			State:     state,
			Version:   version,
			UpdatedAt: time.Now(),
		}).
		FirstOrCreate(&record).Error
}

// Delete removes the persisted state for a document.
func (s *PersistenceStore) Delete(docID uint) error {
	return s.db.Where("document_id = ?", docID).Delete(&collabmodel.DocumentCollabState{}).Error
}
