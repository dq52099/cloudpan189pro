package taskstate

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
)

type Handler interface {
	FileLogList() httpcontext.HandlerFunc
	TaskEngineList() httpcontext.HandlerFunc
	ClearTaskLogs() httpcontext.HandlerFunc
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeTaskStateStartCode)

var (
	codeGetTaskEngineStatsFailed = bi.Next("获取任务引擎状态失败")
	codeListTasksFailed          = bi.Next("任务列表获取失败")
	codeCountTasksFailed         = bi.Next("任务数量统计失败")
)

type handler struct {
	taskEngine         taskengine.TaskEngine
	fileTaskLogService filetasklogSvi.Service
}

func NewHandler(taskEngine taskengine.TaskEngine, fileTaskLogService filetasklogSvi.Service) Handler {
	return &handler{
		taskEngine:         taskEngine,
		fileTaskLogService: fileTaskLogService,
	}
}
