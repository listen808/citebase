package search

import (
	"io/fs"
	"path/filepath"
)

// IndexPath 当前索引目录的绝对路径；未初始化时为空串。
func IndexPath() string {
	return savedIndexPath
}

// IndexDiskFacts 实测索引目录的总字节数与最后一次段文件写入时间。
//
// 为什么走文件系统而不是 Bleve API：Bleve 只给 DocCount，既没有体积也没有
// 更新时间。原先 service 层把更新时间写成 time.Now()，于是监控页每刷新一次
// 都报"刚刚更新过"，索引实际早已停更也看不出来——这类值不能出现在监控页上。
//
// 不加锁：只读目录元数据，不碰全局 index。重建期间目录被换掉或某个文件被
// 短暂占用时，WalkDir 对该项返回 err，按跳过处理即可，不影响其余统计。
func IndexDiskFacts() (size int64, lastUpdated int64) {
	dir := savedIndexPath
	if dir == "" {
		return 0, 0
	}
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		size += info.Size()
		if mt := info.ModTime().Unix(); mt > lastUpdated {
			lastUpdated = mt
		}
		return nil
	})
	return size, lastUpdated
}
