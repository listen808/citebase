package search

import "sort"

// RankedResult represents a search result with a document ID and score,
// used internally by fusion algorithms.
type RankedResult struct {
	DocumentID uint
	Score      float64
	// Source tracks which search system produced this result ("fulltext", "vector", "hybrid").
	Source string
}

// ReciprocalRankFusion implements the RRF algorithm:
//
//	score(d) = sum( 1 / (k + rank_i) ) for each ranked list i that contains d
//
// k is a smoothing constant (typically 60). Higher k compresses score differences
// between high-ranked and low-ranked items.
func ReciprocalRankFusion(results [][]RankedResult, k int) []RankedResult {
	if k <= 0 {
		k = 60
	}

	scores := make(map[uint]float64)

	for _, list := range results {
		for rank, r := range list {
			scores[r.DocumentID] += 1.0 / float64(k+rank+1)
		}
	}

	merged := make([]RankedResult, 0, len(scores))
	for docID, score := range scores {
		merged = append(merged, RankedResult{
			DocumentID: docID,
			Score:      score,
			Source:     "rrf",
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	return merged
}

// WeightedLinearFusion combines two ranked lists using weighted linear combination:
//
//	score(d) = w1 * norm_score_fulltext(d) + w2 * norm_score_vector(d)
//
// Scores are normalized to [0,1] within each list before combining.
func WeightedLinearFusion(fulltextResults, vectorResults []RankedResult, w1, w2 float64) []RankedResult {
	normFT := normalizeScores(fulltextResults)
	normVec := normalizeScores(vectorResults)

	scores := make(map[uint]float64)

	for _, r := range normFT {
		scores[r.DocumentID] += w1 * r.Score
	}
	for _, r := range normVec {
		scores[r.DocumentID] += w2 * r.Score
	}

	merged := make([]RankedResult, 0, len(scores))
	for docID, score := range scores {
		merged = append(merged, RankedResult{
			DocumentID: docID,
			Score:      score,
			Source:     "linear",
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	return merged
}

// normalizeScores returns a copy of results with scores normalized to [0,1]
// based on the maximum score in the list.
func normalizeScores(results []RankedResult) []RankedResult {
	if len(results) == 0 {
		return nil
	}

	maxScore := 0.0
	for _, r := range results {
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	if maxScore <= 0 {
		maxScore = 1.0
	}

	normalized := make([]RankedResult, len(results))
	for i, r := range results {
		normalized[i] = RankedResult{
			DocumentID: r.DocumentID,
			Score:      r.Score / maxScore,
			Source:     r.Source,
		}
	}

	return normalized
}
