package ai

import (
	"crypto/sha256"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/rag"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/utils"
)

type VectorSyncOp struct {
	DocID   int
	SpaceID int
	DocName string
	Content string
	OpType  string
}

// maxChunksPerDoc 每个文档最大 chunk 数，与 chunkID = docID*1000+i 公式一致
const maxChunksPerDoc = 1000

type VectorSyncer struct {
	vs          embedding.VectorStoreInterface
	emb         embedding.Embedder
	embIdentity string
	notify      chan struct{}
	quit        chan struct{}
	done        chan struct{}
	mu          sync.Mutex
	started     bool
}

// NewVectorSyncer embIdentity 是 embedding 栈的身份串（provider:model），
// 与内容 SHA256 一起构成文档指纹；换模型即全量重建。
func NewVectorSyncer(vs embedding.VectorStoreInterface, emb embedding.Embedder, embIdentity string) *VectorSyncer {
	return &VectorSyncer{
		vs:          vs,
		emb:         emb,
		embIdentity: embIdentity,
		notify:      make(chan struct{}, 1),
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

func (s *VectorSyncer) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true

	// 清理7天前已完成的任务，避免表堆积
	err := knowledgerepo.VectorSyncTaskRepo().PurgeDone(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		logx.Infof("[VectorSyncer] Warning: failed to purge old tasks: %v", err)
	} else {
		logx.Infof("[VectorSyncer] Purged old completed tasks (older than 7 days)")
	}

	pending, _ := knowledgerepo.VectorSyncTaskRepo().CountByStatus("pending")
	if pending > 0 {
		logx.Infof("[VectorSyncer] Recovered %d pending tasks from DB", pending)
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}

	go s.worker()
	logx.Infof("[VectorSyncer] Started (persistent queue mode)")
}

// P0-5: 简化 Stop 的锁使用——避免 defer Unlock + 手动 Unlock + 手动 Lock 的脆弱组合
// 流程：短暂持锁改状态 → 解锁 → 等待 worker 退出（worker 不需要持锁）
func (s *VectorSyncer) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	close(s.quit)
	s.started = false
	s.mu.Unlock()

	<-s.done
	logx.Infof("[VectorSyncer] Stopped")
}

// docSyncFingerprint P0 增量缓存：embedding 身份 + 内容一起进 SHA256。
// 指纹未变 = 内容没改且 embedding 栈没换 → 跳过本次索引。
func (s *VectorSyncer) docSyncFingerprint(content string) string {
	h := sha256.New()
	h.Write([]byte(s.embIdentity))
	h.Write([]byte{0})
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Enqueue 普通入队：store/update 时若文档指纹未变则直接跳过（P0 增量缓存）。
func (s *VectorSyncer) Enqueue(op VectorSyncOp) {
	s.enqueue(op, false)
}

// EnqueueForce 强制入队：不做指纹判重，重建索引入口用。
func (s *VectorSyncer) EnqueueForce(op VectorSyncOp) {
	s.enqueue(op, true)
}

func (s *VectorSyncer) enqueue(op VectorSyncOp, force bool) {
	if !force && (op.OpType == "store" || op.OpType == "update") && op.Content != "" {
		fp := s.docSyncFingerprint(op.Content)
		state, err := knowledgerepo.VectorSyncStateRepo().GetByDocID(op.DocID)
		if err == nil && state.ContentSHA256 == fp {
			logx.Infof("[VectorSyncer] Doc %d unchanged (fp %s...), skipping sync", op.DocID, fp[:12])
			return
		}
		// 查询失败不拦截：状态表读不到就当没配过，照常入队。
	}

	task := &knowledgemodel.VectorSyncTask{
		DocID:   op.DocID,
		SpaceID: op.SpaceID,
		DocName: op.DocName,
		Content: op.Content,
		OpType:  op.OpType,
		Status:  "pending",
	}
	if err := knowledgerepo.VectorSyncTaskRepo().Create(task); err != nil {
		logx.Infof("[VectorSyncer] Failed to persist task for doc %d: %v", op.DocID, err)
		return
	}

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// recordSyncState 索引成功后落指纹。指纹落库失败不致命：下次会多索引一次，不会漏。
func (s *VectorSyncer) recordSyncState(docID int, content string) {
	fp := s.docSyncFingerprint(content)
	if err := knowledgerepo.VectorSyncStateRepo().Upsert(&knowledgemodel.VectorSyncState{
		DocID:         docID,
		ContentSHA256: fp,
		EmbIdentity:   s.embIdentity,
	}); err != nil {
		logx.Infof("[VectorSyncer] Warning: failed to record sync state for doc %d: %v", docID, err)
	}
}

func (s *VectorSyncer) worker() {
	idleTimer := time.NewTimer(30 * time.Second)
	defer idleTimer.Stop()
	defer close(s.done)

	for {
		s.processBatch()

		idleTimer.Stop()
		idleTimer = time.NewTimer(30 * time.Second)

		select {
		case <-s.notify:
		case <-idleTimer.C:
		case <-s.quit:
			return
		}
	}
}

func (s *VectorSyncer) processBatch() {
	for {
		select {
		case <-s.quit:
			return
		default:
		}

		tasks, err := knowledgerepo.VectorSyncTaskRepo().FetchPending(10)
		if err != nil {
			logx.Errorf("[VectorSyncer] Fetch pending error: %v", err)
			return
		}
		if len(tasks) == 0 {
			return
		}

		for _, task := range tasks {
			select {
			case <-s.quit:
				return
			default:
			}
			if err := knowledgerepo.VectorSyncTaskRepo().MarkProcessing(task.ID); err != nil {
				// P0：原来这里 continue 会立刻重新 FetchPending 拿到同一批任务，
				// 形成无退避忙等（CPU 打满）且完全不检查 quit → Stop 永久挂起。
				// 现在记录错误并退出本轮批处理，由 worker 的 notify/30s 定时器再试。
				logx.Errorf("[VectorSyncer] MarkProcessing task %d failed, aborting batch: %v", task.ID, err)
				return
			}
			s.processTask(task)
		}
	}
}

// failTask 按重试次数把任务标记为可重试失败或死信。
func (s *VectorSyncer) failTask(task knowledgemodel.VectorSyncTask, errMsg string) {
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	if task.RetryCount >= 2 {
		_ = knowledgerepo.VectorSyncTaskRepo().MarkDead(task.ID, errMsg)
		return
	}
	_ = knowledgerepo.VectorSyncTaskRepo().MarkFailed(task.ID, errMsg)
}

// chunkContextPrefix P1-1：进索引的 chunk 带上"文档名 > 章节路径"头。
//
// 没有它，chunk 一离开文档就丢失适用范围——"华东区除外""持有 7 天内费率 1.5%"
// 这类内容单独拿出来不知道属于哪个产品、哪个版本。Anthropic 的 Contextual
// Retrieval 用 LLM 生成这段上下文；这里用分块时已经解析好的 heading，
// 效果弱一些但零成本、零延迟。
func chunkContextPrefix(docName string, chunk rag.Chunk) string {
	parts := make([]string, 0, 2)
	if docName != "" {
		parts = append(parts, docName)
	}
	if chunk.HeadingPath != "" {
		parts = append(parts, chunk.HeadingPath)
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " > ") + "]\n"
}

func (s *VectorSyncer) processTask(task knowledgemodel.VectorSyncTask) {
	if s.vs == nil || s.emb == nil {
		_ = knowledgerepo.VectorSyncTaskRepo().MarkFailed(task.ID, "vector store or embedder not available")
		return
	}

	logx.Infof("[VectorSyncer] Processing task %d: doc=%d op=%s content_len=%d", task.ID, task.DocID, task.OpType, len(task.Content))

	switch task.OpType {
	case "store", "update":
		if task.Content == "" {
			logx.Infof("[VectorSyncer] Task %d: empty content, marking done", task.ID)
			_ = knowledgerepo.VectorSyncTaskRepo().MarkDone(task.ID)
			return
		}

		// P0：先做全部 embedding（最容易失败的远程调用），成功后才删除旧向量。
		// 原实现“先删后写”，一旦 embedding 中断/失败，旧向量已被删除且不重试成功，
		// 索引就永久缺块。现在删除后只剩本地写入，且部分失败会走可重试分支。
		chunker := rag.NewAdaptiveChunker(1500, s.emb, rag.StrategyAuto)
		chunks := chunker.Chunk(task.Content)
		logx.Infof("[VectorSyncer] Task %d: content chunked into %d chunks", task.ID, len(chunks))

		if len(chunks) == 0 {
			content := task.Content
			if len(content) > 8000 {
				content = utils.TruncateRunes(content, 8000)
			}
			logx.Infof("[VectorSyncer] Task %d: no chunks, embedding full content (%d chars)", task.ID, len(content))
			vec, err := s.emb.GetEmbedding(content)
			if err != nil {
				s.failTask(task, "embedding(full): "+err.Error())
				return
			}
			logx.Infof("[VectorSyncer] Task %d: got embedding vector len=%d", task.ID, len(vec))
			if err := s.vs.DeleteDocVectors(task.DocID, maxChunksPerDoc); err != nil {
				logx.Warnf("[VectorSyncer] Task %d: delete old vectors warning: %v", task.ID, err)
			}
			if err := s.vs.StoreVector(task.DocID, task.SpaceID, task.DocName, vec); err != nil {
				s.failTask(task, "store: "+err.Error())
				logx.Infof("[VectorSyncer] Task %d: store error for doc %d: %v", task.ID, task.DocID, err)
				return
			}
			logx.Infof("[VectorSyncer] Task %d: stored vector for doc %d", task.ID, task.DocID)
			_ = knowledgerepo.VectorSyncTaskRepo().MarkDone(task.ID)
			s.recordSyncState(task.DocID, task.Content)
			return
		}

		// 1) 先在内存里生成所有 chunk 向量（不触碰索引）
		texts := make([]string, len(chunks))
		for i, chunk := range chunks {
			ct := chunk.Content
			if len(ct) > 8000 {
				ct = utils.TruncateRunes(ct, 8000)
			}
			// P1-1：chunk 带"身份"进索引（Contextual Retrieval 的零成本版本）。
			// heading 在分块时已经解析出来，不需要额外的 LLM 调用。
			texts[i] = chunkContextPrefix(task.DocName, chunk) + ct
		}

		logx.Infof("[VectorSyncer] Task %d: batch embedding %d chunks", task.ID, len(texts))
		vecs, batchErr := s.emb.GetEmbeddings(texts)
		if batchErr != nil {
			logx.Infof("[VectorSyncer] Task %d: batch embedding error for doc %d, falling back to sequential: %v", task.ID, task.DocID, batchErr)
			vecs = nil
		}
		if len(vecs) != len(chunks) {
			if len(vecs) > 0 {
				logx.Infof("[VectorSyncer] Task %d: vector count mismatch (%d vs %d), falling back to sequential", task.ID, len(vecs), len(chunks))
			}
			vecs = nil
			for i, text := range texts {
				vec, err := s.emb.GetEmbedding(text)
				if err != nil {
					logx.Infof("[VectorSyncer] Task %d: chunk %d embedding error for doc %d: %v", task.ID, i, task.DocID, err)
					continue
				}
				if vecs == nil {
					vecs = make([][]float64, len(texts))
				}
				vecs[i] = vec
			}
			if vecs == nil {
				s.failTask(task, "all chunk embeddings failed")
				logx.Infof("[VectorSyncer] Task %d: doc %d all chunk embeddings failed", task.ID, task.DocID)
				return
			}
		}

		// 2) 向量齐备后才清旧索引，再做本地写入
		if err := s.vs.DeleteDocVectors(task.DocID, maxChunksPerDoc); err != nil {
			logx.Warnf("[VectorSyncer] Task %d: delete old vectors warning: %v", task.ID, err)
		}

		successCount := 0
		var lastStoreErr string
		for i, vec := range vecs {
			if len(vec) == 0 {
				logx.Infof("[VectorSyncer] Task %d: chunk %d returned empty vector, skipping", task.ID, i)
				lastStoreErr = fmt.Sprintf("chunk %d empty vector", i)
				continue
			}
			chunkName := fmt.Sprintf("%s#chunk%d", task.DocName, i)
			chunkID := task.DocID*1000 + i
			if err := s.vs.StoreVector(chunkID, task.SpaceID, chunkName, vec); err != nil {
				lastStoreErr = fmt.Sprintf("chunk %d store: %v", i, err)
				logx.Infof("[VectorSyncer] Task %d: chunk %d store error for doc %d: %v", task.ID, i, task.DocID, err)
				continue
			}
			successCount++
		}

		// P0：只有在全部 chunk 都写入成功后才 MarkDone；
		// 部分失败标记为可重试失败，任务会重新入队补齐缺失的分块，避免索引永久缺块。
		if successCount == len(chunks) {
			_ = knowledgerepo.VectorSyncTaskRepo().MarkDone(task.ID)
			s.recordSyncState(task.DocID, task.Content)
			logx.Infof("[VectorSyncer] Task %d: doc %d indexed %d/%d chunks", task.ID, task.DocID, successCount, len(chunks))
		} else {
			errMsg := fmt.Sprintf("partial index: %d/%d chunks ok", successCount, len(chunks))
			if lastStoreErr != "" {
				errMsg += "; last: " + lastStoreErr
			}
			s.failTask(task, errMsg)
			logx.Infof("[VectorSyncer] Task %d: doc %d partially indexed %d/%d chunks, will retry", task.ID, task.DocID, successCount, len(chunks))
		}

	case "delete":
		// 删除文档及其所有 chunk 向量
		if err := s.vs.DeleteDocVectors(task.DocID, maxChunksPerDoc); err != nil {
			errMsg := err.Error()
			if len(errMsg) > 500 {
				errMsg = errMsg[:500]
			}
			_ = knowledgerepo.VectorSyncTaskRepo().MarkFailed(task.ID, errMsg)
			logx.Infof("[VectorSyncer] Delete error for doc %d: %v", task.DocID, err)
			return
		}
		_ = knowledgerepo.VectorSyncTaskRepo().MarkDone(task.ID)
		// P0：文档删了，指纹行一并清掉，避免残留导致同名新文档被误判"未变化"。
		if err := knowledgerepo.VectorSyncStateRepo().DeleteByDocID(task.DocID); err != nil {
			logx.Infof("[VectorSyncer] Warning: failed to clear sync state for doc %d: %v", task.DocID, err)
		}

	default:
		_ = knowledgerepo.VectorSyncTaskRepo().MarkDead(task.ID, "unknown op_type: "+task.OpType)
	}
}

func (s *VectorSyncer) Process(op VectorSyncOp) {
	s.processTask(knowledgemodel.VectorSyncTask{
		DocID:   op.DocID,
		SpaceID: op.SpaceID,
		DocName: op.DocName,
		Content: op.Content,
		OpType:  op.OpType,
	})
}

func (s *VectorSyncer) PurgeDoneTasks(olderThan time.Duration) error {
	return knowledgerepo.VectorSyncTaskRepo().PurgeDone(time.Now().Add(-olderThan))
}

// ChunkWithHash 带内容哈希的 Chunk
type ChunkWithHash struct {
	rag.Chunk
	Hash string
}

// DiffResult chunk diff 结果
type DiffResult struct {
	Added    []ChunkWithHash // 新增的 chunk
	Removed  []ChunkWithHash // 删除的 chunk
	Modified []ChunkWithHash // 内容变化的 chunk（index 相同但 hash 不同）
}

// chunkContentHash 计算 chunk 内容的 SHA256 哈希
func chunkContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8]) // 取前 8 字节即可
}

// hashChunks 为 chunk 列表生成哈希
func hashChunks(chunks []rag.Chunk) []ChunkWithHash {
	result := make([]ChunkWithHash, len(chunks))
	for i, c := range chunks {
		result[i] = ChunkWithHash{
			Chunk: c,
			Hash:  chunkContentHash(c.Content),
		}
	}
	return result
}

// DiffChunks 基于 content hash 对比新旧 chunk 列表，返回增量变更
func DiffChunks(oldChunks, newChunks []rag.Chunk) DiffResult {
	oldHashed := hashChunks(oldChunks)
	newHashed := hashChunks(newChunks)

	oldMap := make(map[string]ChunkWithHash, len(oldHashed))
	for _, c := range oldHashed {
		oldMap[c.Hash] = c
	}

	newMap := make(map[string]ChunkWithHash, len(newHashed))
	for _, c := range newHashed {
		newMap[c.Hash] = c
	}

	var result DiffResult

	// 找新增：在 new 中但不在 old 中的
	for _, nc := range newHashed {
		if _, exists := oldMap[nc.Hash]; !exists {
			result.Added = append(result.Added, nc)
		}
	}

	// 找删除：在 old 中但不在 new 中的
	for _, oc := range oldHashed {
		if _, exists := newMap[oc.Hash]; !exists {
			result.Removed = append(result.Removed, oc)
		}
	}

	// 找修改：位置相同但内容不同的
	minLen := len(oldHashed)
	if len(newHashed) < minLen {
		minLen = len(newHashed)
	}
	for i := 0; i < minLen; i++ {
		if oldHashed[i].Hash != newHashed[i].Hash {
			result.Modified = append(result.Modified, newHashed[i])
		}
	}

	return result
}
