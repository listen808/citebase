package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type QdrantVectorStore struct {
	endpoint    string
	collection  string
	apiKey      string
	client      *http.Client
	dims        int
	initialized bool
}

func NewQdrantVectorStore(endpoint, apiKey string) *QdrantVectorStore {
	return &QdrantVectorStore{
		endpoint:   strings.TrimRight(endpoint, "/"),
		collection: "citebase_vectors",
		apiKey:     apiKey,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (qs *QdrantVectorStore) initCollection(dims int) error {
	if qs.initialized {
		return nil
	}

	if dims <= 0 {
		dims = 1536
	}
	qs.dims = dims

	url := fmt.Sprintf("%s/collections/%s", qs.endpoint, qs.collection)
	respBody, err := qs.doGet(url)
	if err == nil {
		var resp struct {
			Result struct {
				Status string `json:"status"`
			} `json:"result"`
		}
		if err := json.Unmarshal(respBody, &resp); err == nil && resp.Result.Status == "green" {
			qs.initialized = true
			logx.Infof("[Qdrant] Collection '%s' already exists", qs.collection)
			return nil
		}
	}

	createURL := fmt.Sprintf("%s/collections/%s", qs.endpoint, qs.collection)
	reqBody := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     dims,
			"distance": "Cosine",
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("qdrant marshal: %w", err)
	}
	_, err = qs.doPut(createURL, bodyBytes)
	if err != nil {
		return fmt.Errorf("qdrant create collection: %w", err)
	}

	qs.initialized = true
	logx.Infof("[Qdrant] Collection '%s' created (dims=%d, distance=Cosine)", qs.collection, dims)
	return nil
}

func (qs *QdrantVectorStore) StoreVector(docID int, spaceID int, name string, vector []float64) error {
	if len(vector) == 0 {
		return nil
	}

	if err := qs.initCollection(len(vector)); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/collections/%s/points", qs.endpoint, qs.collection)
	point := map[string]interface{}{
		"id":     docID,
		"vector": vector,
		"payload": map[string]interface{}{
			"space_id": spaceID,
			"name":     name,
		},
	}

	reqBody := map[string]interface{}{
		"points": []map[string]interface{}{point},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("qdrant marshal: %w", err)
	}

	respBody, err := qs.doPut(url, bodyBytes)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "Conflict") {
			return qs.upsertVector(docID, spaceID, name, vector)
		}
		return fmt.Errorf("qdrant upsert: %w", err)
	}
	_ = respBody
	return nil
}

func (qs *QdrantVectorStore) upsertVector(docID int, spaceID int, name string, vector []float64) error {
	url := fmt.Sprintf("%s/collections/%s/points", qs.endpoint, qs.collection)
	point := map[string]interface{}{
		"id":     docID,
		"vector": vector,
		"payload": map[string]interface{}{
			"space_id": spaceID,
			"name":     name,
		},
	}

	reqBody := map[string]interface{}{
		"points": []map[string]interface{}{point},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	_, err = qs.doPut(url, bodyBytes)
	return err
}

func (qs *QdrantVectorStore) DeleteVector(docID int) error {
	url := fmt.Sprintf("%s/collections/%s/points/delete", qs.endpoint, qs.collection)
	reqBody := map[string]interface{}{
		"points": []int{docID},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	_, err = qs.doPost(url, bodyBytes)
	return err
}

// DeleteDocVectors 删除文档及其所有 chunk 向量
func (qs *QdrantVectorStore) DeleteDocVectors(docID int, maxChunks int) error {
	url := fmt.Sprintf("%s/collections/%s/points/delete", qs.endpoint, qs.collection)
	points := make([]int, 0, maxChunks+1)
	points = append(points, docID)
	for i := 0; i < maxChunks; i++ {
		points = append(points, docID*1000+i)
	}
	reqBody := map[string]interface{}{
		"points": points,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	_, err = qs.doPost(url, bodyBytes)
	return err
}

func (qs *QdrantVectorStore) DeleteVectorsByPrefix(prefix string) error {
	return nil
}

func (qs *QdrantVectorStore) Search(queryVec []float64, limit int) ([]VectorSearchResult, error) {
	if len(queryVec) == 0 {
		return nil, nil
	}

	if err := qs.initCollection(len(queryVec)); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/collections/%s/search", qs.endpoint, qs.collection)
	reqBody := map[string]interface{}{
		"vector":       queryVec,
		"limit":        limit,
		"with_payload": true,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("qdrant marshal: %w", err)
	}

	respBody, err := qs.doPost(url, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}

	var resp struct {
		Result []struct {
			ID      int                    `json:"id"`
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("qdrant parse search response: %w", err)
	}

	var results []VectorSearchResult
	for _, r := range resp.Result {
		spaceID := 0
		docName := ""
		if r.Payload != nil {
			if sid, ok := r.Payload["space_id"]; ok {
				switch v := sid.(type) {
				case float64:
					spaceID = int(v)
				case json.Number:
					if n, err := v.Int64(); err == nil {
						spaceID = int(n)
					}
				}
			}
			if n, ok := r.Payload["name"]; ok {
				if s, ok := n.(string); ok {
					docName = s
				}
			}
		}

		if r.Score > 0 {
			results = append(results, VectorSearchResult{
				DocID:   r.ID,
				SpaceID: spaceID,
				Name:    docName,
				Score:   r.Score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func (qs *QdrantVectorStore) DocCount() int {
	url := fmt.Sprintf("%s/collections/%s", qs.endpoint, qs.collection)
	respBody, err := qs.doGet(url)
	if err != nil {
		return 0
	}
	var resp struct {
		Result struct {
			PointsCount int `json:"points_count"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return 0
	}
	return resp.Result.PointsCount
}

func (qs *QdrantVectorStore) Close() error {
	return nil
}

func (qs *QdrantVectorStore) doGet(url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}
	qs.setAuth(req)
	return qs.doRequest(req)
}

func (qs *QdrantVectorStore) doPost(url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	qs.setAuth(req)
	return qs.doRequest(req)
}

func (qs *QdrantVectorStore) doPut(url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	qs.setAuth(req)
	return qs.doRequest(req)
}

func (qs *QdrantVectorStore) setAuth(req *http.Request) {
	if qs.apiKey != "" {
		req.Header.Set("api-key", qs.apiKey)
	}
}

func (qs *QdrantVectorStore) doRequest(req *http.Request) ([]byte, error) {
	resp, err := qs.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("qdrant HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

var _ VectorStoreInterface = (*QdrantVectorStore)(nil)
