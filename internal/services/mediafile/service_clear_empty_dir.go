package mediafile

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

// ClearEmptyDir 清理空目录（包括子目录）
func (s *service) ClearEmptyDir(ctx context.Context, entryPath string) error {
	validatedPath, err := validateMediaStorageRoot(entryPath)
	if err != nil {
		ctx.Error("拒绝清理非法媒体空目录根路径", zap.String("path", entryPath), zap.Error(err))

		return err
	}

	entryPath = validatedPath

	var dirs []string

	err = filepath.WalkDir(entryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			dirs = append(dirs, path)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// 按路径长度降序对目录进行排序
	// 这样我们就可以先处理最深的目录。
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})

	for _, dir := range dirs {
		if dir == entryPath {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			// 忽略读取目录的错误，也许它已经被删除了。
			continue
		}

		if len(entries) == 0 {
			// 目录为空，删除它。
			// 忽略错误，因为另一个并发进程可能已经删除了它。
			_ = os.Remove(dir)
		}
	}

	return nil
}
