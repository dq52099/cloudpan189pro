package mediafile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DeleteStrm 删除单个 STRM 文件及其数据库记录。
// 磁盘删除失败会记录 warning 并继续删除 DB 记录，避免孤儿 DB 记录阻塞后续流程。
func (s *service) DeleteStrm(ctx context.Context, fid int64, rootPath string) error {
	if fid <= 0 {
		return errInvalidMediaFileFID
	}

	file, err := s.QueryStrm(ctx, fid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}

		return err
	}

	// 删除磁盘文件。历史 DB 记录可能包含异常路径；遇到非法路径时只清理 DB 记录。
	diskPath, pathErr := mediaFileDiskPath(rootPath, file.Path)
	if pathErr != nil {
		ctx.Warn("跳过非法 STRM 文件路径（将继续清理 DB 记录）",
			zap.String("path", file.Path),
			zap.Error(pathErr),
		)
	} else if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		ctx.Warn("删除 STRM 文件失败（将继续清理 DB 记录）",
			zap.String("path", diskPath),
			zap.Error(err),
		)
	}

	// 删除记录
	result := s.getDB(ctx).Where("id = ?", file.ID).Delete(new(models.MediaFile))
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.Wrap(gorm.ErrRecordNotFound, "媒体文件记录不存在")
	}

	return nil
}

// Clear 清除根路径下所有文件夹并清空对应 DB 记录。
// 单个文件删除失败时记录 warning 并继续；所有文件处理完后统一清理 DB。
//
// 安全检查：
//   - rootPath 必须与 shared.MediaConfig.StoragePath 一致，防止调用方传入错误的危险路径；
//   - rootPath 不能是根目录 `/` 或 Windows 盘符根 `C:\`。
func (s *service) Clear(ctx context.Context, rootPath string) error {
	validatedRoot, err := validateMediaStorageRoot(rootPath)
	if err != nil {
		ctx.Error("拒绝清理非法媒体根路径", zap.String("path", rootPath), zap.Error(err))

		return err
	}

	rootPath = validatedRoot

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		ctx.Error("读取目录失败", zap.String("path", rootPath), zap.Error(err))

		return err
	}

	var failedCount int

	for _, entry := range entries {
		target := filepath.Join(rootPath, entry.Name())
		if err := os.RemoveAll(target); err != nil {
			ctx.Warn("删除文件失败，已跳过", zap.Error(err), zap.String("path", target))

			failedCount++
		}
	}

	if failedCount > 0 {
		ctx.Warn("部分磁盘文件清理失败，将继续清理 DB 记录", zap.Int("failed_count", failedCount))
	}

	// 再删除数据库中的所有记录（即便部分磁盘清理失败也要保持 DB 一致）
	if err := s.getDB(ctx).Where("1 = 1").Delete(new(models.MediaFile)).Error; err != nil {
		ctx.Error("清空数据库失败", zap.Error(err))

		return err
	}

	ctx.Info("清空媒体文件数据成功", zap.String("rootPath", rootPath), zap.Int("disk_failed", failedCount))

	return nil
}

// validateMediaStorageRoot 校验媒体根路径，并返回规范化后的绝对路径。
func validateMediaStorageRoot(rootPath string) (string, error) {
	clean := strings.TrimSpace(rootPath)
	if clean == "" {
		return "", errors.New("媒体根路径不能为空")
	}

	absPath, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("解析路径失败: %w", err)
	}

	// 防止 `/` 或 盘符根
	if absPath == "/" || absPath == `\` || (len(absPath) == 3 && absPath[1] == ':') {
		return "", errors.New("拒绝清理系统根目录或盘符根")
	}

	// 必须与当前配置的 StoragePath 一致（校准从 http handler 传来的值）
	cfg := shared.MediaConfig
	if cfg == nil || strings.TrimSpace(cfg.StoragePath) == "" {
		return "", errors.New("媒体存储路径未配置")
	}

	absCfg, err := filepath.Abs(strings.TrimSpace(cfg.StoragePath))
	if err != nil {
		return "", fmt.Errorf("解析配置路径失败: %w", err)
	}

	if absCfg == "/" || absCfg == `\` || (len(absCfg) == 3 && absCfg[1] == ':') {
		return "", errors.New("拒绝使用系统根目录或盘符根作为媒体配置")
	}

	if absCfg != absPath {
		return "", fmt.Errorf("传入路径与媒体配置不一致: got=%s, want=%s", absPath, absCfg)
	}

	return absPath, nil
}

// ClearAll 清空所有媒体文件的 DB 记录（不清除实际文件）。
// 适用于场景：外部手动维护磁盘文件，只需重置 DB 登记。
func (s *service) ClearAll(ctx context.Context) error {
	result := s.getDB(ctx).Unscoped().Where("1 = 1").Delete(new(models.MediaFile))
	if result.Error != nil {
		ctx.Error("清空 media_files 失败", zap.Error(result.Error))

		return result.Error
	}

	ctx.Info("清空 media_files 成功", zap.Int64("deleted", result.RowsAffected))

	return nil
}
