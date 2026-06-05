package taskstate

import (
	"unicode/utf8"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type (
	taskEngineListResponse struct {
		Stats        taskengine.TaskStats   `json:"stats"`        // 任务引擎统计信息
		RunningTasks []*taskengine.TaskInfo `json:"runningTasks"` // 正在运行的任务列表
		PendingTasks []*taskengine.TaskInfo `json:"pendingTasks"` // 待处理的任务列表
		IsRunning    bool                   `json:"isRunning"`    // 引擎是否正在运行
	}
)

func sanitizeTaskEngineTasks(tasks []*taskengine.TaskInfo) []*taskengine.TaskInfo {
	if len(tasks) == 0 {
		return tasks
	}

	sanitizedTasks := make([]*taskengine.TaskInfo, 0, len(tasks))
	for _, task := range tasks {
		sanitizedTasks = append(sanitizedTasks, sanitizeTaskEngineTask(task))
	}

	return sanitizedTasks
}

func sanitizeTaskEngineTask(task *taskengine.TaskInfo) *taskengine.TaskInfo {
	if task == nil {
		return nil
	}

	sanitizedTask := &taskengine.TaskInfo{
		Payload:   sanitizeTaskPayload(task.Payload),
		ID:        task.ID,
		Topic:     task.Topic,
		WorkerId:  task.WorkerId,
		ReceiveAt: task.ReceiveAt,
		Status:    task.Status,
	}

	if task.StartAt != nil {
		startAt := *task.StartAt
		sanitizedTask.StartAt = &startAt
	}

	if task.EndAt != nil {
		endAt := *task.EndAt
		sanitizedTask.EndAt = &endAt
	}

	if task.Results != nil {
		sanitizedTask.Results = append([]taskengine.ProcessorResult(nil), task.Results...)
		for idx := range sanitizedTask.Results {
			sanitizedTask.Results[idx].Error = sanitizeTaskText(sanitizedTask.Results[idx].Error)
		}
	}

	return sanitizedTask
}

func sanitizeTaskPayload(payload []byte) []byte {
	if payload == nil {
		return nil
	}

	if !utf8.Valid(payload) {
		return []byte(utils.RedactedSecret)
	}

	return []byte(sanitizeTaskText(string(payload)))
}

func sanitizeTaskText(text string) string {
	return utils.RedactSensitiveText(utils.RedactURLsInTextForLog(text))
}

// TaskEngineList 获取任务引擎状态和运行中的任务列表
// @Summary 获取任务引擎状态
// @Description 获取任务引擎的统计信息和当前正在运行的任务列表
// @Tags 任务状态管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=taskEngineListResponse} "获取任务引擎状态成功"
// @Failure 400 {object} httpcontext.Response "获取任务引擎状态失败，code=7003"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/task_state/task_engine/list [get]
func (h *handler) TaskEngineList() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		if !h.ensureTaskEngine(ctx) {
			return
		}

		// 获取任务引擎统计信息
		stats := h.taskEngine.GetStats()

		// 获取正在运行的任务列表
		runningTasks := sanitizeTaskEngineTasks(h.taskEngine.GetRunningTasks())

		// 获取待处理的任务列表
		pendingTasks := sanitizeTaskEngineTasks(h.taskEngine.GetPendingTasks())

		// 检查引擎是否正在运行
		isRunning := h.taskEngine.IsRunning()

		response := &taskEngineListResponse{
			Stats:        stats,
			RunningTasks: runningTasks,
			PendingTasks: pendingTasks,
			IsRunning:    isRunning,
		}

		ctx.Success(response)
	}
}
