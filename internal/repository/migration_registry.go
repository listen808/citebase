package repository

// 表迁移注册表：各领域通过 init() 注册自己的模型，InitDB 统一遍历执行 AutoMigrate。
// 新增表时只需在对应领域的 migrate_*.go 文件中追加，无需修改 InitDB，
// 降低 db.go 与所有 model 包之间的耦合。

// migrationModels 汇总所有已注册的待迁移模型（按注册顺序保留，保证外键/依赖表先建）。
var migrationModels []interface{}

// RegisterModels 注册一批待自动迁移的模型。通常在各领域文件的 init() 中调用。
// 注册顺序即迁移顺序，被依赖的基础表应先注册。
func RegisterModels(models ...interface{}) {
	migrationModels = append(migrationModels, models...)
}

// registeredModels 返回所有已注册模型的快照。
func registeredModels() []interface{} {
	out := make([]interface{}, len(migrationModels))
	copy(out, migrationModels)
	return out
}

// RegisteredModels 返回 AutoMigrate 注册表中的全部模型，供 tools/genddl 等外部工具生成 DDL。
// 这是表清单的唯一真源：新增表只需在 migrate_*.go 注册，DDL 与运行时结构自动保持一致。
func RegisteredModels() []interface{} {
	return registeredModels()
}

// IndexSpec 是一条复合/前缀索引声明，字段语义与 db.go 内部的 indexDef 一致。
type IndexSpec struct {
	Table  string
	Name   string
	Unique bool
	Cols   string
}

// AllIndexSpecs 返回 ensureAllIndexes 使用的索引清单，供 tools/genddl 生成 DDL 时共用。
// 复合索引以此为准，避免 table.sql 与运行时索引集合出现两套真相。
func AllIndexSpecs() []IndexSpec {
	out := make([]IndexSpec, 0, len(allIndexes))
	for _, i := range allIndexes {
		out = append(out, IndexSpec{Table: i.table, Name: i.name, Unique: i.unique, Cols: i.cols})
	}
	return out
}
