package resource

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	loginlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/loginlog"
	mediaconfigSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	usergroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Handler interface {
	Summary() httpcontext.HandlerFunc
}

type handler struct {
	db                 *gorm.DB
	logger             *zap.Logger
	userService        userSvi.Service
	userGroupService   usergroupSvi.Service
	mountPointService  mountpointSvi.Service
	cloudTokenService  cloudtokenSvi.Service
	mediaConfigService mediaconfigSvi.Service
	mediaFileService   mediafileSvi.Service
	loginLogService    loginlogSvi.Service
	taskEngine         taskengine.TaskEngine
}

func NewHandler(
	db *gorm.DB,
	logger *zap.Logger,
	userService userSvi.Service,
	userGroupService usergroupSvi.Service,
	mountPointService mountpointSvi.Service,
	cloudTokenService cloudtokenSvi.Service,
	mediaConfigService mediaconfigSvi.Service,
	mediaFileService mediafileSvi.Service,
	loginLogService loginlogSvi.Service,
	taskEngine taskengine.TaskEngine,
) Handler {
	return &handler{
		db:                 db,
		logger:             logger,
		userService:        userService,
		userGroupService:   userGroupService,
		mountPointService:  mountPointService,
		cloudTokenService:  cloudTokenService,
		mediaConfigService: mediaConfigService,
		mediaFileService:   mediaFileService,
		loginLogService:    loginLogService,
		taskEngine:         taskEngine,
	}
}

type SummaryResponse struct {
	Users struct {
		Total    int64 `json:"total"`
		Active   int64 `json:"active"`
		Disabled int64 `json:"disabled"`
	} `json:"users"`
	UserGroups  int64 `json:"userGroups"`
	MountPoints struct {
		Total       int64 `json:"total"`
		Enabled     int64 `json:"enabled"`
		AutoRefresh int64 `json:"autoRefresh"`
	} `json:"mountPoints"`
	CloudTokens struct {
		Total  int64 `json:"total"`
		Active int64 `json:"active"`
	} `json:"cloudTokens"`
	VirtualFiles struct {
		Folders int64 `json:"folders"`
		Files   int64 `json:"files"`
	} `json:"virtualFiles"`
	SubscribeShares int64 `json:"subscribeShares"`
	Media           struct {
		Enabled    bool  `json:"enabled"`
		StrmFiles  int64 `json:"strmFiles"`
		MediaFiles int64 `json:"mediaFiles"`
	} `json:"media"`
	AutoIngest struct {
		Plans   int64 `json:"plans"`
		Logs24h int64 `json:"logs24h"`
	} `json:"autoIngest"`
	Tasks struct {
		Pending   int64 `json:"pending"`
		Running   int64 `json:"running"`
		Failed    int64 `json:"failed"`
		Completed int64 `json:"completed"`
	} `json:"tasks"`
}

// Summary 获取资源概览
// @Summary 获取资源概览
// @Description 获取用户、挂载点、云盘令牌、虚拟文件、媒体、自动入库和任务状态的汇总统计
// @Tags 资源概览
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=SummaryResponse} "获取成功"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/resource/summary [get]
func (h *handler) Summary() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		resp := &SummaryResponse{}

		// 用户统计
		resp.Users.Total = h.countWithLog(&models.User{}, "", nil, "users_total")
		resp.Users.Active = h.countWithLog(&models.User{}, "status = ?", []interface{}{1}, "users_active")
		resp.Users.Disabled = h.countWithLog(&models.User{}, "status = ?", []interface{}{2}, "users_disabled")

		resp.UserGroups = h.countWithLog(&models.UserGroup{}, "", nil, "user_groups")

		// 挂载点：MountPoint 模型没有 enable 字段，Enabled 等同于 Total
		resp.MountPoints.Total = h.countWithLog(&models.MountPoint{}, "", nil, "mount_points_total")
		resp.MountPoints.Enabled = resp.MountPoints.Total
		resp.MountPoints.AutoRefresh = h.countWithLog(&models.MountPoint{}, "enable_auto_refresh = ?", []interface{}{true}, "mount_points_auto_refresh")

		// 云盘令牌
		resp.CloudTokens.Total = h.countWithLog(&models.CloudToken{}, "", nil, "cloud_tokens_total")
		resp.CloudTokens.Active = h.countWithLog(&models.CloudToken{}, "status = ?", []interface{}{1}, "cloud_tokens_active")

		// 虚拟文件
		resp.VirtualFiles.Folders = h.countWithLog(&models.VirtualFile{}, "is_dir = ?", []interface{}{true}, "virtual_files_folders")
		resp.VirtualFiles.Files = h.countWithLog(&models.VirtualFile{}, "is_dir = ?", []interface{}{false}, "virtual_files_files")

		// 订阅分享：统计所有订阅类型挂载点
		resp.SubscribeShares = h.countWithLog(
			&models.MountPoint{},
			"os_type IN ?",
			[]interface{}{[]string{
				string(models.OsTypeSubscribe),
				string(models.OsTypeSubscribeShareFolder),
				string(models.OsTypeSubscribeShareFile),
			}},
			"subscribe_shares",
		)

		// 媒体
		if h.mediaConfigService != nil {
			config, err := h.mediaConfigService.Query(ctx.GetContext())
			if err != nil {
				h.logger.Warn("查询媒体配置失败", zap.Error(err))
			} else if config != nil {
				resp.Media.Enabled = config.Enable
				if config.Enable {
					resp.Media.StrmFiles = h.countWithLog(
						&models.MediaFile{},
						"media_type = ?",
						[]interface{}{string(media.TypeStrm)},
						"media_strm",
					)
					resp.Media.MediaFiles = h.countWithLog(&models.MediaFile{}, "", nil, "media_total")
				}
			}
		}

		// 自动入库
		resp.AutoIngest.Plans = h.countWithLog(&models.AutoIngestPlan{}, "", nil, "auto_ingest_plans")
		resp.AutoIngest.Logs24h = h.countWithLog(
			&models.AutoIngestLog{},
			"created_at > ?",
			[]interface{}{time.Now().AddDate(0, 0, -1)},
			"auto_ingest_logs_24h",
		)

		// 任务引擎状态
		if h.taskEngine != nil {
			stats := h.taskEngine.GetStats()
			resp.Tasks.Pending = stats.PendingTasks
			resp.Tasks.Running = stats.RunningTasks
			resp.Tasks.Failed = stats.FailedTasks
			resp.Tasks.Completed = stats.CompletedTasks
		}

		ctx.Success(resp)
	}
}

// countWithLog 统计记录数，失败时记录日志但不中断（资源概览允许部分失败）。
func (h *handler) countWithLog(model interface{}, where string, args []interface{}, label string) int64 {
	var count int64

	q := h.db.Model(model)
	if where != "" {
		q = q.Where(where, args...)
	}

	if err := q.Count(&count).Error; err != nil {
		h.logger.Warn("资源概览统计失败", zap.String("label", label), zap.Error(err))

		return 0
	}

	return count
}
