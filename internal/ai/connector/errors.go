package connector

import "fmt"

// errZipEntryNotFound 表示在 ZIP 包中找不到指定条目。
func errZipEntryNotFound(name string) error {
	return fmt.Errorf("zip 条目未找到: %s", name)
}
