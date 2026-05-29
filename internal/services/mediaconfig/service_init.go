package mediaconfig

import (
	"github.com/pkg/errors"

	"go.uber.org/zap"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"gorm.io/gorm"
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
	AutoRebuildCron     string
}

var (
	storageDisableAllowedEmpty = errors.New("媒体文件根路径不能为空")
	baseURLEmptyErr            = errors.New("BaseURL 不能为空")
	alreadyInitializedErr      = errors.New("媒体配置已经初始化过了")
)

// Init 初始化媒体配置：

func (s *service) Init(ctx context.Context, req *InitRequest) error {
	if req == nil {
		return storageDisableAllowedEmpty
	}

	if req.StoragePath == "" {
		return storageDisableAllowedEmpty
	}

	if req.BaseURL == "" {
		return baseURLEmptyErr
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

	if req.AutoRebuildCron == "" {
		req.AutoRebuildCron = "0 2 * * *"
	}

	newCfg := &models.MediaConfig{
		ID:                  1,
		Enable:              req.Enable,
		StoragePath:         req.StoragePath,
		AutoClean:           req.AutoClean,
		ConflictPolicy:      req.ConflictPolicy,
		BaseURL:             req.BaseURL,
		IncludedSuffixes:    req.IncludedSuffixes,
		AutoRebuildEnable:   req.AutoRebuildEnable,
		AutoRebuildInterval: req.AutoRebuildInterval,
		AutoRebuildCron:     req.AutoRebuildCron,
	}

	if createErr := s.svc.GetDB(ctx).Create(newCfg).Error; createErr != nil {
		if isMediaConfigInitialized(s.svc.GetDB(ctx)) {
			return alreadyInitializedErr
		}

		ctx.Error("媒体配置创建失败", zap.Error(createErr))

		return createErr
	}

	shared.MediaConfig = newCfg

	return nil
}

func isMediaConfigInitialized(db *gorm.DB) bool {
	var count int64

	if err := db.Model(new(models.MediaConfig)).Where("id = ?", int64(1)).Count(&count).Error; err != nil {
		return false
	}

	return count > 0
}
