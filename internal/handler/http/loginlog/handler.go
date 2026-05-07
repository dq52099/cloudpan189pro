package loginlog

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	loginlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/loginlog"
)

// Handler 定义 login 日志相关的 HTTP 处理器接口
type Handler interface {
	// List 登录日志列表
	List() httpcontext.HandlerFunc
	// Clear 清空登录日志
	Clear() httpcontext.HandlerFunc
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeLoginLogStartCode)

var (
	codeListFailed  = bi.Next("获取登录日志列表失败")
	codeClearFailed = bi.Next("清空登录日志失败")
)

type handler struct {
	loginLogService loginlogSvi.Service
}

func NewHandler(loginLogService loginlogSvi.Service) Handler {
	return &handler{
		loginLogService: loginLogService,
	}
}
