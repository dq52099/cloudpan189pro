package media

import (
	"net/http"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mediaconfigSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

// Handler 定义 media 相关的 HTTP 处理器接口
type Handler interface {
	// 媒体配置
	ConfigInit() httpcontext.HandlerFunc
	ConfigInfo() httpcontext.HandlerFunc
	ConfigUpdate() httpcontext.HandlerFunc
	ConfigToggle() httpcontext.HandlerFunc

	// 媒体操作
	Clear() httpcontext.HandlerFunc
	RebuildStrmFile() httpcontext.HandlerFunc
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeMediaStartCode)

var (
	// 配置相关错误码
	codeConfigQueryFailed  = bi.Next("查询媒体配置失败")
	codeConfigInitFailed   = bi.Next("初始化媒体配置失败")
	codeConfigUpdateFailed = bi.Next("更新媒体配置失败")
	codeConfigToggleFailed = bi.Next("切换媒体配置启用状态失败")
	codeConfigNotInit      = bi.Next("媒体配置未初始化").WithHTTPCode(http.StatusNotFound)

	// 操作相关错误码
	codeMediaNotEnabled           = bi.Next("媒体功能未启用")
	codeClearFailed               = bi.Next("清理媒体文件失败")
	codeRebuildFailed             = bi.Next("重建strm文件失败")
	codeRebuildMountPointNotFound = bi.Next("指定挂载点不存在或无权限").WithHTTPCode(http.StatusNotFound)
)

// handler 依赖的服务
type handler struct {
	mediaConfigService mediaconfigSvi.Service
	mediaFileService   mediafileSvi.Service
	mountpointService  mountpointSvi.Service
	virtualfileService virtualfileSvi.Service
	verifyService      verifySvi.Service
	fileTaskLogService filetasklogSvi.Service
	taskEngine         taskengine.TaskEngine
}

// NewHandler 构造函数
func NewHandler(
	mediaConfigService mediaconfigSvi.Service,
	mediaFileService mediafileSvi.Service,
	mountpointService mountpointSvi.Service,
	virtualfileService virtualfileSvi.Service,
	verifyService verifySvi.Service,
	fileTaskLogService filetasklogSvi.Service,
	taskEngine taskengine.TaskEngine,
) Handler {
	return &handler{
		mediaConfigService: mediaConfigService,
		mediaFileService:   mediaFileService,
		mountpointService:  mountpointService,
		virtualfileService: virtualfileService,
		verifyService:      verifyService,
		fileTaskLogService: fileTaskLogService,
		taskEngine:         taskEngine,
	}
}
