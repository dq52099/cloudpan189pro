package storage

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

// BatchParseFromText 批量解析文本
// @Summary 批量解析文本（预览）
// @Description 解析文本内容（如分享链接），验证CloudToken，返回资源的真实名称和ID，但不创建挂载
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body topic.BatchParseTextRequest true "解析请求参数"
// @Success 200 {object} httpcontext.Response "解析成功，data为解析结果列表"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "CloudToken不存在"
// @Failure 400 {object} httpcontext.Response "解析服务内部错误"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/storage/batch_parse_text [post]
func (h *handler) BatchParseFromText() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		// 使用 topic 包中定义的请求结构
		req := new(topic.BatchParseTextRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		// 校验 CloudToken 是否存在
		tokenInfo, err := h.cloudTokenService.Query(ctx.GetContext(), req.CloudToken)
		if err != nil || tokenInfo == nil {
			ctx.Fail(busCodeStorageCloudTokenNotExist)
			return
		}

		// 调用 Service 层进行解析 (核心逻辑在 Service 中)
		result, err := h.mountPointService.BatchParseText(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(busCodeStorageQueryPathFailed.WithError(err))
			return
		}

		// 返回解析结果列表
		ctx.Success(result)
	}
}
