// Package doccontent 是正文存储的单一开关点。
//
// storage.content_source=db 时正文读写走 aw_document_content（按 document_id）；
// 其余情况保持原行为，由调用方传入各自的 DocumentStorage 与 pageFile 读写磁盘。
// 这样切换/回滚只改一行配置，调用方的路径算法在 file 模式下完全不变。
package doccontent

import (
	"github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
	"gorm.io/gorm"
)

// repo 按需构造：不能做包级变量 —— 包初始化早于 main 里的 InitDB，那时 repos.DB 还是 nil。
func repo() *workspacerepo.DocumentContentRepository {
	return workspacerepo.NewDocumentContentRepository()
}

// UseDB 当前是否以数据库为正文来源
func UseDB() bool {
	return config.AppConfig.Storage.ContentSource == "db"
}

// Get 读取正文。db 模式按 documentID 读库；file 模式读磁盘（pageFile 由调用方算好）。
func Get(documentID int, store *utils.DocumentStorage, pageFile string) (string, error) {
	if UseDB() {
		return repo().Get(documentID)
	}
	return store.GetContent(pageFile)
}

// Set 写入正文。db 模式写库；file 模式写磁盘。
func Set(documentID int, store *utils.DocumentStorage, pageFile string, content string) error {
	if UseDB() {
		return repo().Upsert(documentID, content)
	}
	return store.SetContent(pageFile, content)
}

// SetTx 在事务内写入正文（仅 db 模式有实际动作，file 模式由调用方在事务外写盘）。
func SetTx(tx *gorm.DB, documentID int, content string) error {
	if !UseDB() {
		return nil
	}
	return repo().UpsertTx(tx, documentID, content)
}

// Delete 删除正文。file 模式仍按原逻辑删文件（含目录整棵删除）。
func Delete(documentID int, store *utils.DocumentStorage, pageFile string, docType int) error {
	if UseDB() {
		return repo().Delete(documentID)
	}
	return store.Delete(pageFile, docType)
}

// Rename 重命名/移动正文。db 模式正文跟着 document_id 走，无需搬动。
func Rename(store *utils.DocumentStorage, oldPageFile, newPageFile string, docType int) error {
	if UseDB() {
		return nil
	}
	return store.RenameByPath(oldPageFile, newPageFile, docType)
}

// Row 组装迁移/回填用的正文行
func Row(documentID int, content string) *workspace.DocumentContent {
	return workspacerepo.NewDocumentContentRow(documentID, content)
}
