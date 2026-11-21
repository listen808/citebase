package schema

import (
	"context"
	"errors"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/embedding"
	aischema "github.com/listen808/citebase/internal/ai/schema"
	modelai "github.com/listen808/citebase/internal/model/ai"
	schemamodel "github.com/listen808/citebase/internal/model/schema"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	schemarepo "github.com/listen808/citebase/internal/repository/schema"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
)

// textToSQLBudget 是服务层起 LLM 生成的时间预算上限。
// 数值不是新发明的一档：openai 适配层的 httpClient.Timeout 是 300s，
// 前端调 /generate-sql 与 /schema-spaces/:id/ask 时 axios 的 timeout 也是 300000ms。
// 原先这两处传的是裸 context.Background()，等于这一层永远不超时也不取消 ——
// 客户端断开之后 LLM 还在往下跑，而多候选生成是并跑好几个，成本按倍数算。
// 现在 AskQuestion / AskQuestionWithHistory 都从请求 ctx 派生超时
// （context.WithTimeout(ctx, textToSQLBudget)），调用者身份随 ctx 传到 provider 层，
// CheckQuota 预检随之生效。
const textToSQLBudget = 300 * time.Second

// SchemaService Schema业务逻辑层
type SchemaService struct {
	schemaRepo  *schemarepo.SchemaRepository
	docRepo     *workspacerepo.DocumentRepository
	spaceRepo   *workspacerepo.SpaceRepository
	docStorage  *utils.DocumentStorage
	parser      *text2sql.DDLParser
	chunker     *aischema.Chunker
	vectorStore *aischema.VectorStore

	// Text2SQL 合并字段
	textToSQLRepo      *airepo.TextToSQLRepo
	textToSQLGenerator *text2sql.Generator
	textToSQLParser    *text2sql.DDLParser
	promptTemplateRepo *airepo.PromptTemplateRepo
	vectorSyncCallback func(docID, spaceID int, docName, content, opType string)

	// 列统计采集器
	statsCollector *text2sql.SchemaStatsCollector

	// 活跃扫描任务管理
	activeScans sync.Map // map[int]context.CancelFunc, key=taskID
}

func NewSchemaService(schemaVS *embedding.SchemaVectorStore) *SchemaService {
	schemaRepo := schemarepo.NewSchemaRepository()
	vs := aischema.NewVectorStore(schemaVS)
	docDir := config.AppConfig.Storage.DocumentDir
	if docDir == "" {
		docDir = "./data/markdowns"
	}
	return &SchemaService{
		schemaRepo:  schemaRepo,
		docRepo:     workspacerepo.NewDocumentRepository(),
		spaceRepo:   workspacerepo.NewSpaceRepository(),
		docStorage:  utils.NewDocumentStorageWithDir(docDir),
		parser:      text2sql.NewDDLParser(),
		chunker:     aischema.NewChunker(),
		vectorStore: vs,
	}
}

// SetTextToSQLDependencies 注入 Text2SQL 依赖（在 main.go 中调用，避免构造函数参数过多）
func (s *SchemaService) SetTextToSQLDependencies(repo *airepo.TextToSQLRepo) error {
	s.textToSQLRepo = repo
	s.textToSQLParser = text2sql.NewDDLParser()

	gen, err := text2sql.NewGenerator(repo)
	if err != nil {
		return fmt.Errorf("init text2sql generator: %w", err)
	}
	s.textToSQLGenerator = gen
	return nil
}

// SetTextToSQLPromptTemplateRepo 注入提示词模板仓库
func (s *SchemaService) SetTextToSQLPromptTemplateRepo(repo *airepo.PromptTemplateRepo) {
	s.promptTemplateRepo = repo
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetPromptTemplateRepo(repo)
	}
}

// SetTextToSQLFeedbackRepo 注入 Text2SQL 执行反馈仓库
func (s *SchemaService) SetTextToSQLFeedbackRepo(repo *airepo.Text2SQLFeedbackRepo) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetFeedbackRepo(repo)
	}
}

// SetTextToSQLTraceRepo 注入 SQL Trace 仓库，启用生成链路追踪
func (s *SchemaService) SetTextToSQLTraceRepo(repo *airepo.SQLTraceRepo) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetTraceCollector(repo)
	}
}

// SetTextToSQLSemanticRepos 注入语义层仓储（语义指标、维度、列统计）
func (s *SchemaService) SetTextToSQLSemanticRepos(metricRepo *airepo.SemanticMetricRepo, dimensionRepo *airepo.SemanticDimensionRepo, statsRepo *airepo.ColumnStatsRepo) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetSemanticRepos(metricRepo, dimensionRepo, statsRepo)
	}
}

// SetTextToSQLPromptVersionRepo 注入 Prompt 版本仓库并启用版本选择器
func (s *SchemaService) SetTextToSQLPromptVersionRepo(repo *airepo.PromptVersionRepo) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetPromptVersionSelector(text2sql.NewPromptVersionSelector(repo))
	}
}

// SetSchemaStatsCollector 注入列统计采集器
func (s *SchemaService) SetSchemaStatsCollector(statsRepo *airepo.ColumnStatsRepo, dsRepo *airepo.DataSourceRepo) {
	s.statsCollector = text2sql.NewSchemaStatsCollector(repos.DB, statsRepo, dsRepo)
}

// CollectColumnStats 触发列统计采集（供定时任务或导入后调用）
func (s *SchemaService) CollectColumnStats(spaceID uint64) error {
	if s.statsCollector == nil {
		return fmt.Errorf("stats collector not initialized")
	}
	if s.textToSQLRepo == nil {
		return fmt.Errorf("text2sql repo not initialized")
	}

	tables, err := s.textToSQLRepo.GetTableCachesBySpace(spaceID)
	if err != nil || len(tables) == 0 {
		return fmt.Errorf("no tables found for space %d", spaceID)
	}

	for _, table := range tables {
		var columns []text2sql.ColumnInfo
		for _, col := range table.ColumnsJSON {
			columns = append(columns, text2sql.ColumnInfo{
				Name: col.Name,
				Type: col.Type,
			})
		}
		if err := s.statsCollector.CollectForTable(spaceID, table.TableNam, columns); err != nil {
			logx.Infof("[SchemaService] Failed to collect stats for table %s: %v", table.TableNam, err)
		}
	}
	return nil
}

// ListPromptTemplates 按类型和查询类型列出提示词模板
func (s *SchemaService) ListPromptTemplates(templateType, queryType string) ([]modelai.PromptTemplate, error) {
	if s.promptTemplateRepo == nil {
		return nil, fmt.Errorf("prompt template repo not available")
	}
	return s.promptTemplateRepo.ListByTypeAndQueryType(templateType, queryType)
}

// SetTextToSQLRAGAdapter 设置 Text2SQL 的 RAG 适配器
func (s *SchemaService) SetTextToSQLRAGAdapter(adapter *text2sql.SQLRAGAdapter) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetRAGAdapter(adapter)
	}
}

// SetTextToSQLExecutor 设置 Text2SQL 的 SQL 执行器
func (s *SchemaService) SetTextToSQLExecutor(executor *text2sql.SQLExecutor) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetExecutor(executor)
	}
}

// GetTextToSQLGenerator 获取 Text2SQL 生成器
func (s *SchemaService) GetTextToSQLGenerator() *text2sql.Generator {
	return s.textToSQLGenerator
}

// SetSchemaLinker 注册 Schema Linker（两阶段 Schema Linking）
func (s *SchemaService) SetSchemaLinker(linker *text2sql.SchemaLinker) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetSchemaLinker(linker)
	}
}

// SetVectorSyncCallback 设置向量同步回调
func (s *SchemaService) SetVectorSyncCallback(cb func(docID, spaceID int, docName, content, opType string)) {
	s.vectorSyncCallback = cb
}

// TextToSQLRepo 获取 Text2SQL 仓库（供外部使用，如 RAG adapter 初始化）
func (s *SchemaService) TextToSQLRepo() *airepo.TextToSQLRepo {
	return s.textToSQLRepo
}

// CreateSchemaSpace 创建Schema空间
func (s *SchemaService) CreateSchemaSpace(spaceID int, name string, dbType string) (*schemamodel.SchemaSpace, error) {
	existing, _ := s.schemaRepo.FindSpaceByWikiSpaceID(spaceID)
	if existing != nil {
		return nil, fmt.Errorf("该 Wiki 空间已关联 Schema 空间「%s」，不能重复创建", existing.Name)
	}
	space := &schemamodel.SchemaSpace{
		SpaceID: spaceID,
		Name:    name,
		DBType:  dbType,
		Status:  1,
	}
	if err := s.schemaRepo.CreateSpace(space); err != nil {
		return nil, err
	}
	return space, nil
}

// GetSchemaSpace 获取Schema空间
func (s *SchemaService) GetSchemaSpace(id int) (*schemamodel.SchemaSpace, error) {
	return s.schemaRepo.FindSpaceByID(id)
}

// GetSchemaSpaceByWikiSpace 通过Wiki空间ID获取Schema空间
func (s *SchemaService) GetSchemaSpaceByWikiSpace(spaceID int) (*schemamodel.SchemaSpace, error) {
	return s.schemaRepo.FindSpaceByWikiSpaceID(spaceID)
}

// ListSchemaSpaces 列出Schema空间
func (s *SchemaService) ListSchemaSpaces(page, pageSize int) ([]*schemamodel.SchemaSpace, int64, error) {
	return s.schemaRepo.ListSpaces(page, pageSize)
}

// DeleteSchemaSpace 删除Schema空间（下游数据与空间本体在同一条事务里）
func (s *SchemaService) DeleteSchemaSpace(id int) error {
	// 向量集合不是事务资源，只能放在 DB 提交之后：
	// 先清向量的话，一旦级联删失败就出现"库里还在、向量已经没了"的半删状态，
	// 而那种状态下 Text2SQL 检索到的是空集合，看起来像"这个空间没数据"。
	if err := s.schemaRepo.DeleteSpaceCascade(id); err != nil {
		return err
	}
	if err := s.vectorStore.ClearCollection(id); err != nil {
		logx.Warnf("[SchemaService] 空间 %d 已删除，但向量集合清理失败（残留集合不影响正确性，可重跑）: %v", id, err)
	}
	return nil
}

// StartScan 启动扫描
func (s *SchemaService) StartScan(schemaSpaceID int) (*schemamodel.SchemaScanTask, error) {
	// 获取Schema空间
	space, err := s.schemaRepo.FindSpaceByID(schemaSpaceID)
	if err != nil {
		return nil, err
	}

	// 获取Wiki空间下的所有Markdown文档
	docs, err := s.docRepo.FindBySpaceID(space.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents: %w", err)
	}

	// 创建扫描任务，初始状态 pending
	now := time.Now()
	task := &schemamodel.SchemaScanTask{
		SchemaSpaceID: schemaSpaceID,
		Status:        "pending",
		TotalDocs:     len(docs),
		ProcessedDocs: 0,
		FoundTables:   0,
	}
	if err := s.schemaRepo.CreateScanTask(task); err != nil {
		return nil, err
	}

	// 启动扫描前改为 running
	task.Status = "running"
	task.StartedAt = &now
	if err := s.schemaRepo.UpdateScanTask(task); err != nil {
		return nil, err
	}

	logx.Infof("[SchemaService] Created scan task %d, starting goroutine with %d docs", task.ID, len(docs))

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	s.activeScans.Store(task.ID, cancel)

	// 执行扫描（异步）
	go func(taskID int, spaceID int, documents []*workspacemodel.Document) {
		defer func() {
			if r := recover(); r != nil {
				logx.Infof("[SchemaService] Scan goroutine panic: %v", r)
				s.failScanTask(taskID, fmt.Sprintf("panic: %v", r))
			}
			s.activeScans.Delete(taskID)
		}()
		logx.Infof("[SchemaService] Starting scan goroutine for task %d", taskID)
		s.executeScan(ctx, spaceID, taskID, documents)
		logx.Infof("[SchemaService] Scan goroutine finished for task %d", taskID)
	}(task.ID, schemaSpaceID, docs)

	return task, nil
}

// executeScan 执行扫描（异步）
func (s *SchemaService) executeScan(ctx context.Context, schemaSpaceID, taskID int, docs []*workspacemodel.Document) {
	defer func() {
		if r := recover(); r != nil {
			logx.Infof("[SchemaService] Scan panic recovered: %v", r)
			s.failScanTask(taskID, fmt.Sprintf("internal error: %v", r))
		}
	}()

	task, err := s.schemaRepo.FindScanTaskByID(taskID)
	if err != nil || task == nil {
		logx.Infof("[SchemaService] Scan task %d not found: %v", taskID, err)
		return
	}

	// 检查是否已被取消
	if task.Status == "cancelled" {
		logx.Infof("[SchemaService] Scan task %d already cancelled, aborting", taskID)
		return
	}

	logx.Infof("[SchemaService] Starting scan task %d for schema space %d, docs: %d", taskID, schemaSpaceID, len(docs))

	// 清空旧数据：四张下游表必须在一条事务里删 —— 原先逐条删且不看错误，
	// 中途失败会留下"表删了、列还在"的半空空间，而向量集合随后又被写满，
	// 下一次生成的 SQL 就建在一套自相矛盾的 schema 上。
	if err := s.schemaRepo.ClearSpaceCascade(schemaSpaceID); err != nil {
		logx.Errorf("[SchemaService] 重扫前清空空间 %d 失败，中止本次扫描（不拿半空 schema 继续跑）: %v", schemaSpaceID, err)
		s.failScanTask(taskID, fmt.Sprintf("清空旧 schema 数据失败: %v", err))
		return
	}
	if err := s.vectorStore.ClearCollection(schemaSpaceID); err != nil {
		logx.Warnf("[SchemaService] 空间 %d 向量集合清理失败（本次重扫会覆盖同名条目）: %v", schemaSpaceID, err)
	}

	foundTables := 0
	var allChunks []*aischema.SchemaChunk
	var errorMsgs []string

	for i, doc := range docs {
		// 检查 context 取消
		select {
		case <-ctx.Done():
			logx.Infof("[SchemaService] Scan task %d cancelled at doc %d/%d", taskID, i, len(docs))
			s.cancelScanTask(taskID)
			return
		default:
		}

		// 检查暂停状态
		currentTask, _ := s.schemaRepo.FindScanTaskByID(taskID)
		if currentTask != nil && currentTask.Status == "paused" {
			logx.Infof("[SchemaService] Scan task %d paused at doc %d/%d", taskID, i, len(docs))
			// 阻塞等待恢复或取消
			for {
				select {
				case <-ctx.Done():
					s.cancelScanTask(taskID)
					return
				case <-time.After(2 * time.Second):
					currentTask, _ = s.schemaRepo.FindScanTaskByID(taskID)
					if currentTask == nil {
						return
					}
					if currentTask.Status == "running" {
						logx.Infof("[SchemaService] Scan task %d resumed", taskID)
						break
					}
					if currentTask.Status == "cancelled" {
						s.cancelScanTask(taskID)
						return
					}
				}
				if currentTask.Status == "running" {
					break
				}
			}
		}

		// 更新进度
		task.ProcessedDocs = i + 1
		if err := s.schemaRepo.UpdateScanTask(task); err != nil {
			logx.Infof("[SchemaService] Failed to update task progress: %v", err)
		}

		// 读取文档内容
		pageFile := s.getPageFilePath(doc)
		absFile := s.docStorage.GetAbsPageFile(pageFile)
		logx.Infof("[SchemaService] Scanning doc %d/%d: %s (file: %s, abs: %s)", i+1, len(docs), doc.Name, pageFile, absFile)
		content, err := doccontent.Get(doc.ID, s.docStorage, pageFile)
		if err != nil {
			logx.Infof("[SchemaService] Failed to read doc %s: %v", pageFile, err)
			errorMsgs = append(errorMsgs, fmt.Sprintf("read %s: %v", doc.Name, err))
			continue
		}
		if content == "" {
			logx.Infof("[SchemaService] Doc %s is empty (file not found at %s)", pageFile, absFile)
			// 尝试列出目录内容帮助诊断
			docDir := filepath.Dir(absFile)
			if entries, err := os.ReadDir(docDir); err == nil {
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				logx.Infof("[SchemaService] Dir %s contains: %v", docDir, names)
			} else {
				logx.Infof("[SchemaService] Cannot read dir %s: %v", docDir, err)
			}
			continue
		}
		logx.Infof("[SchemaService] Doc %s content length: %d", doc.Name, len(content))

		// 提取SQL代码块
		sqlBlocks := aischema.ExtractSQLBlocks(content)
		logx.Infof("[SchemaService] Doc %s: found %d SQL blocks", doc.Name, len(sqlBlocks))
		if len(sqlBlocks) == 0 {
			continue
		}

		// 解析每个SQL块
		for _, sql := range sqlBlocks {
			cache, err := s.parser.ParseDDL(sql)
			if err != nil {
				logx.Infof("[SchemaService] Failed to parse SQL in doc %s: %v", doc.Name, err)
				continue
			}
			if cache.TableNam == "" {
				continue
			}

			foundTables++
			logx.Infof("[SchemaService] Found table: %s", cache.TableNam)

			// 从约束中提取主键列
			var primaryKeys []string
			for _, c := range cache.ConstraintsJSON {
				if c.ConstraintType == "PRIMARY KEY" {
					primaryKeys = append(primaryKeys, c.Columns...)
				}
			}

			// 保存表元数据
			tableModel := &schemamodel.SchemaTable{
				SchemaSpaceID: schemaSpaceID,
				Name:          cache.TableNam,
				TableComment:  cache.TableComment,
				PrimaryKey:    strings.Join(primaryKeys, ","),
				DocID:         doc.ID,
				DocPath:       doc.Path,
				RawDDL:        cache.DDLContent,
				Status:        1,
			}
			if err := s.schemaRepo.CreateTable(tableModel); err != nil {
				logx.Infof("[SchemaService] Failed to create table %s: %v", cache.TableNam, err)
				continue
			}

			// 构建外键列集合，用于标记列的外键属性
			fkColumnSet := make(map[string]bool)
			for _, c := range cache.ConstraintsJSON {
				if c.ConstraintType == "FOREIGN KEY" {
					for _, col := range c.Columns {
						fkColumnSet[col] = true
					}
				}
			}

			// 保存字段
			for _, col := range cache.ColumnsJSON {
				colModel := &schemamodel.SchemaColumn{
					SchemaSpaceID: schemaSpaceID,
					TableID:       tableModel.ID,
					Table:         cache.TableNam,
					ColumnName:    col.Name,
					DataType:      col.Type,
					Nullable:      1,
					DefaultValue:  col.DefaultValue,
					ColumnComment: col.Comment,
					IsPrimaryKey:  0,
					IsForeignKey:  0,
					IsIndex:       0,
				}
				if !col.IsNullable {
					colModel.Nullable = 0
				}
				if col.IsPK {
					colModel.IsPrimaryKey = 1
				}
				if fkColumnSet[col.Name] {
					colModel.IsForeignKey = 1
				}
				if err := s.schemaRepo.CreateColumn(colModel); err != nil {
					logx.Infof("[SchemaService] Failed to create column %s.%s: %v", cache.TableNam, col.Name, err)
				}
			}

			// 保存外键关系
			for _, c := range cache.ConstraintsJSON {
				if c.ConstraintType == "FOREIGN KEY" && len(c.Columns) > 0 && len(c.RefColumns) > 0 {
					rel := &schemamodel.SchemaRelation{
						SchemaSpaceID: schemaSpaceID,
						SourceTable:   cache.TableNam,
						SourceColumn:  c.Columns[0],
						TargetTable:   c.RefTable,
						TargetColumn:  c.RefColumns[0],
						RelationType:  "one_to_many",
						Confidence:    1.0,
						Source:        "ddl",
					}
					if err := s.schemaRepo.CreateRelation(rel); err != nil {
						logx.Infof("[SchemaService] Failed to create relation: %v", err)
					}
				}
			}

			// 创建分块
			chunks, err := s.chunker.CreateChunks(schemaSpaceID, sql)
			if err != nil {
				logx.Infof("[SchemaService] Failed to create chunks for %s: %v", cache.TableNam, err)
			} else {
				allChunks = append(allChunks, chunks...)
			}
		}
	}

	// 批量写入向量存储
	if len(allChunks) > 0 {
		if err := s.vectorStore.AddChunks(schemaSpaceID, allChunks); err != nil {
			logx.Infof("[SchemaService] Failed to add chunks to vector store: %v", err)
		}
	}

	// 更新Schema空间统计
	space, _ := s.schemaRepo.FindSpaceByID(schemaSpaceID)
	if space != nil {
		now := time.Now()
		space.LastScanAt = &now
		space.DocCount = len(docs)
		space.TableCount = foundTables
		_ = s.schemaRepo.UpdateSpace(space)
	}

	// 完成任务
	completedAt := time.Now()
	task.Status = "completed"
	task.FoundTables = foundTables
	task.CompletedAt = &completedAt
	if len(errorMsgs) > 0 {
		task.ErrorMessage = strings.Join(errorMsgs, "; ")
	}
	if err := s.schemaRepo.UpdateScanTask(task); err != nil {
		logx.Infof("[SchemaService] Failed to complete task: %v", err)
	}
	logx.Infof("[SchemaService] Scan task %d completed, found %d tables", taskID, foundTables)
}

// failScanTask 标记扫描任务失败
func (s *SchemaService) failScanTask(taskID int, errMsg string) {
	task, _ := s.schemaRepo.FindScanTaskByID(taskID)
	if task == nil {
		return
	}
	completedAt := time.Now()
	task.Status = "failed"
	task.ErrorMessage = errMsg
	task.CompletedAt = &completedAt
	_ = s.schemaRepo.UpdateScanTask(task)
}

// cancelScanTask 标记扫描任务为已取消
func (s *SchemaService) cancelScanTask(taskID int) {
	task, _ := s.schemaRepo.FindScanTaskByID(taskID)
	if task == nil {
		return
	}
	completedAt := time.Now()
	task.Status = "cancelled"
	task.CompletedAt = &completedAt
	_ = s.schemaRepo.UpdateScanTask(task)
}

// PauseScanTask 暂停扫描任务
func (s *SchemaService) PauseScanTask(taskID int) error {
	task, err := s.schemaRepo.FindScanTaskByID(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}
	if task.Status != "running" {
		return fmt.Errorf("cannot pause task with status: %s", task.Status)
	}
	task.Status = "paused"
	return s.schemaRepo.UpdateScanTask(task)
}

// ResumeScanTask 恢复扫描任务
func (s *SchemaService) ResumeScanTask(taskID int) error {
	task, err := s.schemaRepo.FindScanTaskByID(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}
	if task.Status != "paused" {
		return fmt.Errorf("cannot resume task with status: %s", task.Status)
	}
	task.Status = "running"
	return s.schemaRepo.UpdateScanTask(task)
}

// CancelScanTask 取消扫描任务
func (s *SchemaService) CancelScanTask(taskID int) error {
	task, err := s.schemaRepo.FindScanTaskByID(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}
	if task.Status != "running" && task.Status != "paused" && task.Status != "pending" {
		return fmt.Errorf("cannot cancel task with status: %s", task.Status)
	}

	// 调用 cancel 函数停止 goroutine
	if cancelFn, ok := s.activeScans.Load(taskID); ok {
		if fn, ok := cancelFn.(context.CancelFunc); ok {
			fn()
		}
	}

	task.Status = "cancelled"
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	return s.schemaRepo.UpdateScanTask(task)
}

// GetScanTaskStatus 获取扫描任务状态
func (s *SchemaService) GetScanTaskStatus(taskID int) (*schemamodel.SchemaScanTask, error) {
	return s.schemaRepo.FindScanTaskByID(taskID)
}

// ListScanTasks 列出扫描任务
func (s *SchemaService) ListScanTasks(schemaSpaceID int) ([]*schemamodel.SchemaScanTask, error) {
	return s.schemaRepo.ListScanTasks(schemaSpaceID)
}

// DebugScan 调试扫描（同步执行）
func (s *SchemaService) DebugScan(schemaSpaceID int) (map[string]interface{}, error) {
	space, err := s.schemaRepo.FindSpaceByID(schemaSpaceID)
	if err != nil {
		return nil, err
	}

	docs, err := s.docRepo.FindBySpaceID(space.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents: %w", err)
	}

	var logs []string
	logs = append(logs, fmt.Sprintf("Found %d documents in wiki space %d", len(docs), space.SpaceID))

	foundTables := 0
	for i, doc := range docs {
		pageFile := s.getPageFilePath(doc)
		logs = append(logs, fmt.Sprintf("Doc %d: %s (file: %s)", i+1, doc.Name, pageFile))

		content, err := doccontent.Get(doc.ID, s.docStorage, pageFile)
		if err != nil {
			logs = append(logs, fmt.Sprintf("  ERROR reading: %v", err))
			continue
		}
		if content == "" {
			logs = append(logs, "  EMPTY content")
			continue
		}
		logs = append(logs, fmt.Sprintf("  Content length: %d", len(content)))

		sqlBlocks := aischema.ExtractSQLBlocks(content)
		logs = append(logs, fmt.Sprintf("  SQL blocks: %d", len(sqlBlocks)))

		for j, sql := range sqlBlocks {
			cache, err := s.parser.ParseDDL(sql)
			if err != nil {
				logs = append(logs, fmt.Sprintf("  SQL %d parse error: %v", j+1, err))
				continue
			}
			if cache.TableNam == "" {
				continue
			}
			foundTables++
			logs = append(logs, fmt.Sprintf("  SQL %d: table=%s, cols=%d", j+1, cache.TableNam, len(cache.ColumnsJSON)))
		}
	}

	return map[string]interface{}{
		"wiki_space_id": space.SpaceID,
		"doc_count":     len(docs),
		"found_tables":  foundTables,
		"logs":          logs,
	}, nil
}

// DeleteScanTask 删除扫描任务
func (s *SchemaService) DeleteScanTask(taskID int) error {
	return s.schemaRepo.DeleteScanTask(taskID)
}

// DeleteAllScanTasks 删除所有扫描任务
func (s *SchemaService) DeleteAllScanTasks(schemaSpaceID int) error {
	return s.schemaRepo.DeleteScanTasksBySpace(schemaSpaceID)
}

// ListTables 列出表
func (s *SchemaService) ListTables(schemaSpaceID int, keyword string, page, pageSize int) ([]*schemamodel.SchemaTable, int64, error) {
	return s.schemaRepo.ListTables(schemaSpaceID, keyword, page, pageSize)
}

// ListTableCache 列出表缓存（按 SchemaSpace 作用域）
func (s *SchemaService) ListTableCache(schemaSpaceID int, keyword string, page, pageSize int) ([]modelai.TableCache, int64, error) {
	return s.schemaRepo.ListTableCache(schemaSpaceID, keyword, page, pageSize)
}

// ListBusinessRules 列出业务规则（按 SchemaSpace 作用域）
func (s *SchemaService) ListBusinessRules(schemaSpaceID int, page, pageSize int) ([]modelai.BusinessRule, int64, error) {
	return s.schemaRepo.ListBusinessRules(schemaSpaceID, page, pageSize)
}

// CreateBusinessRule 创建业务规则（按 SchemaSpace 作用域）
func (s *SchemaService) CreateBusinessRule(rule *modelai.BusinessRule) error {
	return s.schemaRepo.CreateBusinessRule(rule)
}

// DeleteBusinessRule 删除业务规则（按 SchemaSpace 作用域）
func (s *SchemaService) DeleteBusinessRule(id uint64) error {
	return s.schemaRepo.DeleteBusinessRule(id)
}

// RefreshKnowledge 刷新知识库（按 SchemaSpace 作用域）
func (s *SchemaService) RefreshKnowledge(schemaSpaceID int) error {
	// 获取该 SchemaSpace 下的所有表
	tables, err := s.schemaRepo.ListAllTables(schemaSpaceID)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}

	// 清空向量集合
	s.vectorStore.ClearCollection(schemaSpaceID)

	// 重建向量索引
	var allChunks []*aischema.SchemaChunk
	for _, table := range tables {
		chunks, err := s.chunker.CreateChunks(schemaSpaceID, table.RawDDL)
		if err != nil {
			logx.Infof("[SchemaService] Failed to create chunks for %s: %v", table.Name, err)
		} else {
			allChunks = append(allChunks, chunks...)
		}
	}

	// 批量写入向量存储
	if len(allChunks) > 0 {
		if err := s.vectorStore.AddChunks(schemaSpaceID, allChunks); err != nil {
			return fmt.Errorf("add chunks: %w", err)
		}
	}

	return nil
}

// GetTableDetail 获取表详情
func (s *SchemaService) GetTableDetail(schemaSpaceID int, tableName string) (*schemamodel.SchemaTable, []*schemamodel.SchemaColumn, []*schemamodel.SchemaRelation, error) {
	table, err := s.schemaRepo.FindTableByName(schemaSpaceID, tableName)
	if err != nil {
		return nil, nil, nil, err
	}

	columns, _ := s.schemaRepo.ListColumnsByTableName(schemaSpaceID, tableName)
	relations, _ := s.schemaRepo.ListRelationsByTable(schemaSpaceID, tableName)

	return table, columns, relations, nil
}

// AskQuestion 问答生成SQL
// ctx 携带请求上下文（含调用者身份），供 provider 层做 Token 配额预检；
// 同时以 textToSQLBudget 为上限，避免客户端断开后 LLM 仍继续跑。
func (s *SchemaService) AskQuestion(ctx context.Context, schemaSpaceID int, question string) (*modelai.SQLResult, error) {
	if s.textToSQLGenerator != nil {
		space, err := s.schemaRepo.FindSpaceByID(schemaSpaceID)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, textToSQLBudget)
		defer cancel()
		return s.textToSQLGenerator.GenerateWithSpaceContext(ctx, question, uint64(space.SpaceID))
	}
	return nil, errors.New("Text2SQL生成器未初始化")
}

// AskQuestionWithHistory 多轮对话问答生成SQL
// ctx 携带请求上下文（含调用者身份），供 provider 层做 Token 配额预检；
// 以 textToSQLBudget 为上限，避免客户端断开后 LLM 仍继续跑。
func (s *SchemaService) AskQuestionWithHistory(ctx context.Context, schemaSpaceID int, question string, history []text2sql.ConversationMessage) (*modelai.SQLResult, error) {
	if s.textToSQLGenerator == nil {
		return nil, errors.New("Text2SQL生成器未初始化")
	}
	space, err := s.schemaRepo.FindSpaceByID(schemaSpaceID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, textToSQLBudget)
	defer cancel()
	result, _, err := s.textToSQLGenerator.GenerateWithConversation(ctx, question, uint64(space.SpaceID), history)
	return result, err
}

// GetSchemaStats 获取Schema统计
func (s *SchemaService) GetSchemaStats(schemaSpaceID int) (map[string]interface{}, error) {
	tables, err := s.schemaRepo.ListAllTables(schemaSpaceID)
	if err != nil {
		return nil, err
	}

	// 一次 COUNT 而不是逐表把列全读进来再数（见 CountColumnsBySpace）。
	totalColumns, err := s.schemaRepo.CountColumnsBySpace(schemaSpaceID)
	if err != nil {
		return nil, err
	}

	// 这两条原先是 `relations, _ :=` / `tasks, _ :=`：出错就是 0，
	// 于是面板上写着"0 个关系、0 个任务"，看起来像这个空间真的什么都没有。
	relations, err := s.schemaRepo.ListRelations(schemaSpaceID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.schemaRepo.ListScanTasks(schemaSpaceID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"table_count":     len(tables),
		"column_count":    totalColumns,
		"relation_count":  len(relations),
		"scan_task_count": len(tasks),
	}, nil
}

// getPageFilePath 获取文档文件路径
func (s *SchemaService) getPageFilePath(doc *workspacemodel.Document) string {
	space, _ := s.spaceRepo.FindByID(doc.SpaceID)
	spaceName := fmt.Sprintf("space_%d", doc.SpaceID)
	if space != nil {
		spaceName = space.Name
	}

	if doc.Path == "0" || doc.Path == "" {
		if doc.Name == spaceName && doc.Type == workspacemodel.DocumentTypeDir {
			return filepath.Join(spaceName, utils.DocumentDefaultName+utils.DocumentPageSuffix)
		}
		return s.docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
	}

	pathParts := strings.Split(doc.Path, ",")
	var parentPath string
	for i := 0; i < len(pathParts)-1; i++ {
		pid := pathParts[i]
		if pid == "0" || pid == "" {
			continue
		}
		parentID := 0
		fmt.Sscanf(pid, "%d", &parentID)
		if parentID > 0 {
			parentDoc, _ := s.docRepo.FindByID(parentID)
			if parentDoc != nil {
				if parentPath == "" {
					parentPath = parentDoc.Name
				} else {
					parentPath = filepath.Join(parentPath, parentDoc.Name)
				}
			}
		}
	}

	if parentPath == "" {
		return s.docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
	}
	return s.docStorage.GetPageFileByPath(spaceName, parentPath, doc.Name, doc.Type)
}

// ============================================================
// Text2SQL 合并方法（从 text_to_sql_service.go 迁移）
// ============================================================

// TextToSQLGenerateSQL 生成 SQL（Text2SQL）
func (s *SchemaService) TextToSQLGenerateSQL(text string, spaceID uint64) (*modelai.SQLResult, error) {
	if s.textToSQLGenerator == nil {
		return nil, fmt.Errorf("text2sql not initialized")
	}
	if spaceID > 0 {
		return s.textToSQLGenerator.GenerateWithSpace(text, spaceID)
	}
	return s.textToSQLGenerator.Generate(text)
}

// TextToSQLGenerateWithConversation 多轮对话生成 SQL
func (s *SchemaService) TextToSQLGenerateWithConversation(text string, spaceID uint64, history []text2sql.ConversationMessage) (*modelai.SQLResult, *text2sql.ClarificationResult, error) {
	if s.textToSQLGenerator == nil {
		return nil, nil, fmt.Errorf("text2sql not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), textToSQLBudget)
	defer cancel()
	return s.textToSQLGenerator.GenerateWithConversation(ctx, text, spaceID, history)
}

// TextToSQLGenerateSQLWithRAG 使用 RAG 增强生成 SQL（兼容旧签名，无调用者身份）
func (s *SchemaService) TextToSQLGenerateSQLWithRAG(text string, spaceID uint64) (*TextToSQLRAGResult, error) {
	return s.TextToSQLGenerateSQLWithRAGContext(context.Background(), text, spaceID)
}

// TextToSQLGenerateSQLWithRAGContext 带请求上下文版本：ctx 携带真实调用者身份
// （provider.ContextWithUserID），供 provider 层做 Token 配额预检。
func (s *SchemaService) TextToSQLGenerateSQLWithRAGContext(ctx context.Context, text string, spaceID uint64) (*TextToSQLRAGResult, error) {
	if s.textToSQLGenerator == nil {
		return nil, fmt.Errorf("text2sql not initialized")
	}
	result, ragSources, err := s.textToSQLGenerator.GenerateWithRAGContext(ctx, text, spaceID)
	if err != nil {
		return nil, err
	}
	return &TextToSQLRAGResult{
		SQLResult:  result,
		RAGSources: ragSources,
	}, nil
}

// TextToSQLRAGResult RAG 增强的 SQL 生成结果
type TextToSQLRAGResult struct {
	*modelai.SQLResult
	RAGSources []text2sql.RAGSchemaSource `json:"rag_sources,omitempty"`
}

// TextToSQLImportDDL 导入 DDL
func (s *SchemaService) TextToSQLImportDDL(spaceID uint64, ddlContents []string) (int, error) {
	if s.textToSQLRepo == nil || s.textToSQLParser == nil {
		return 0, fmt.Errorf("text2sql not initialized")
	}
	count := 0
	for _, ddl := range ddlContents {
		cache, err := s.textToSQLParser.ParseDDL(ddl)
		if err != nil {
			continue
		}
		if cache.TableNam == "" {
			continue
		}
		cache.SpaceID = &spaceID
		cache.SchemaSpaceID = &spaceID
		if err := s.textToSQLRepo.UpsertTableCache(cache); err != nil {
			continue
		}
		count++
	}

	caches, _ := s.textToSQLRepo.GetAllTableCaches()
	relations := s.textToSQLParser.InferRelations(caches)
	for _, rel := range relations {
		s.textToSQLRepo.UpsertRelation(&rel)
	}

	if s.textToSQLGenerator != nil {
		if err := s.textToSQLGenerator.RefreshKnowledge(); err != nil {
			return count, fmt.Errorf("refresh knowledge: %w", err)
		}
	}

	// 导入后异步采集列统计
	if s.statsCollector != nil && count > 0 {
		go func() {
			if err := s.CollectColumnStats(spaceID); err != nil {
				logx.Infof("[SchemaService] Auto collect column stats failed: %v", err)
			} else {
				logx.Infof("[SchemaService] Auto collect column stats completed for space %d", spaceID)
			}
		}()
	}

	return count, nil
}

// TextToSQLCreateRule 创建业务规则
func (s *SchemaService) TextToSQLCreateRule(rule *modelai.BusinessRule) error {
	return s.textToSQLRepo.CreateRule(rule)
}

// TextToSQLGetRule 获取业务规则
func (s *SchemaService) TextToSQLGetRule(id uint64) (*modelai.BusinessRule, error) {
	return s.textToSQLRepo.FindByID(id)
}

// TextToSQLUpdateRule 更新业务规则
func (s *SchemaService) TextToSQLUpdateRule(rule *modelai.BusinessRule) error {
	return s.textToSQLRepo.UpdateRule(rule)
}

// TextToSQLDeleteRule 删除业务规则
func (s *SchemaService) TextToSQLDeleteRule(id uint64) error {
	return s.textToSQLRepo.DeleteRule(id)
}

// TextToSQLListRules 列出业务规则
func (s *SchemaService) TextToSQLListRules(page, pageSize int, ruleType, tableName string) ([]modelai.BusinessRule, int64, error) {
	return s.textToSQLRepo.ListRules(page, pageSize, ruleType, tableName)
}

// TextToSQLListTables 列出表缓存
func (s *SchemaService) TextToSQLListTables(page, pageSize int, keyword string) ([]modelai.TableCache, int64, error) {
	// page/page_size 直接来自查询串且调用方不校验：page=0 或负数会让旧实现的内存切片
	// 越界 panic。统一钳制到合法区间，并把分页下推到 SQL（不再全表加载）。
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if keyword != "" {
		tables, err := s.textToSQLRepo.SearchTables(keyword)
		return tables, int64(len(tables)), err
	}

	return s.textToSQLRepo.ListTableCaches(page, pageSize)
}

// TextToSQLGetTableDetail 获取表缓存详情
func (s *SchemaService) TextToSQLGetTableDetail(tableName string) (*modelai.TableCache, error) {
	return s.textToSQLRepo.GetTableCache(tableName)
}

// TextToSQLGetTableRelations 获取表关系
func (s *SchemaService) TextToSQLGetTableRelations(tableName string) ([]modelai.TableRelation, error) {
	return s.textToSQLRepo.GetRelationsByTable(tableName)
}

// TextToSQLCreateRelation 创建表关系
func (s *SchemaService) TextToSQLCreateRelation(rel *modelai.TableRelation) error {
	return s.textToSQLRepo.UpsertRelation(rel)
}

// TextToSQLDeleteRelation 删除表关系
func (s *SchemaService) TextToSQLDeleteRelation(id uint64) error {
	return s.textToSQLRepo.DeleteRelation(id)
}

// TextToSQLRefreshKnowledge 刷新知识库
func (s *SchemaService) TextToSQLRefreshKnowledge() error {
	if s.textToSQLGenerator == nil {
		return fmt.Errorf("text2sql not initialized")
	}
	return s.textToSQLGenerator.RefreshKnowledge()
}

// TextToSQLGetPromptTemplate 获取 prompt 模板
func (s *SchemaService) TextToSQLGetPromptTemplate() string {
	if s.textToSQLGenerator == nil {
		return ""
	}
	return s.textToSQLGenerator.GetPromptTemplate()
}

// TextToSQLSetPromptTemplate 设置 prompt 模板
func (s *SchemaService) TextToSQLSetPromptTemplate(tpl string) {
	if s.textToSQLGenerator != nil {
		s.textToSQLGenerator.SetPromptTemplate(tpl)
	}
}

// TextToSQLListPromptTemplates 列出提示词模板
func (s *SchemaService) TextToSQLListPromptTemplates(spaceID uint64, includeGlobal bool) ([]modelai.PromptTemplate, error) {
	repo := airepo.NewPromptTemplateRepo(repos.DB)
	return repo.List(spaceID, includeGlobal)
}

// TextToSQLCreatePromptTemplate 创建提示词模板
func (s *SchemaService) TextToSQLCreatePromptTemplate(tpl *modelai.PromptTemplate) error {
	repo := airepo.NewPromptTemplateRepo(repos.DB)
	return repo.Create(tpl)
}

// TextToSQLGetPromptTemplateByID 获取单个提示词模板
func (s *SchemaService) TextToSQLGetPromptTemplateByID(id uint64) (*modelai.PromptTemplate, error) {
	repo := airepo.NewPromptTemplateRepo(repos.DB)
	return repo.FindByID(id)
}

// TextToSQLUpdatePromptTemplate 更新提示词模板
func (s *SchemaService) TextToSQLUpdatePromptTemplate(tpl *modelai.PromptTemplate) error {
	repo := airepo.NewPromptTemplateRepo(repos.DB)
	return repo.Update(tpl)
}

// TextToSQLDeletePromptTemplate 删除提示词模板
func (s *SchemaService) TextToSQLDeletePromptTemplate(id uint64) error {
	repo := airepo.NewPromptTemplateRepo(repos.DB)
	return repo.Delete(id)
}

// TextToSQLSetDefaultPromptTemplate 设置默认模板
func (s *SchemaService) TextToSQLSetDefaultPromptTemplate(id uint64, spaceID uint64) error {
	repo := airepo.NewPromptTemplateRepo(repos.DB)
	return repo.SetDefault(id, spaceID)
}

// TextToSQLTogglePromptTemplate 切换模板激活状态
func (s *SchemaService) TextToSQLTogglePromptTemplate(id uint64, active bool) error {
	repo := airepo.NewPromptTemplateRepo(repos.DB)
	return repo.ToggleActive(id, active)
}

// TextToSQLScanSpace 扫描指定空间中的 SQL 建表语句
func (s *SchemaService) TextToSQLScanSpace(spaceID uint64) (*text2sql.ScanResult, error) {
	if s.textToSQLRepo == nil {
		return nil, fmt.Errorf("text2sql not initialized")
	}

	scanner := text2sql.NewSpaceScanner(s.docRepo, s.spaceRepo, s.docStorage, s.textToSQLRepo, s.vectorSyncCallback)
	result, err := scanner.ScanSpace(int(spaceID))
	if err != nil {
		return nil, err
	}

	if s.textToSQLGenerator != nil {
		if err := s.textToSQLGenerator.RefreshKnowledge(); err != nil {
			return result, fmt.Errorf("刷新知识库失败: %w", err)
		}
	}

	return result, nil
}

// TextToSQLGetSpaceTables 获取指定空间的表列表
func (s *SchemaService) TextToSQLGetSpaceTables(spaceID uint64) ([]modelai.TableCache, error) {
	return s.textToSQLRepo.GetTableCachesBySpace(spaceID)
}

// TextToSQLScanAndImportAllDocs 扫描所有文档中的 DDL 并导入表缓存
func (s *SchemaService) TextToSQLScanAndImportAllDocs() (int, error) {
	if s.textToSQLRepo == nil || s.textToSQLParser == nil {
		return 0, fmt.Errorf("text2sql not initialized")
	}

	docs, err := s.docRepo.FindAllDocuments()
	if err != nil {
		return 0, fmt.Errorf("find all documents: %w", err)
	}

	sqlBlockRe := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	createTableRe := regexp.MustCompile("(?i)CREATE\\s+TABLE\\s+")
	var count int

	for _, doc := range docs {
		if doc.Type != workspacemodel.DocumentTypePage {
			continue
		}

		pageFile := s.getPageFilePath(doc)
		content, err := doccontent.Get(doc.ID, s.docStorage, pageFile)
		if err != nil || content == "" {
			continue
		}

		matches := sqlBlockRe.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			sqlContent := match[1]
			if !createTableRe.MatchString(sqlContent) {
				continue
			}

			cache, err := s.textToSQLParser.ParseDDL(sqlContent)
			if err != nil || cache.TableNam == "" {
				continue
			}

			spaceID := uint64(doc.SpaceID)
			cache.SpaceID = &spaceID
			cache.SchemaSpaceID = &spaceID

			if err := s.textToSQLRepo.UpsertTableCache(cache); err != nil {
				logx.Infof("[SchemaService] Failed to save table cache %s: %v", cache.TableNam, err)
				continue
			}
			count++
		}
	}

	if s.textToSQLGenerator != nil {
		if err := s.textToSQLGenerator.RefreshKnowledge(); err != nil {
			return count, fmt.Errorf("refresh knowledge: %w", err)
		}
	}

	caches, _ := s.textToSQLRepo.GetAllTableCaches()
	relations := s.textToSQLParser.InferRelations(caches)
	for _, rel := range relations {
		rel.SpaceID = nil
		s.textToSQLRepo.UpsertRelation(&rel)
	}

	return count, nil
}
