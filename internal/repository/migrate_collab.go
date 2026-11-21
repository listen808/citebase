package repository

import (
	collabmodel "github.com/listen808/citebase/internal/model/collab"
)

// Collaborative editing tables
func init() {
	RegisterModels(
		&collabmodel.DocumentCollabState{},
		&collabmodel.DocumentCollabSession{},
	)
}
