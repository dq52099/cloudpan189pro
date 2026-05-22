package usergroup

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"
)

type getBindFilesRequest struct {
	GroupId int64 `form:"groupId" binding:"required,min=1" example:"1"` // 用户组ID
}

type getBindFilesResponse struct {
	FileIds []int64 `json:"fileIds" example:"1001,1002,1003"` // 文件ID列表
}

// GetBindFiles 获取用户组绑定的文件列表
// @Summary 获取用户组绑定的文件列表
// @Description 获取指定用户组绑定的所有文件信息，包括文件ID、文件名等详细信息
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param groupId query int true "用户组ID" minimum(1)
// @Success 200 {object} httpcontext.Response{data=getBindFilesResponse} "获取绑定文件成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "获取绑定文件失败，code=3005"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "用户组不存在"
// @Router /api/user_group/bind_files [get]
func (h *handler) GetBindFiles() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(getBindFilesRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if _, err := h.userGroupService.Query(ctx.GetContext(), req.GroupId); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserGroupNotFound.WithError(err))

				return
			}

			ctx.Fail(codeGetBindFilesFailed.WithError(err))

			return
		}

		fileIds, err := h.group2FileService.GetBindFiles(ctx.GetContext(), req.GroupId)
		if err != nil {
			ctx.Fail(codeGetBindFilesFailed.WithError(err))

			return
		}

		resp := &getBindFilesResponse{
			FileIds: fileIds,
		}

		ctx.Success(resp)
	}
}
