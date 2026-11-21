package text2sql

import (
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"time"

	modelai "github.com/listen808/citebase/internal/model/ai"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils"
)

// VectorSyncFunc 向量同步回调函数类型
type VectorSyncFunc func(docID, spaceID int, docName, content, opType string)

// SpaceScanner 扫描指定空间中的 SQL 建表语句
type SpaceScanner struct {
	docRepo    *workspacerepo.DocumentRepository
	spaceRepo  *workspacerepo.SpaceRepository
	docStorage *utils.DocumentStorage
	parser     *DDLParser
	repo       *airepo.TextToSQLRepo
	vectorSync VectorSyncFunc
	chunker    *SQLSchemaChunker
}

// ScanResult 扫描结果
type ScanResult struct {
	TotalDocs     int      `json:"total_docs"`
	ScannedDocs   int      `json:"scanned_docs"`
	NewTables     int      `json:"new_tables"`
	UpdatedTables int      `json:"updated_tables"`
	DeletedTables int      `json:"deleted_tables"`
	SkippedDocs   int      `json:"skipped_docs"`
	Tables        []string `json:"tables"`
	Errors        []string `json:"errors,omitempty"`
}

// NewSpaceScanner 创建空间扫描器
func NewSpaceScanner(
	docRepo *workspacerepo.DocumentRepository,
	spaceRepo *workspacerepo.SpaceRepository,
	docStorage *utils.DocumentStorage,
	repo *airepo.TextToSQLRepo,
	vectorSync VectorSyncFunc,
) *SpaceScanner {
	return &SpaceScanner{
		docRepo:    docRepo,
		spaceRepo:  spaceRepo,
		docStorage: docStorage,
		parser:     NewDDLParser(),
		repo:       repo,
		vectorSync: vectorSync,
		chunker:    NewSQLSchemaChunker(),
	}
}

// ScanSpace 全量扫描指定空间（清除旧数据并重新扫描）
func (s *SpaceScanner) ScanSpace(spaceID int) (*ScanResult, error) {
	space, err := s.spaceRepo.FindByID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("查找空间 %d 失败: %w", spaceID, err)
	}
	if space == nil {
		return nil, fmt.Errorf("空间 %d 不存在", spaceID)
	}

	lastCaches, _ := s.repo.GetTableCachesBySpace(uint64(spaceID))
	oldTableMap := make(map[string]bool)
	for _, c := range lastCaches {
		oldTableMap[c.TableNam] = true
	}

	if err := s.repo.DeleteTableCachesBySpace(uint64(spaceID)); err != nil {
		return nil, fmt.Errorf("清除旧表缓存失败: %w", err)
	}
	if err := s.repo.DeleteRelationsBySpace(uint64(spaceID)); err != nil {
		return nil, fmt.Errorf("清除旧表关系失败: %w", err)
	}

	tables, rels, err := s.scanAndAssemble(space, spaceID)
	if err != nil {
		return nil, err
	}

	newTableMap := make(map[string]bool)
	for _, t := range tables {
		newTableMap[t.TableNam] = true
	}

	result := &ScanResult{
		TotalDocs: len(tables) + len(rels),
		Tables:    make([]string, 0, len(tables)),
	}

	for _, table := range tables {
		if oldTableMap[table.TableNam] {
			result.UpdatedTables++
		} else {
			result.NewTables++
		}
		result.Tables = append(result.Tables, table.TableNam)
	}

	for t := range oldTableMap {
		if !newTableMap[t] {
			result.DeletedTables++
		}
	}

	s.syncVectors(tables, spaceID)
	s.saveRelations(rels, uint64(spaceID))

	logx.Infof("[SpaceScanner] Space %d full scan completed: %d tables (%d new, %d updated, %d deleted)",
		spaceID, len(tables), result.NewTables, result.UpdatedTables, result.DeletedTables)

	return result, nil
}

// ScanSpaceIncremental 增量扫描指定空间（只扫描变更的文档）
func (s *SpaceScanner) ScanSpaceIncremental(spaceID int, lastScanAt time.Time) (*ScanResult, error) {
	space, err := s.spaceRepo.FindByID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("查找空间 %d 失败: %w", spaceID, err)
	}
	if space == nil {
		return nil, fmt.Errorf("空间 %d 不存在", spaceID)
	}

	docs, err := s.docRepo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("获取空间文档失败: %w", err)
	}

	oldCaches, _ := s.repo.GetTableCachesBySpace(uint64(spaceID))
	oldTableMap := make(map[string]*modelai.TableCache)
	for i := range oldCaches {
		oldTableMap[oldCaches[i].TableNam] = &oldCaches[i]
	}

	result := &ScanResult{
		TotalDocs: len(docs),
	}

	var newCaches []*modelai.TableCache
	var keptTables []*modelai.TableCache

	for _, doc := range docs {
		if doc.Type != workspacemodel.DocumentTypePage {
			result.SkippedDocs++
			continue
		}

		if time.Unix(int64(doc.UpdateTime), 0).Before(lastScanAt) && !time.Unix(int64(doc.UpdateTime), 0).Equal(lastScanAt) {
			result.SkippedDocs++
			continue
		}

		tables, err := s.scanDocument(doc, space.Name)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("文档 %s 扫描失败: %v", doc.Name, err))
			continue
		}
		if tables == nil {
			continue
		}

		result.ScannedDocs++

		docOldCaches := s.getDocTables(oldCaches, uint64(doc.ID))

	outerKept:
		for _, table := range tables {
			table.SpaceID = uint64Ptr(uint64(spaceID))
			table.SchemaSpaceID = uint64Ptr(uint64(spaceID))
			table.DocID = uint64Ptr(uint64(doc.ID))

			existing, ok := oldTableMap[table.TableNam]
			if !ok {
				result.NewTables++
			} else if existing.DocID != nil && *existing.DocID == uint64(doc.ID) {
				result.UpdatedTables++
				// 同文档的表定义已变化：新内容必须落库（旧实现只保留旧行，
				// 增量扫描结果永远不进 DB，导致查询结果与文档不一致）
				if err := s.repo.UpsertTableCache(table); err != nil {
					logx.Infof("[SpaceScanner] 增量扫描更新表 %s 失败: %v", table.TableNam, err)
					keptTables = append(keptTables, existing)
				} else {
					keptTables = append(keptTables, table)
				}
				continue outerKept
			} else {
				result.UpdatedTables++
			}

			newCaches = append(newCaches, table)
			result.Tables = append(result.Tables, table.TableNam)
		}

		for _, oldCache := range docOldCaches {
			found := false
			for _, t := range tables {
				if t.TableNam == oldCache.TableNam {
					found = true
					break
				}
			}
			if !found {
				// 文档里已删除的表同步删除 DB 行，否则残留幽灵表
				if err := s.repo.DeleteTableCacheBySpaceAndName(oldCache.TableNam, uint64(spaceID)); err != nil {
					logx.Infof("[SpaceScanner] 增量扫描删除表 %s 失败: %v", oldCache.TableNam, err)
				}
				if s.vectorSync != nil {
					chunkID := doc.ID * 1000
					for idx, t := range tables {
						if t.TableNam == oldCache.TableNam {
							chunkID = doc.ID*1000 + idx
							break
						}
					}
					s.vectorSync(chunkID, spaceID, fmt.Sprintf("%s#%s", doc.Name, oldCache.TableNam), "", "delete")
				}
				result.DeletedTables++
				delete(oldTableMap, oldCache.TableNam)
			}
		}
	}

	for _, table := range newCaches {
		// 新表必须先 Upsert 落库再同步向量：
		// 1) 不落库则 GetTableCachesBySpace/KnowledgeBase 看不到这些表（扫描成功但表数在减少）；
		// 2) 向量 docID 用 table.ID，未入库时恒为 0，多张新表会共享 docID 互相覆盖。
		if err := s.repo.UpsertTableCache(table); err != nil {
			logx.Infof("[SpaceScanner] 增量扫描保存新表 %s 失败: %v", table.TableNam, err)
			continue
		}
		if s.vectorSync != nil {
			richText := s.chunker.BuildRichText(table, nil)
			chunkName := fmt.Sprintf("%s#%s", "ddl", table.TableNam)
			s.vectorSync(int(table.ID), int(*table.SpaceID), chunkName, richText, "store")
		}
	}

	for _, table := range keptTables {
		if s.vectorSync != nil {
			richText := s.chunker.BuildRichText(table, nil)
			chunkName := fmt.Sprintf("%s#%s", "ddl", table.TableNam)
			s.vectorSync(int(table.ID), int(*table.SpaceID), chunkName, richText, "store")
		}
	}

	allCaches, err := s.repo.GetTableCachesBySpace(uint64(spaceID))
	if err == nil {
		relations := s.parser.InferRelations(allCaches)
		s.saveRelations(relations, uint64(spaceID))
	}

	logx.Infof("[SpaceScanner] Space %d incremental scan completed: %d scanned, %d skipped, %d new, %d updated, %d deleted",
		spaceID, result.ScannedDocs, result.SkippedDocs, result.NewTables, result.UpdatedTables, result.DeletedTables)

	return result, nil
}

// scanAndAssemble 扫描空间中所有文档并组装结果
func (s *SpaceScanner) scanAndAssemble(space *workspacemodel.Space, spaceID int) ([]*modelai.TableCache, []modelai.TableRelation, error) {
	docs, err := s.docRepo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取空间文档失败: %w", err)
	}

	var allTables []*modelai.TableCache

	for _, doc := range docs {
		if doc.Type != workspacemodel.DocumentTypePage {
			continue
		}

		tables, err := s.scanDocument(doc, space.Name)
		if err != nil {
			logx.Infof("[SpaceScanner] 文档 %s 扫描失败: %v", doc.Name, err)
			continue
		}
		if tables == nil {
			continue
		}

		for _, table := range tables {
			table.SpaceID = uint64Ptr(uint64(spaceID))
			table.SchemaSpaceID = uint64Ptr(uint64(spaceID))
			table.DocID = uint64Ptr(uint64(doc.ID))

			if err := s.repo.UpsertTableCache(table); err != nil {
				logx.Infof("[SpaceScanner] 保存表 %s 失败: %v", table.TableNam, err)
				continue
			}
			allTables = append(allTables, table)
		}
	}

	caches, err := s.repo.GetTableCachesBySpace(uint64(spaceID))
	if err != nil {
		return allTables, nil, err
	}

	relations := s.parser.InferRelations(caches)

	cachePtrs := make([]*modelai.TableCache, len(caches))
	for i := range caches {
		cachePtrs[i] = &caches[i]
	}

	return cachePtrs, relations, nil
}

// syncVectors 同步所有表的向量
func (s *SpaceScanner) syncVectors(tables []*modelai.TableCache, spaceID int) {
	if s.vectorSync == nil {
		return
	}
	for idx, table := range tables {
		relations, _ := s.repo.GetRelationsByTable(table.TableNam)
		richText := s.chunker.BuildRichTextWithRelations(table, relations)
		chunkName := fmt.Sprintf("ddl#%s", table.TableNam)
		s.vectorSync(idx, spaceID, chunkName, richText, "store")
	}
}

// saveRelations 保存表关系
func (s *SpaceScanner) saveRelations(relations []modelai.TableRelation, spaceID uint64) {
	for _, rel := range relations {
		rel.SpaceID = uint64Ptr(spaceID)
		if err := s.repo.UpsertRelation(&rel); err != nil {
			logx.Infof("[SpaceScanner] 保存表关系 %s.%s→%s.%s 失败: %v",
				rel.SourceTable, rel.SourceField, rel.TargetTable, rel.TargetField, err)
		}
	}
}

// getDocTables 获取指定文档的所有表缓存
func (s *SpaceScanner) getDocTables(caches []modelai.TableCache, docID uint64) []modelai.TableCache {
	var docCaches []modelai.TableCache
	for _, c := range caches {
		if c.DocID != nil && *c.DocID == docID {
			docCaches = append(docCaches, c)
		}
	}
	return docCaches
}

// scanDocument 扫描单个文档
func (s *SpaceScanner) scanDocument(doc *workspacemodel.Document, spaceName string) ([]*modelai.TableCache, error) {
	pageFile := s.docStorage.GetPageFile(spaceName, doc.Name, doc.Type)

	content, err := doccontent.Get(doc.ID, s.docStorage, pageFile)
	if err != nil {
		return nil, fmt.Errorf("读取文档内容失败: %w", err)
	}
	if content == "" {
		return nil, nil
	}

	tables, err := s.parser.ParseFromMarkdown(content)
	if err != nil {
		return nil, fmt.Errorf("解析 DDL 失败: %w", err)
	}

	return tables, nil
}

// uint64Ptr 返回 uint64 指针
func uint64Ptr(v uint64) *uint64 {
	return &v
}

// IsTableCached 检查指定空间是否已有缓存
func (s *SpaceScanner) IsTableCached(spaceID int) (bool, error) {
	count, err := s.repo.CountTableCachesBySpace(uint64(spaceID))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
