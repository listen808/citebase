package rag

import (
	"fmt"
	"path/filepath"
	"strings"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils"
)

func getPageFilePath(doc *workspacemodel.Document, docRepo *workspacerepo.DocumentRepository, spaceRepo *workspacerepo.SpaceRepository, docStorage *utils.DocumentStorage) string {
	space, _ := spaceRepo.FindByID(doc.SpaceID)
	spaceName := fmt.Sprintf("space_%d", doc.SpaceID)
	if space != nil {
		spaceName = space.Name
	}

	if doc.Path == "0" || doc.Path == "" {
		if doc.Name == spaceName && doc.Type == workspacemodel.DocumentTypeDir {
			return filepath.Join(spaceName, utils.DocumentDefaultName+utils.DocumentPageSuffix)
		}
		result := docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
		return result
	}

	pathParts := strings.Split(doc.Path, ",")
	var parentPath string
	skipFirst := true
	for _, part := range pathParts {
		if part == "" || part == "0" {
			continue
		}
		parentID := 0
		fmt.Sscanf(part, "%d", &parentID)
		if parentID > 0 {
			if skipFirst {
				skipFirst = false
				continue
			}
			parentDoc, err := docRepo.FindByID(parentID)
			if err == nil {
				parentPath = parentPath + "/" + parentDoc.Name
			}
		}
	}

	var pageFile string
	if parentPath == "" {
		pageFile = docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
	} else {
		fullPath := spaceName + parentPath
		if doc.Type == workspacemodel.DocumentTypePage {
			pageFile = fullPath + "/" + doc.Name + utils.DocumentPageSuffix
		} else {
			pageFile = fullPath + "/" + doc.Name + "/" + utils.DocumentDefaultName + utils.DocumentPageSuffix
		}
	}
	return pageFile
}
