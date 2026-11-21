package embedding

// S7: VectorStoreInterface 向量存储抽象接口
// 支持 BBolt/Milvus/Qdrant 等多种后端
type VectorStoreInterface interface {
	StoreVector(docID int, spaceID int, name string, vector []float64) error
	DeleteVector(docID int) error
	// DeleteDocVectors 删除文档及其所有 chunk 向量（docID*1000+0 到 docID*1000+maxChunks-1）
	DeleteDocVectors(docID int, maxChunks int) error
	DeleteVectorsByPrefix(prefix string) error
	Search(queryVec []float64, limit int) ([]VectorSearchResult, error)
	DocCount() int
	Close() error
}

// Ensure VectorStore implements VectorStoreInterface
var _ VectorStoreInterface = (*VectorStore)(nil)
