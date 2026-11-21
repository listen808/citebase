package repository

import (
	evalmodel "github.com/listen808/citebase/internal/model/eval"
	schemamodel "github.com/listen808/citebase/internal/model/schema"
)

// Schema Intelligence 与 RAG 评估框架相关表。
func init() {
	RegisterModels(
		// Schema Intelligence
		&schemamodel.SchemaSpace{},
		&schemamodel.SchemaTable{},
		&schemamodel.SchemaColumn{},
		&schemamodel.SchemaRelation{},
		&schemamodel.SchemaBusinessTag{},
		&schemamodel.SchemaScanTask{},

		// RAG 评估框架
		&evalmodel.EvalDataset{}, // 评估数据集
		&evalmodel.EvalResult{},  // 评估结果
	)
}
