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
	Media struct {
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

func (h *handler) Summary() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		resp := &SummaryResponse{}

		h.db.Model(&models.User{}).Count(&resp.Users.Total)
		h.db.Model(&models.User{}).Where("status = ?", 1).Count(&resp.Users.Active)
		h.db.Model(&models.User{}).Where("status = ?", 0).Count(&resp.Users.Disabled)

		h.db.Model(&models.UserGroup{}).Count(&resp.UserGroups)

		h.db.Model(&models.MountPoint{}).Count(&resp.MountPoints.Total)
		h.db.Model(&models.MountPoint{}).Where("enable = ?", true).Count(&resp.MountPoints.Enabled)
		h.db.Model(&models.MountPoint{}).Where("enable_auto_refresh = ?", true).Count(&resp.MountPoints.AutoRefresh)

		h.db.Model(&models.CloudToken{}).Count(&resp.CloudTokens.Total)
		h.db.Model(&models.CloudToken{}).Where("enable = ?", true).Count(&resp.CloudTokens.Active)

		if h.mediaConfigService != nil {
			config, err := h.mediaConfigService.Query(ctx.GetContext())
			if err == nil && config != nil {
				resp.Media.Enabled = config.Enable
				if config.Enable {
					h.db.Model(&models.MediaFile{}).Where("is_str = ?", true).Count(&resp.Media.StrmFiles)
					h.db.Model(&models.MediaFile{}).Count(&resp.Media.MediaFiles)
				}
			}
		}

		h.db.Model(&models.AutoIngestPlan{}).Count(&resp.AutoIngest.Plans)
		h.db.Model(&models.AutoIngestLog{}).Where("created_at > ?", time.Now().AddDate(0, 0, -1)).Count(&resp.AutoIngest.Logs24h)

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
