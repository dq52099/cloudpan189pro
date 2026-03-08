package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
)

// 使用指针以便区分“未提供”和“提供零值”的场景
type modifyAdditionRequest struct {
	LocalProxy                *bool  `json:"localProxy" example:"false"`                                                // 是否启用本地代理（可选）
	MultipleStream            *bool  `json:"multipleStream" example:"true"`                                             // 是否启用多线程分流（可选）
	MultipleStreamThreadCount *int   `json:"multipleStreamThreadCount" binding:"omitempty,min=1,max=64" example:"4"`    // 多线程数量（可选）
	MultipleStreamChunkSize   *int64 `json:"multipleStreamChunkSize" binding:"omitempty,min=1048576" example:"4194304"` // 分片大小，单位字节（可选，>=1MiB）
	TaskThreadCount           *int   `json:"taskThreadCount" binding:"omitempty,min=1,max=32" example:"1"`              // 任务线程数量（可选）
	WorkerCount               *int   `json:"workerCount" binding:"omitempty,min=1,max=32" example:"5"`                  // 工作流数量（可选）
	EnableStorageAutoRefresh  *bool  `json:"enableStorageAutoRefresh" example:"true"`                                   // 是否启用存储自动刷新（可选）
}

// ModifyAddition 修改系统附加设置（可选字段更新）
// @Summary 修改系统附加设置
// @Description 更新系统的 SettingAddition（可选字段更新），仅管理员可操作
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param request body modifyAdditionRequest true "系统附加设置（仅填写需要修改的字段）"
// @Success 200 {object} httpcontext.Response "系统附加设置更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "更新系统附加设置失败，code=6006"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/setting/modify_addition [post]
func (h *handler) ModifyAddition() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyAdditionRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 查询当前设置以进行合并更新
		current, err := h.settingService.Query(ctx.GetContext())
		if err != nil {
			ctx.Fail(codeQueryFailed.WithError(err))

			return
		}

		merged := current.Addition

		// 仅对提供的字段进行覆盖
		if req.LocalProxy != nil {
			merged.LocalProxy = *req.LocalProxy
		}

		if req.MultipleStream != nil {
			merged.MultipleStream = *req.MultipleStream
		}

		if req.MultipleStreamThreadCount != nil {
			merged.MultipleStreamThreadCount = *req.MultipleStreamThreadCount
		}

		if req.MultipleStreamChunkSize != nil {
			merged.MultipleStreamChunkSize = *req.MultipleStreamChunkSize
		}

		if req.TaskThreadCount != nil {
			merged.TaskThreadCount = *req.TaskThreadCount
		}

		if req.WorkerCount != nil {
			merged.WorkerCount = *req.WorkerCount
		}

		if req.EnableStorageAutoRefresh != nil {
			merged.EnableStorageAutoRefresh = *req.EnableStorageAutoRefresh
		}

		// 更新数据库
		if err := h.settingService.Update(ctx.GetContext(),
			utils.WithField("addition", merged),
		); err != nil {
			ctx.Fail(codeModifyAdditionFailed.WithError(err))

			return
		}

		// 同步内存态
		shared.SettingAddition = models.SettingAddition{
			LocalProxy:                merged.LocalProxy,
			MultipleStream:            merged.MultipleStream,
			MultipleStreamThreadCount: merged.MultipleStreamThreadCount,
			MultipleStreamChunkSize:   merged.MultipleStreamChunkSize,
			TaskThreadCount:           merged.TaskThreadCount,
			WorkerCount:               merged.WorkerCount,
			EnableStorageAutoRefresh:  merged.EnableStorageAutoRefresh,
		}

		ctx.Success()
	}
}
