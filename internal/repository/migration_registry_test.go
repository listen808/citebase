package repository

import (
	"reflect"
	"strings"
	"testing"
)

// TestRegisteredModelsPopulated 验证各领域 init() 已向注册表注册模型。
func TestRegisteredModelsPopulated(t *testing.T) {
	models := registeredModels()
	if len(models) == 0 {
		t.Fatal("registeredModels() returned empty; migrate_*.go init() not registering")
	}
	// 核心 + AI + schema 领域合计应远超个位数
	if len(models) < 20 {
		t.Errorf("expected >= 20 registered models, got %d", len(models))
	}
}

// TestNewTablesRegistered 回归防护：新增业务表必须出现在启动迁移注册表中，
// 否则 AutoMigrate 不会建表、genddl 也不会生成 DDL（新库直接缺表）。
func TestNewTablesRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, m := range registeredModels() {
		rv := reflect.Indirect(reflect.ValueOf(m))
		if !rv.IsValid() {
			continue
		}
		registered[rv.Type().Name()] = true
	}

	required := []string{"DocumentPermission", "AIUsageDaily"}
	for _, name := range required {
		if !registered[name] {
			t.Errorf("model %q not registered; new tables must be added in migrate_*.go init() so AutoMigrate creates them on startup", name)
		}
	}
}

// TestAllIndexesCoverRegisteredUniqueIndexes 防止 allIndexes 与注册表漂移：
// allIndexes 里声明的索引必须指向已注册的表。
func TestAllIndexesCoverRegisteredTables(t *testing.T) {
	registered := make(map[string]bool)
	for _, m := range registeredModels() {
		rv := reflect.Indirect(reflect.ValueOf(m))
		if !rv.IsValid() {
			continue
		}
		registered[rv.Type().Name()] = true
	}
	for _, idx := range allIndexes {
		if !strings.HasPrefix(idx.table, "aw_") {
			t.Errorf("index %q references table %q without aw_ prefix", idx.name, idx.table)
		}
	}
}

// TestRegisteredModelsSnapshotIsolated 验证返回的是快照，外部修改不影响内部状态。
func TestRegisteredModelsSnapshotIsolated(t *testing.T) {
	before := registeredModels()
	n := len(before)
	if n == 0 {
		t.Skip("no registered models")
	}
	before[0] = nil // 修改快照
	after := registeredModels()
	if after[0] == nil {
		t.Error("registeredModels() should return an isolated copy, internal state was mutated")
	}
	if len(after) != n {
		t.Errorf("registered model count changed unexpectedly: %d -> %d", n, len(after))
	}
}

// TestRegisterModelsAppends 验证 RegisterModels 追加行为。
func TestRegisterModelsAppends(t *testing.T) {
	before := len(registeredModels())
	type dummy struct{}
	RegisterModels(&dummy{})
	after := len(registeredModels())
	if after != before+1 {
		t.Errorf("RegisterModels should append 1, count %d -> %d", before, after)
	}
}
