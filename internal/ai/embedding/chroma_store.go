package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/utils"
)

type ChromaVectorStore struct {
	endpoint     string
	collection   string
	collectionID string
	client       *http.Client
	dimensions   int
	initialized  bool
	v2BaseURL    string
	initMu       sync.Mutex
}

func NewChromaVectorStore(endpoint string) *ChromaVectorStore {
	ep := strings.TrimRight(endpoint, "/")
	return &ChromaVectorStore{
		endpoint:   ep,
		collection: "citebase_vectors",
		client:     &http.Client{Timeout: 30 * time.Second},
		dimensions: 0,
	}
}

// probeV2Base 探测可用的 Chroma v2 API 基础路径（结果缓存）
// P3：加锁保护 v2BaseURL 读写，避免并发探测
func (cs *ChromaVectorStore) probeV2Base() string {
	cs.initMu.Lock()
	defer cs.initMu.Unlock()
	if cs.v2BaseURL != "" {
		return cs.v2BaseURL
	}

	paths := []string{
		fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections", cs.endpoint),
		fmt.Sprintf("%s/api/v2/collections", cs.endpoint),
	}

	for _, path := range paths {
		req, _ := http.NewRequestWithContext(context.Background(), "HEAD", path, nil)
		resp, err := cs.client.Do(req)
		if err == nil {
			resp.Body.Close()
			cs.v2BaseURL = path
			logx.Infof("[Chroma] v2 base URL detected: %s", path)
			return path
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	// fallback
	cs.v2BaseURL = paths[0]
	return paths[0]
}

// initCollection 使用 Chroma v2 get_or_create 初始化集合（支持失败重试）
func (cs *ChromaVectorStore) initCollection(dimensions int) error {
	if cs.initialized {
		return nil
	}

	cs.initMu.Lock()
	defer cs.initMu.Unlock()

	// 双重检查
	if cs.initialized {
		return nil
	}

	if dimensions <= 0 {
		dimensions = 1536
	}
	cs.dimensions = dimensions

	baseURL := cs.probeV2Base()

	reqBody := map[string]interface{}{
		"name":          cs.collection,
		"get_or_create": true,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("chroma marshal: %w", err)
	}

	respBody, err := cs.doPost(baseURL, bodyBytes, "")
	if err != nil {
		return fmt.Errorf("chroma v2 init collection: %w", err)
	}

	var created struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return fmt.Errorf("chroma parse create response: %w (body: %.200s)", err, string(respBody))
	}

	if created.ID == "" {
		return fmt.Errorf("chroma v2: collection ID empty in response: %.200s", string(respBody))
	}

	cs.collectionID = created.ID
	cs.initialized = true
	logx.Infof("[Chroma] v2 collection '%s' ready (id=%s, url=%s)", cs.collection, cs.collectionID, baseURL)
	return nil
}

// collectionURL 构建集合操作 URL
func (cs *ChromaVectorStore) collectionURL(subPath string) string {
	return fmt.Sprintf("%s/%s/%s", cs.probeV2Base(), cs.collectionID, subPath)
}

func (cs *ChromaVectorStore) StoreVector(docID int, spaceID int, name string, vector []float64) error {
	if len(vector) == 0 {
		return nil
	}
	if err := cs.initCollection(len(vector)); err != nil {
		return err
	}

	reqBody := map[string]interface{}{
		"ids":        []string{fmt.Sprintf("%d", docID)},
		"embeddings": [][]float64{vector},
		"metadatas":  []map[string]interface{}{{"space_id": spaceID, "name": name}},
		"documents":  []string{name},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	_, err = cs.doPost(cs.collectionURL("add"), bodyBytes, "")
	if err != nil {
		// 如果是重复ID错误，用upsert
		if cs.isConflict(err) {
			_, err = cs.doPost(cs.collectionURL("upsert"), bodyBytes, "")
		}
	}
	return err
}

func (cs *ChromaVectorStore) updateVector(docID int, spaceID int, name string, vector []float64) error {
	reqBody := map[string]interface{}{
		"ids":        []string{fmt.Sprintf("%d", docID)},
		"embeddings": [][]float64{vector},
		"metadatas":  []map[string]interface{}{{"space_id": spaceID, "name": name}},
		"documents":  []string{name},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	_, err = cs.doPost(cs.collectionURL("upsert"), bodyBytes, "")
	return err
}

// StoreChunk 存储文档分块向量（chunkID = docID*1000 + chunkIdx）
func (cs *ChromaVectorStore) StoreChunk(docID int, chunkIdx int, spaceID int, name string, content string, vector []float64) error {
	if len(vector) == 0 {
		return nil
	}
	if err := cs.initCollection(len(vector)); err != nil {
		return err
	}

	reqBody := map[string]interface{}{
		"ids":        []string{fmt.Sprintf("%d", docID*1000+chunkIdx)},
		"embeddings": [][]float64{vector},
		"metadatas":  []map[string]interface{}{{"space_id": spaceID, "name": name, "doc_id": docID, "chunk_idx": chunkIdx}},
		"documents":  []string{utils.TruncateRunes(content, 500)},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	_, err = cs.doPost(cs.collectionURL("add"), bodyBytes, "")
	if err != nil && cs.isConflict(err) {
		_, err = cs.doPost(cs.collectionURL("upsert"), bodyBytes, "")
	}
	return err
}

func (cs *ChromaVectorStore) DeleteVector(docID int) error {
	return cs.DeleteDocVectors(docID, 100)
}

func (cs *ChromaVectorStore) DeleteDocVectors(docID int, maxChunks int) error {
	if err := cs.initCollection(0); err != nil {
		return err
	}

	ids := make([]string, 0, maxChunks+1)
	ids = append(ids, fmt.Sprintf("%d", docID))
	for i := 0; i < maxChunks; i++ {
		ids = append(ids, fmt.Sprintf("%d", docID*1000+i))
	}

	reqBody := map[string]interface{}{
		"ids": ids,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	_, err = cs.doPost(cs.collectionURL("delete"), bodyBytes, "")
	return err
}

func (cs *ChromaVectorStore) DeleteVectorsByPrefix(prefix string) error {
	return nil
}

func (cs *ChromaVectorStore) Search(queryVec []float64, limit int) ([]VectorSearchResult, error) {
	if len(queryVec) == 0 {
		return nil, nil
	}
	if err := cs.initCollection(len(queryVec)); err != nil {
		return nil, err
	}

	reqBody := map[string]interface{}{
		"query_embeddings": [][]float64{queryVec},
		"n_results":        limit,
		"include":          []string{"metadatas", "distances", "documents"},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("chroma marshal: %w", err)
	}

	respBody, err := cs.doPost(cs.collectionURL("query"), bodyBytes, "")
	if err != nil {
		return nil, fmt.Errorf("chroma query: %w", err)
	}

	var queryResp struct {
		IDs       [][]string                 `json:"ids"`
		Distances [][]float64                `json:"distances"`
		Metadatas [][]map[string]interface{} `json:"metadatas"`
	}
	if err := json.Unmarshal(respBody, &queryResp); err != nil {
		return nil, fmt.Errorf("chroma parse: %w (body: %.200s)", err, string(respBody))
	}

	var results []VectorSearchResult
	if len(queryResp.IDs) == 0 || len(queryResp.IDs[0]) == 0 {
		return results, nil
	}

	for i, idStr := range queryResp.IDs[0] {
		docID := parseDocID(idStr)
		if docID == 0 {
			continue
		}

		score := 1.0
		if i < len(queryResp.Distances) && i < len(queryResp.Distances[0]) {
			score = 1.0 - queryResp.Distances[0][i]
		}

		spaceID := 0
		docName := ""
		if i < len(queryResp.Metadatas) && i < len(queryResp.Metadatas[0]) {
			meta := queryResp.Metadatas[0][i]
			if meta != nil {
				if sid, ok := meta["space_id"]; ok {
					switch v := sid.(type) {
					case float64:
						spaceID = int(v)
					case json.Number:
						if n, err := v.Int64(); err == nil {
							spaceID = int(n)
						}
					}
				}
				if n, ok := meta["name"]; ok {
					if s, ok := n.(string); ok {
						docName = s
					}
				}
			}
		}

		if score > 0 {
			results = append(results, VectorSearchResult{
				DocID:   docID,
				SpaceID: spaceID,
				Name:    docName,
				Score:   score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func (cs *ChromaVectorStore) DocCount() int {
	respBody, err := cs.doGet(cs.collectionURL("count"))
	if err != nil {
		return 0
	}
	var count int
	_ = json.Unmarshal(respBody, &count)
	return count
}

func (cs *ChromaVectorStore) Close() error { return nil }

// isConflict 判断是否是ID冲突错误
func (cs *ChromaVectorStore) isConflict(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "already exists") ||
		strings.Contains(s, "AlreadyExists") ||
		strings.Contains(s, "409") ||
		strings.Contains(s, "UniqueConstraint")
}

// --- HTTP helpers ---

func (cs *ChromaVectorStore) doGet(url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	resp, err := cs.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("chroma HTTP %d: %.200s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (cs *ChromaVectorStore) doPost(url string, body []byte, _ string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("chroma HTTP %d: %.200s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func parseDocID(idStr string) int {
	var docID int
	fmt.Sscanf(idStr, "%d", &docID)
	if docID >= 1000 {
		return docID / 1000
	}
	return docID
}

var _ VectorStoreInterface = (*ChromaVectorStore)(nil)
