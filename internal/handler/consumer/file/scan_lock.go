package file

import "sync"

// scanLocks 跟踪每个挂载点是否正在被扫描。
// 同一挂载点在同一时刻只允许一个扫描任务运行，其余会被快速拒绝并返回特殊错误 ErrScanInProgress。
//
// 这是一个"软"锁：仅限当前进程内，不跨进程；对 SQLite/单实例部署足够。
var scanLocks sync.Map // map[int64]struct{}

// acquireScanLock 尝试获取指定挂载点文件 ID 的扫描锁。
// 成功返回 true + release 函数；失败返回 false。
func acquireScanLock(fileId int64) (release func(), ok bool) {
	if fileId <= 0 {
		return func() {}, true
	}

	if _, loaded := scanLocks.LoadOrStore(fileId, struct{}{}); loaded {
		return nil, false
	}

	return func() {
		scanLocks.Delete(fileId)
	}, true
}
