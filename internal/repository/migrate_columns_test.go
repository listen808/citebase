package repository

import (
	"reflect"
	"testing"
)

// 这几条是纯静态判据（不连库）：它们要拦的是"补列机制自己漂了"，
// 而那件事在真实库里表现为**功能静默不工作**，不会有任何启动失败。

// criticalColumns 的每一项都必须能在显式补列清单里找到列定义。
// 少了这一条，verifyCriticalColumns 就退化成"发现缺列、报个错、什么也不做" ——
// 而它被写出来的理由正是"缺列不该静默"。
func TestCriticalColumnsHaveFallbackDefinition(t *testing.T) {
	defs := map[string]string{}
	for _, f := range missingColumnFallbacks() {
		defs[f.table+"."+f.col] = f.def
	}
	if len(defs) == 0 {
		t.Fatal("显式补列清单是空的 —— 判据在空转")
	}
	for _, c := range criticalColumns {
		if defs[c.table+"."+c.col] == "" {
			t.Errorf("关键列 %s.%s 不在显式补列清单里：启动核对只会报红、补不了", c.table, c.col)
		}
	}
}

// 清单里的列必须真的对应某个已注册模型上的字段（column tag 或字段名默认转换）。
// 反过来的漂移更常见：模型改了列名，清单还在 ALTER 一个没人读的列，
// 于是每次启动多一列"僵尸字段"，而缺的那一列仍然缺。
func TestCriticalColumnsExistOnRegisteredModels(t *testing.T) {
	type colKey struct{ table, col string }
	present := map[colKey]bool{}
	for _, m := range registeredModels() {
		v := reflect.ValueOf(m)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			continue
		}
		named, ok := m.(interface{ TableName() string })
		if !ok {
			continue
		}
		table := named.TableName()
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			col := f.Name
			if tag := f.Tag.Get("gorm"); tag != "" {
				for _, part := range splitTag(tag) {
					if len(part) > 7 && part[:7] == "column:" {
						col = part[7:]
					}
				}
			}
			present[colKey{table, col}] = true
		}
	}
	for _, c := range criticalColumns {
		if !present[colKey{c.table, c.col}] {
			t.Errorf("关键列 %s.%s 在任何已注册模型上都找不到对应字段 —— 要么模型漏了字段（AutoMigrate 与 genddl 都不会管它），要么这条该从清单里删掉", c.table, c.col)
		}
	}
}

// 清单不许有重复项：重复不只是浪费一次 ALTER，它会把"同一列两个不同定义"这种漂移藏起来
// （第一份生效、第二份永远走不到"列不存在"那一支）。
func TestNoDuplicateFallbackEntries(t *testing.T) {
	seen := map[string]string{}
	for _, f := range missingColumnFallbacks() {
		key := f.table + "." + f.col
		if prev, dup := seen[key]; dup {
			if prev != f.def {
				t.Errorf("%s 在显式清单里有两个不同定义：\n  %s\n  %s", key, prev, f.def)
			} else {
				t.Errorf("%s 在显式清单里重复登记了一次", key)
			}
		}
		seen[key] = f.def
	}
}

func splitTag(tag string) []string {
	out := []string{}
	cur := ""
	for _, r := range tag {
		if r == ';' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}
