package usergroup

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"
)

type batchBindFilesRequest struct {
	GroupID int64   `json:"groupId" binding:"required,min=1" example:"1"`        // 用户组ID
	FileIDs []int64 `json:"fileIds" binding:"required" example:"1001,1002,1003"` // 文件ID列表，空数组表示清空绑定
}

// BatchBindFiles 批量绑定文件到用户组
// @Summary 批量绑定文件到用户组
// @Description 批量绑定文件到指定用户组，会先删除该组的所有旧文件绑定关系，然后创建新的绑定关系。支持文件ID去重处理；fileIds 为空数组时清空绑定
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body batchBindFilesRequest true "用户组ID和文件ID列表"
// @Success 200 {object} httpcontext.Response "绑定成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "批量绑定文件失败，code=3004"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "用户组不存在"
// @Router /api/user_group/batch_bind_files [post]
func (h *handler) BatchBindFiles() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(batchBindFilesRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if _, err := h.userGroupService.Query(ctx.GetContext(), req.GroupID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserGroupNotFound.WithError(err))

				return
			}

			ctx.Fail(codeBatchBindFilesFailed.WithError(err))

			return
		}

		if err := h.group2FileService.BatchBindFiles(ctx.GetContext(), req.GroupID, req.FileIDs); err != nil {
			ctx.Fail(codeBatchBindFilesFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
