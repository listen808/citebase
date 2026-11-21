package social

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// TestIsDuplicateKey 钉住这条判据本身：它是并发双击不再报 500 的唯一依据，
// 判错了就是"把真的数据库错误当成都功"或"把重复当成失败"，两个方向都静默。
func TestIsDuplicateKey(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"驱动原生 1062", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}, true},
		{"包装过一层的 1062", fmt.Errorf("create collection: %w", &mysql.MySQLError{Number: 1062}), true},
		{"gorm 再包一层", fmt.Errorf("wrap: %w", fmt.Errorf("x: %w", &mysql.MySQLError{Number: 1062})), true},
		{"别的 MySQL 错误号", &mysql.MySQLError{Number: 1048, Message: "Column cannot be null"}, false},
		{"1062 的文案兜底", errors.New("Error 1062: Duplicate entry '1-2-3' for key 'uk_user_resource'"), true},
		{"无关错误", errors.New("connection refused"), false},
		{"记录不存在不是重复", gorm.ErrRecordNotFound, false},
	}
	for _, c := range cases {
		if got := IsDuplicateKey(c.err); got != c.want {
			t.Errorf("%s: IsDuplicateKey(%v) = %v，期望 %v", c.name, c.err, got, c.want)
		}
	}
}
