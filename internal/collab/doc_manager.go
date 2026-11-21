package collab

import (
	"errors"
	"fmt"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	collabmodel "github.com/listen808/citebase/internal/model/collab"
	"github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// DocManager manages the in-memory state of all active collaborative documents.
// It loads document state on first connect, applies Yjs updates, and periodically
// persists state to MySQL.
type DocManager struct {
	mu        sync.RWMutex
	docs      map[uint]*DocState
	persistCh chan uint // doc IDs that need persisting
	stopCh    chan struct{}
	// persistedCh 由落盘循环在刷盘结束后关闭；Stop() 等它，而不是等 close(stopCh)。
	persistedCh      chan struct{}
	stopOnce         sync.Once
	persistCloseOnce sync.Once
}

// DocState holds the in-memory Yjs state for a single document.
type DocState struct {
	DocumentID uint
	State      []byte // Yjs document state (encoded Y.Doc state)
	Version    int
	Dirty      bool // has uncommitted changes
	LastActive time.Time
}

var (
	docManagerInstance *DocManager
	docManagerOnce     sync.Once
)

// GetDocManager returns the singleton DocManager instance.
func GetDocManager() *DocManager {
	docManagerOnce.Do(func() {
		docManagerInstance = &DocManager{
			docs:        make(map[uint]*DocState),
			persistCh:   make(chan uint, 100),
			stopCh:      make(chan struct{}),
			persistedCh: make(chan struct{}),
		}
		go docManagerInstance.persistLoop()
	})
	return docManagerInstance
}

// maxDocStateBytes 单个文档协作状态在内存中的字节上限（P0：防止无界增长耗尽内存）。
// 超过时丢弃最旧的字节并记录 warn。
const maxDocStateBytes = 8 << 20 // 8 MiB

// GetDocState returns the current Yjs state for a document.
// If the document is not yet loaded, it loads from the database.
// 返回副本：内部 State 会被 ApplyUpdate 并发追加，直接返回切片会产生 data race。
func (dm *DocManager) GetDocState(docID uint) []byte {
	dm.mu.RLock()
	ds, ok := dm.docs[docID]
	if !ok {
		dm.mu.RUnlock()
		// Load from database；查询失败时返回 nil（不得写入空状态覆盖真实数据）
		state, err := dm.loadDocState(docID)
		if err != nil {
			logx.Warnf("[Collab] Get doc %d state failed: %v", docID, err)
			return nil
		}
		return state
	}
	out := make([]byte, len(ds.State))
	copy(out, ds.State)
	dm.mu.RUnlock()

	// LastActive 是写操作，必须持写锁（原来在 RLock 下改写会与其它读者竞争）
	dm.mu.Lock()
	if cur, exists := dm.docs[docID]; exists {
		cur.LastActive = time.Now()
	}
	dm.mu.Unlock()
	return out
}

// ApplyUpdate applies an incremental Yjs update to the document state.
// In a full implementation, this would merge the update into the Yjs document.
// For now, we store updates and persist the combined state.
// P0：返回 error —— 状态加载失败时必须让调用方知道并且不要继续写入/覆盖真实数据。
func (dm *DocManager) ApplyUpdate(docID uint, update []byte) error {
	dm.mu.RLock()
	_, ok := dm.docs[docID]
	dm.mu.RUnlock()

	if !ok {
		// Load existing state first; 查询失败直接返回错误，绝不以空状态覆盖真实数据
		if _, err := dm.loadDocState(docID); err != nil {
			return err
		}
	}

	dm.mu.Lock()
	ds, exists := dm.docs[docID]
	if !exists {
		ds = &DocState{
			DocumentID: docID,
		}
		dm.docs[docID] = ds
	}
	// Append the update to the state.
	// In a full Yjs implementation, we would apply the update to a Y.Doc
	// and then extract the full state. For simplicity, we store updates
	// sequentially and combine them on persist.
	ds.State = append(ds.State, update...)
	ds.Dirty = true
	ds.LastActive = time.Now()
	dm.enforceStateLimitLocked(ds)
	dm.mu.Unlock()

	// Schedule persistence (non-blocking)
	select {
	case dm.persistCh <- docID:
	default:
	}
	return nil
}

// enforceStateLimitLocked 截断超出上限的状态（丢弃最旧字节），防止内存无界增长。
// 调用方必须持有 dm.mu 的写锁。
func (dm *DocManager) enforceStateLimitLocked(ds *DocState) {
	if len(ds.State) <= maxDocStateBytes {
		return
	}
	dropped := len(ds.State) - maxDocStateBytes
	ds.State = append([]byte(nil), ds.State[dropped:]...)
	logx.Warnf("[Collab] Doc %d collab state exceeded %d bytes, dropped %d oldest bytes (state truncated)",
		ds.DocumentID, maxDocStateBytes, dropped)
}

// InitDoc initializes a document in the doc manager if not already loaded.
func (dm *DocManager) InitDoc(docID uint) error {
	dm.mu.RLock()
	_, ok := dm.docs[docID]
	dm.mu.RUnlock()

	if ok {
		return nil
	}
	_, err := dm.loadDocState(docID)
	return err
}

// OnDocEmpty is called when the last client leaves a document.
// It persists the state and unloads it from memory after a delay.
func (dm *DocManager) OnDocEmpty(docID uint) {
	dm.mu.RLock()
	ds, ok := dm.docs[docID]
	dirty := ok && ds.Dirty
	dm.mu.RUnlock()

	if dirty {
		dm.persistDocState(docID)
	}

	// Schedule unload after idle timeout
	go func() {
		time.Sleep(5 * time.Minute)
		dm.mu.Lock()
		if ds, ok := dm.docs[docID]; ok {
			if time.Since(ds.LastActive) > 4*time.Minute {
				if ds.Dirty {
					dm.mu.Unlock()
					dm.persistDocState(docID)
					dm.mu.Lock()
				}
				delete(dm.docs, docID)
				logx.Infof("[Collab] Unloaded doc %d from memory", docID)
			}
		}
		dm.mu.Unlock()
	}()
}

// loadDocState loads a document's Yjs state from the database.
// P0：区分“查询失败”与“确实没有协作状态”：
//   - 查询失败（非 ErrRecordNotFound）→ 返回 error，且不写入空 DocState，
//     调用方必须停止写入/持久化，否则会用空内容覆盖真实数据；
//   - 记录不存在 / 数据库不可用 → 视为“暂无状态”，返回 (nil, nil)。
func (dm *DocManager) loadDocState(docID uint) ([]byte, error) {
	if repository.DB == nil {
		return nil, nil
	}

	var state collabmodel.DocumentCollabState
	err := repository.DB.Where("document_id = ?", docID).First(&state).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logx.Errorf("[Collab] Failed to load doc %d state: %v", docID, err)
			return nil, fmt.Errorf("load collab state of doc %d: %w", docID, err)
		}
		// Document has no collab state yet; start with empty state
		dm.mu.Lock()
		if _, exists := dm.docs[docID]; !exists {
			dm.docs[docID] = &DocState{
				DocumentID: docID,
				LastActive: time.Now(),
			}
		}
		dm.mu.Unlock()
		return nil, nil
	}

	loaded := make([]byte, len(state.State))
	copy(loaded, state.State)

	dm.mu.Lock()
	if _, exists := dm.docs[docID]; !exists {
		dm.docs[docID] = &DocState{
			DocumentID: docID,
			State:      loaded,
			Version:    state.Version,
			LastActive: time.Now(),
		}
	}
	ds := dm.docs[docID]
	out := make([]byte, len(ds.State))
	copy(out, ds.State)
	dm.mu.Unlock()

	logx.Infof("[Collab] Loaded doc %d state (version %d, %d bytes)", docID, state.Version, len(state.State))
	return out, nil
}

// persistDocState saves the current document state to the database.
func (dm *DocManager) persistDocState(docID uint) {
	dm.mu.RLock()
	ds, ok := dm.docs[docID]
	if !ok || !ds.Dirty {
		dm.mu.RUnlock()
		return
	}
	stateCopy := make([]byte, len(ds.State))
	copy(stateCopy, ds.State)
	version := ds.Version
	dm.mu.RUnlock()

	if repository.DB == nil {
		return
	}

	var existing collabmodel.DocumentCollabState
	err := repository.DB.Where("document_id = ?", docID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new record
		record := collabmodel.DocumentCollabState{
			DocumentID: docID,
			State:      stateCopy,
			Version:    version + 1,
		}
		if err := repository.DB.Create(&record).Error; err != nil {
			logx.Infof("[Collab] Failed to create doc %d state: %v", docID, err)
			return
		}
	} else if err != nil {
		logx.Infof("[Collab] Failed to query doc %d state: %v", docID, err)
		return
	} else {
		// Update existing record
		if err := repository.DB.Model(&existing).Updates(map[string]interface{}{
			"state":   stateCopy,
			"version": version + 1,
		}).Error; err != nil {
			logx.Infof("[Collab] Failed to update doc %d state: %v", docID, err)
			return
		}
	}

	dm.mu.Lock()
	if ds, ok := dm.docs[docID]; ok {
		ds.Version = version + 1
		ds.Dirty = false
	}
	dm.mu.Unlock()

	logx.Infof("[Collab] Persisted doc %d state (version %d, %d bytes)", docID, version+1, len(stateCopy))
}

// persistLoop periodically flushes dirty documents to the database.
func (dm *DocManager) persistLoop() {
	// Debounce: wait 5 seconds after the last update before persisting
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	pending := make(map[uint]bool)

	for {
		select {
		case docID := <-dm.persistCh:
			pending[docID] = true

		case <-ticker.C:
			for docID := range pending {
				dm.persistDocState(docID)
				delete(pending, docID)
			}

		case <-dm.stopCh:
			// Flush all pending on shutdown
			for docID := range pending {
				dm.persistDocState(docID)
			}
			// 落盘循环真的走完了才通知 —— 只有 close(stopCh) 的话，
			// 调用方会以为自己已经保存完，实际持久化还在另一个协程里跑。
			dm.persistDone()
			return
		}
	}
}

// Stop 关停 DocManager 并把脏状态刷盘。
//
// 三条原先缺的东西：
//   - sync.Once：close 已关闭的 channel 会 panic，而 Shutdown 路径与测试路径
//     都可能各调一次，一次重复就崩进程；
//   - 等待刷盘完成：原实现 close 完立刻返回，优雅关停于是"启动了保存"但
//     不保证保存结束 —— 协作编辑的最后几秒内容可能整个丢掉；
//   - 超时兜底：单个 persistDocState 卡住（数据库没响应）不能把关停拖成永不返回。
func (dm *DocManager) Stop() {
	dm.stopOnce.Do(func() {
		close(dm.stopCh)
	})

	select {
	case <-dm.persistedCh:
	case <-time.After(10 * time.Second):
		logx.Warnf("[Collab] 等待协作状态刷盘超时（10s），部分未保存的协作内容可能丢失")
	}
}

// persistDone 由落盘循环在退出前调用。
func (dm *DocManager) persistDone() {
	dm.persistCloseOnce.Do(func() {
		close(dm.persistedCh)
	})
}

// GetDocInfo returns information about an active document for the API.
func (dm *DocManager) GetDocInfo(docID uint) (stateLen int, version int, active bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	ds, ok := dm.docs[docID]
	if !ok {
		return 0, 0, false
	}
	return len(ds.State), ds.Version, true
}
