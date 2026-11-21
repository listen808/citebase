package text2sql

import (
	"fmt"
	"hash/crc32"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// PromptVersionSelector 提示词版本选择器
type PromptVersionSelector struct {
	repo *airepo.PromptVersionRepo
}

func NewPromptVersionSelector(repo *airepo.PromptVersionRepo) *PromptVersionSelector {
	return &PromptVersionSelector{repo: repo}
}

// Select 选择适用的提示词版本
func (s *PromptVersionSelector) Select(spaceID uint64, queryType string, userID string) (*aimodel.PromptVersion, error) {
	versions, err := s.repo.GetByExperiment(spaceID, queryType, "")
	if err != nil || len(versions) == 0 {
		return s.repo.GetActiveByType(spaceID, queryType)
	}

	if len(versions) >= 2 {
		hash := crc32.ChecksumIEEE([]byte(userID + fmt.Sprintf("%d", spaceID)))
		if hash%100 < 50 {
			return &versions[0], nil
		}
		return &versions[1], nil
	}

	return &versions[0], nil
}
