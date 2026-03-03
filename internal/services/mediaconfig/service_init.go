package mediaconfig

import (
	"github.com/pkg/errors"

	"go.uber.org/zap"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
)

// InitRequest 初始化/更新媒体配置请求
type InitRequest struct {
	Enable              bool
	StoragePath         string
	AutoClean           bool
	ConflictPolicy      media.FileConflictPolicy
	BaseURL             string
	IncludedSuffixes    []string
	AutoRebuildEnable   bool
	AutoRebuildInterval int
}

var (
	storageDisableAllowedEmpty = errors.New("媒体文件根路径不能为空")
	baseURLEmptyErr            = errors.New("BaseURL 不能为空")
	alreadyInitializedErr      = errors.New("媒体配置已经初始化过了")
)

// Init 初始化媒体配置：

func (s *service) Init(ctx context.Context, req *InitRequest) error {
	if req.StoragePath == "" {
		return storageDisableAllowedEmpty
	}

	if req.BaseURL == "" {
		return baseURLEmptyErr
	}

	var cnt int64

	if err := s.getDB(ctx).Count(&cnt).Error; err != nil {
		ctx.Error("媒体配置计数失败", zap.Error(err))

		return err
	}

	if cnt > 0 {
		return alreadyInitializedErr
	}

	if req.ConflictPolicy == "" {
		req.ConflictPolicy = media.FileConflictPolicySkip
	}

	if len(req.IncludedSuffixes) == 0 {
		req.IncludedSuffixes = []string{}
	}

	if req.AutoRebuildInterval <= 0 {
		req.AutoRebuildInterval = 24
	}

	newCfg := &models.MediaConfig{
		Enable:              req.Enable,
		StoragePath:         req.StoragePath,
		AutoClean:           req.AutoClean,
		ConflictPolicy:      req.ConflictPolicy,
		BaseURL:             req.BaseURL,
		IncludedSuffixes:    req.IncludedSuffixes,
		AutoRebuildEnable:   req.AutoRebuildEnable,
		AutoRebuildInterval: req.AutoRebuildInterval,
	}

	if createErr := s.svc.GetDB(ctx).Create(newCfg).Error; createErr != nil {
		ctx.Error("媒体配置创建失败", zap.Error(createErr))

		return createErr
	}

	return nil
}
