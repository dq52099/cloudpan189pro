package mountpoint

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/ptr"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

type CreateRequest struct {
	FileId        int64  `json:"fileId"`    // 文件ID
	FullPath      string `json:"fullPath" ` // 完整路径
	OsType        string `json:"osType"`    // 操作系统类型
	TokenId       int64  `json:"tokenId"`   // 令牌ID
	CreatorUserID int64  `json:"-"`         // 创建者用户ID

	EnableAutoRefresh bool `json:"enableAutoRefresh"`
	AutoRefreshDays   int  `json:"autoRefreshDays"`
	RefreshInterval   int  `json:"refreshInterval"`
	EnableDeepRefresh bool `json:"enableDeepRefresh"`
}

func (s *service) Create(ctx context.Context, req *CreateRequest) (int64, error) {
	if req == nil || req.FileId <= 0 {
		return 0, errInvalidMountPointFileID
	}

	if req.TokenId < 0 {
		return 0, errInvalidMountPointTokenID
	}

	fullPath, paths, err := utils.NormalizeStoragePathParts(req.FullPath)
	if err != nil {
		return 0, errInvalidMountPointFullPath
	}

	if len(paths) == 0 {
		return 0, errInvalidMountPointFullPath
	}

	name := paths[len(paths)-1]

	if req.RefreshInterval < 30 {
		req.RefreshInterval = 30
	}

	var beginAt *time.Time
	if req.EnableAutoRefresh {
		beginAt = ptr.Of(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location()))
	}

	mountPoint := &models.MountPoint{
		FileId:        req.FileId,
		Name:          name,
		FullPath:      fullPath,
		OsType:        req.OsType,
		TokenId:       req.TokenId,
		CreatorUserID: req.CreatorUserID,

		EnableAutoRefresh:  req.EnableAutoRefresh,
		AutoRefreshDays:    req.AutoRefreshDays,
		RefreshInterval:    req.RefreshInterval,
		EnableDeepRefresh:  req.EnableDeepRefresh,
		AutoRefreshBeginAt: beginAt,
	}

	if err := s.getDB(ctx).Create(mountPoint).Error; err != nil {
		ctx.Error("创建挂载点失败", zap.Error(err), zap.Int64("fileId", req.FileId), zap.String("fullPath", fullPath))

		return 0, err
	}

	return mountPoint.ID, nil
}
