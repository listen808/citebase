package knowledge

import (
	"fmt"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	"github.com/listen808/citebase/internal/repository"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
)

type DocClusterService struct {
	clusterRepo *knowledgerepo.AIDocClusterRepository
}

func NewDocClusterService() *DocClusterService {
	return &DocClusterService{
		clusterRepo: knowledgerepo.NewAIDocClusterRepository(repository.DB),
	}
}

type ClusterGroup struct {
	ClusterID     string                        `json:"cluster_id"`
	Documents     []knowledgemodel.AIDocCluster `json:"documents"`
	AvgSimilarity float64                       `json:"avg_similarity"`
}

func (s *DocClusterService) GetClusters(page, pageSize int) ([]*ClusterGroup, int64, error) {
	clusters, total, err := s.clusterRepo.FindAll(page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("query clusters: %w", err)
	}

	groupMap := make(map[string]*ClusterGroup)
	for _, c := range clusters {
		g, ok := groupMap[c.ClusterID]
		if !ok {
			g = &ClusterGroup{ClusterID: c.ClusterID}
			groupMap[c.ClusterID] = g
		}
		g.Documents = append(g.Documents, c)
	}

	groups := make([]*ClusterGroup, 0, len(groupMap))
	for _, g := range groupMap {
		if len(g.Documents) > 0 {
			var sum float64
			for _, d := range g.Documents {
				sum += d.Similarity
			}
			g.AvgSimilarity = sum / float64(len(g.Documents))
		}
		groups = append(groups, g)
	}

	return groups, total, nil
}

func (s *DocClusterService) GetClusterByID(clusterID string) (*ClusterGroup, error) {
	clusters, err := s.clusterRepo.FindByClusterID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("query cluster: %w", err)
	}
	if len(clusters) == 0 {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	var sum float64
	for _, c := range clusters {
		sum += c.Similarity
	}

	return &ClusterGroup{
		ClusterID:     clusterID,
		Documents:     clusters,
		AvgSimilarity: sum / float64(len(clusters)),
	}, nil
}

func (s *DocClusterService) GetPendingClusters() ([]*ClusterGroup, error) {
	clusters, err := s.clusterRepo.FindPendingClusters()
	if err != nil {
		return nil, fmt.Errorf("query pending clusters: %w", err)
	}

	groupMap := make(map[string]*ClusterGroup)
	for _, c := range clusters {
		g, ok := groupMap[c.ClusterID]
		if !ok {
			g = &ClusterGroup{ClusterID: c.ClusterID}
			groupMap[c.ClusterID] = g
		}
		g.Documents = append(g.Documents, c)
	}

	groups := make([]*ClusterGroup, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, g)
	}
	return groups, nil
}
