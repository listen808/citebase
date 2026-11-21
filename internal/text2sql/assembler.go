package text2sql

import (
	"fmt"
	"strings"
)

type SQLAssembler struct {
	kb *KnowledgeBase
}

func NewSQLAssembler(kb *KnowledgeBase) *SQLAssembler {
	return &SQLAssembler{kb: kb}
}

func (a *SQLAssembler) Assemble(input *RecognizedInput, joins []JoinPath) string {
	switch input.Intent {
	case "SELECT":
		return a.assembleSelect(input, joins)
	case "COUNT":
		return a.assembleCount(input, joins)
	case "INSERT":
		return a.assembleInsert(input)
	case "UPDATE":
		return a.assembleUpdate(input)
	case "DELETE":
		return a.assembleDelete(input)
	default:
		return a.assembleSelect(input, joins)
	}
}

func (a *SQLAssembler) assembleSelect(input *RecognizedInput, joins []JoinPath) string {
	var sb strings.Builder

	sb.WriteString("SELECT ")

	fields := a.buildSelectFields(input)
	sb.WriteString(fields)

	sb.WriteString("\nFROM ")
	sb.WriteString(input.Tables[0])

	for i, join := range joins {
		alias := fmt.Sprintf("t%d", i+1)
		sb.WriteString(fmt.Sprintf("\n%s %s ON %s.%s = %s.%s",
			join.JoinType,
			join.TargetTable,
			alias,
			join.SourceField,
			alias,
			join.TargetField,
		))
	}

	where := a.buildWhereClause(input)
	if where != "" {
		sb.WriteString("\nWHERE ")
		sb.WriteString(where)
	}

	return sb.String()
}

func (a *SQLAssembler) assembleCount(input *RecognizedInput, joins []JoinPath) string {
	var sb strings.Builder

	sb.WriteString("SELECT COUNT(*) AS count")
	sb.WriteString("\nFROM ")
	sb.WriteString(input.Tables[0])

	for i, join := range joins {
		alias := fmt.Sprintf("t%d", i+1)
		sb.WriteString(fmt.Sprintf("\n%s %s ON %s.%s = %s.%s",
			join.JoinType,
			join.TargetTable,
			alias,
			join.SourceField,
			alias,
			join.TargetField,
		))
	}

	where := a.buildWhereClause(input)
	if where != "" {
		sb.WriteString("\nWHERE ")
		sb.WriteString(where)
	}

	return sb.String()
}

func (a *SQLAssembler) assembleInsert(input *RecognizedInput) string {
	if len(input.Tables) == 0 {
		return ""
	}

	table := input.Tables[0]
	t := a.kb.GetTable(table)
	if t == nil {
		return ""
	}

	var fields []string
	var values []string

	for _, col := range t.ColumnsJSON {
		if col.IsPK {
			continue
		}
		fields = append(fields, col.Name)
		values = append(values, "?")
	}

	return fmt.Sprintf("INSERT INTO %s (%s)\nVALUES (%s)",
		table,
		strings.Join(fields, ", "),
		strings.Join(values, ", "),
	)
}

func (a *SQLAssembler) assembleUpdate(input *RecognizedInput) string {
	if len(input.Tables) == 0 {
		return ""
	}

	table := input.Tables[0]
	var sets []string

	for _, fields := range input.Fields {
		for _, f := range fields {
			sets = append(sets, fmt.Sprintf("%s = ?", f))
		}
	}

	if len(sets) == 0 {
		sets = append(sets, "/* 字段 = 值 */")
	}

	where := a.buildWhereClause(input)

	return fmt.Sprintf("UPDATE %s\nSET %s\nWHERE %s",
		table,
		strings.Join(sets, ", "),
		where,
	)
}

func (a *SQLAssembler) assembleDelete(input *RecognizedInput) string {
	if len(input.Tables) == 0 {
		return ""
	}

	table := input.Tables[0]
	where := a.buildWhereClause(input)

	return fmt.Sprintf("DELETE FROM %s\nWHERE %s",
		table,
		where,
	)
}

func (a *SQLAssembler) buildSelectFields(input *RecognizedInput) string {
	var allFields []string

	for _, fields := range input.Fields {
		allFields = append(allFields, fields...)
	}

	if len(allFields) == 0 {
		return "*"
	}

	return strings.Join(allFields, ", ")
}

func (a *SQLAssembler) buildWhereClause(input *RecognizedInput) string {
	if len(input.Conditions) == 0 {
		return ""
	}

	var parts []string
	for _, cond := range input.Conditions {
		if cond.Op == "LIKE" {
			parts = append(parts, fmt.Sprintf("%s LIKE '%%%s%%'", cond.Field, cond.Value))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s '%s'", cond.Field, cond.Op, cond.Value))
		}
	}

	return strings.Join(parts, " AND ")
}
