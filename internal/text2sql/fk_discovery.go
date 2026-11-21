package text2sql

import (
	"context"
	"fmt"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	schemamodel "github.com/listen808/citebase/internal/model/schema"
	"github.com/listen808/citebase/internal/utils"
)

// ForeignKeyDiscoverer 自动发现表间外键关系
// 通过分析列名模式、数据类型匹配、以及LLM语义推断来发现隐式外键
type ForeignKeyDiscoverer struct {
	schemaRepo  SchemaRepoInterface
	llmProvider LLMProvider
}

// NewForeignKeyDiscoverer 创建外键发现器
func NewForeignKeyDiscoverer(schemaRepo SchemaRepoInterface, llmProvider LLMProvider) *ForeignKeyDiscoverer {
	return &ForeignKeyDiscoverer{
		schemaRepo:  schemaRepo,
		llmProvider: llmProvider,
	}
}

// DiscoveredFK 发现的外键关系
type DiscoveredFK struct {
	SourceTable  string  `json:"source_table"`
	SourceColumn string  `json:"source_column"`
	TargetTable  string  `json:"target_table"`
	TargetColumn string  `json:"target_column"`
	Confidence   float64 `json:"confidence"`
	Method       string  `json:"method"` // "naming" | "type_match" | "llm"
}

// Discover 发现候选表之间的外键关系
func (fkd *ForeignKeyDiscoverer) Discover(ctx context.Context, tables []*schemamodel.SchemaTable, schemaSpaceID int) ([]*DiscoveredFK, error) {
	if len(tables) < 2 {
		return nil, nil
	}

	// 1. 基于命名规则发现（例如 xxx_id, id 匹配）
	namingFKs := fkd.discoverByNamingPattern(tables)

	// 2. 基于类型匹配发现（相同类型+注释的列）
	typeFKs := fkd.discoverByTypeMatch(tables)

	// 3. 合并去重
	fks := mergeFKs(namingFKs, typeFKs)

	// 4. 如果LLM可用，用LLM做语义验证和补充
	if fkd.llmProvider != nil && len(fks) < 20 {
		llmFKs, err := fkd.discoverByLLM(ctx, tables, fks)
		if err != nil {
			logx.Infof("[FKDiscovery] LLM discovery failed: %v, using pattern-based results", err)
		} else {
			fks = mergeFKs(fks, llmFKs)
		}
	}

	logx.Infof("[FKDiscovery] Discovered %d FK relationships for %d tables", len(fks), len(tables))
	return fks, nil
}

// discoverByNamingPattern 基于命名模式发现外键
func (fkd *ForeignKeyDiscoverer) discoverByNamingPattern(tables []*schemamodel.SchemaTable) []*DiscoveredFK {
	var fks []*DiscoveredFK

	// 常见外键命名模式
	suffixes := []string{"_id", "_key", "_fk", "_ref"}

	for _, t1 := range tables {
		for _, t2 := range tables {
			if t1.Name == t2.Name {
				continue
			}
			cols1, _ := fkd.schemaRepo.ListColumnsByTableID(t1.ID)
			cols2, _ := fkd.schemaRepo.ListColumnsByTableID(t2.ID)

			for _, c1 := range cols1 {
				colName := strings.ToLower(c1.ColumnName)

				// 检查是否以常见后缀结尾
				for _, suffix := range suffixes {
					if strings.HasSuffix(colName, suffix) {
						// 提取前缀（例如 user_id → user）
						prefix := strings.TrimSuffix(colName, suffix)

						// 查找目标表是否有匹配的列名
						for _, c2 := range cols2 {
							c2Name := strings.ToLower(c2.ColumnName)
							// 模式1: prefix → id (例如 user_id → users.id)
							// 应检查目标表 t2.Name 是否含 prefix（如 users 含 user），而非源表 t1.Name
							if c2Name == "id" && strings.Contains(strings.ToLower(t2.Name), prefix) {
								fks = append(fks, &DiscoveredFK{
									SourceTable:  t1.Name,
									SourceColumn: c1.ColumnName,
									TargetTable:  t2.Name,
									TargetColumn: c2.ColumnName,
									Confidence:   0.85,
									Method:       "naming",
								})
							}
							// 模式2: prefix_id → prefix_id (同名列)
							// 仅在 t1.Name < t2.Name 时生成，避免双向重复（t1→t2 与 t2→t1）
							if colName == c2Name && t1.Name < t2.Name {
								fks = append(fks, &DiscoveredFK{
									SourceTable:  t1.Name,
									SourceColumn: c1.ColumnName,
									TargetTable:  t2.Name,
									TargetColumn: c2.ColumnName,
									Confidence:   0.70,
									Method:       "naming",
								})
							}
						}
					}
				}
			}
		}
	}
	return fks
}

// discoverByTypeMatch 基于类型匹配发现外键
func (fkd *ForeignKeyDiscoverer) discoverByTypeMatch(tables []*schemamodel.SchemaTable) []*DiscoveredFK {
	var fks []*DiscoveredFK

	for _, t1 := range tables {
		for _, t2 := range tables {
			if t1.Name == t2.Name {
				continue
			}
			cols1, _ := fkd.schemaRepo.ListColumnsByTableID(t1.ID)
			cols2, _ := fkd.schemaRepo.ListColumnsByTableID(t2.ID)

			for _, c1 := range cols1 {
				for _, c2 := range cols2 {
					// 类型匹配 + 列名相似（忽略大小写和后缀）
					if strings.EqualFold(c1.DataType, c2.DataType) {
						c1Name := strings.ToLower(c1.ColumnName)
						c2Name := strings.ToLower(c2.ColumnName)

						// 同名列
						if c1Name == c2Name && isIDLike(c1Name) {
							fks = append(fks, &DiscoveredFK{
								SourceTable:  t1.Name,
								SourceColumn: c1.ColumnName,
								TargetTable:  t2.Name,
								TargetColumn: c2.ColumnName,
								Confidence:   0.75,
								Method:       "type_match",
							})
						}
					}
				}
			}
		}
	}
	return fks
}

// isIDLike 判断列名是否像ID类型
func isIDLike(colName string) bool {
	return strings.HasSuffix(colName, "id") ||
		strings.HasSuffix(colName, "_id") ||
		strings.HasSuffix(colName, "_key") ||
		colName == "id"
}

// discoverByLLM 使用LLM进行语义外键发现
func (fkd *ForeignKeyDiscoverer) discoverByLLM(ctx context.Context, tables []*schemamodel.SchemaTable, existingFKs []*DiscoveredFK) ([]*DiscoveredFK, error) {
	// 构建表结构摘要
	var schemaDesc strings.Builder
	schemaDesc.WriteString("表结构信息：\n\n")
	for _, t := range tables {
		cols, _ := fkd.schemaRepo.ListColumnsByTableID(t.ID)
		schemaDesc.WriteString(fmt.Sprintf("表: %s", t.Name))
		if t.TableComment != "" {
			schemaDesc.WriteString(fmt.Sprintf(" (%s)", t.TableComment))
		}
		schemaDesc.WriteString("\n列:\n")
		for _, c := range cols {
			pk := ""
			if c.IsPrimaryKey == 1 {
				pk = " [主键]"
			}
			comment := ""
			if c.ColumnComment != "" {
				comment = fmt.Sprintf(" -- %s", c.ColumnComment)
			}
			schemaDesc.WriteString(fmt.Sprintf("  - %s (%s)%s%s\n", c.ColumnName, c.DataType, pk, comment))
		}
		schemaDesc.WriteString("\n")
	}

	// 已发现的外键
	if len(existingFKs) > 0 {
		schemaDesc.WriteString("已发现的外键关系（不需要重复）：\n")
		for _, fk := range existingFKs {
			schemaDesc.WriteString(fmt.Sprintf("  - %s.%s → %s.%s\n",
				fk.SourceTable, fk.SourceColumn, fk.TargetTable, fk.TargetColumn))
		}
		schemaDesc.WriteString("\n")
	}

	prompt := fmt.Sprintf(`请分析以下数据库表结构，推断可能的外键关系。
重点发现以下模式：
1. 列名包含另一个表名的（如 orders.user_id → users.id）
2. 同名列在不同表中且语义相同的
3. 根据列注释推断的关联关系

%s
请输出JSON数组格式，每个元素包含：
{
  "source_table": "源表名",
  "source_column": "源表列名",
  "target_table": "目标表名",
  "target_column": "目标表列名",
  "confidence": 0.0-1.0,
  "reason": "推断理由"
}

只输出JSON数组，不要其他内容。`, schemaDesc.String())

	resp, err := fkd.llmProvider.Chat(ctx, "你是一个数据库关系分析专家。", []Message{
		NewTextMessage("user", prompt),
	})
	if err != nil {
		return nil, fmt.Errorf("LLM FK discovery: %w", err)
	}

	return parseLLMFKResponse(resp.Content), nil
}

// parseLLMFKResponse 解析LLM返回的外键JSON
func parseLLMFKResponse(content string) []*DiscoveredFK {
	jsonStr := content
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		jsonStr = content[start : end+1]
	}

	// 简单解析（避免引入encoding/json的复杂解析）
	var fks []*DiscoveredFK
	lines := strings.Split(jsonStr, "{")
	for _, line := range lines {
		if !strings.Contains(line, "source_table") {
			continue
		}
		fk := &DiscoveredFK{Method: "llm", Confidence: 0.7}

		if v := utils.ExtractJSONField(line, "source_table"); v != "" {
			fk.SourceTable = v
		}
		if v := utils.ExtractJSONField(line, "source_column"); v != "" {
			fk.SourceColumn = v
		}
		if v := utils.ExtractJSONField(line, "target_table"); v != "" {
			fk.TargetTable = v
		}
		if v := utils.ExtractJSONField(line, "target_column"); v != "" {
			fk.TargetColumn = v
		}
		if v := utils.ExtractJSONField(line, "confidence"); v != "" {
			fmt.Sscanf(v, "%f", &fk.Confidence)
		}

		if fk.SourceTable != "" && fk.SourceColumn != "" && fk.TargetTable != "" && fk.TargetColumn != "" {
			fks = append(fks, fk)
		}
	}
	return fks
}

// mergeFKs 合并去重外键关系
func mergeFKs(fks1, fks2 []*DiscoveredFK) []*DiscoveredFK {
	seen := make(map[string]*DiscoveredFK)
	all := append(fks1, fks2...)

	for _, fk := range all {
		key := fmt.Sprintf("%s.%s→%s.%s", fk.SourceTable, fk.SourceColumn, fk.TargetTable, fk.TargetColumn)
		if existing, ok := seen[key]; ok {
			// 保留置信度更高的
			if fk.Confidence > existing.Confidence {
				seen[key] = fk
			}
		} else {
			seen[key] = fk
		}
	}

	result := make([]*DiscoveredFK, 0, len(seen))
	for _, fk := range seen {
		result = append(result, fk)
	}
	return result
}
