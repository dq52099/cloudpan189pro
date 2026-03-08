package setting

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	settingSvi "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
)

type Handler interface {
	InitSystem() httpcontext.HandlerFunc
	Info() httpcontext.HandlerFunc
	ModifyTitle() httpcontext.HandlerFunc
	ModifyBaseURL() httpcontext.HandlerFunc
	ToggleEnableAuth() httpcontext.HandlerFunc
	ModifyAddition() httpcontext.HandlerFunc
	Addition() httpcontext.HandlerFunc
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeSettingStartCode)

var (
	codeInitSettingErr         = bi.Next("初始化系统时发生错误")
	codeInitSuperUserErr       = bi.Next("初始化超级管理员时发生错误")
	codeQueryFailed            = bi.Next("查询系统配置失败")
	codeModifyTitleFailed      = bi.Next("更新系统标题失败")
	codeModifyBaseURLFailed    = bi.Next("更新系统基础URL失败")
	codeToggleEnableAuthFailed = bi.Next("更新系统鉴权开关失败")
	codeModifyAdditionFailed   = bi.Next("更新系统附加设置失败")
)

type handler struct {
	userService    userSvi.Service
	settingService settingSvi.Service
	taskEngine     taskengine.TaskEngine
	initTime       time.Time
}

func NewHandler(userService userSvi.Service, settingService settingSvi.Service, taskEngine taskengine.TaskEngine) Handler {
	return &handler{
		userService:    userService,
		settingService: settingService,
		taskEngine:     taskEngine,
		initTime:       time.Now(),
	}
}
