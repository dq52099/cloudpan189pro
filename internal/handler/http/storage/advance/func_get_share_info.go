package advance

import (
	"regexp"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

type getShareInfoRequest struct {
	ShareCode       string `form:"shareCode" binding:"required"`
	ShareAccessCode string `form:"shareAccessCode"`
}

var reShareLink = regexp.MustCompile(`cloud\.189\.cn\/t\/([a-zA-Z0-9]+)`)
var reAccessCode = regexp.MustCompile(`(?:\S+码|code)[:：]\s*([a-zA-Z0-9]+)`)

// GetShareInfo 获取分享信息
// @Summary 获取分享信息
// @Description 根据分享码获取分享的详细信息，支持直接传入完整分享链接
// @Tags 存储高级功能
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param shareCode query string true "分享码或完整链接" example("https://cloud.189.cn/t/abc12345")
// @Param shareAccessCode query string false "分享访问码" example("1234")
// @Success 200 {object} httpcontext.Response{data=cloudbridge.ShareInfo} "获取分享信息成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
// @Failure 400 {object} httpcontext.Response "获取分享详情失败"
// @Router /api/storage/advance/share_info [get]
func (h *handler) GetShareInfo() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(getShareInfoRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		cleanCode := strings.ReplaceAll(req.ShareCode, "（", "(")
		cleanCode = strings.ReplaceAll(cleanCode, "）", ")")
		cleanCode = strings.ReplaceAll(cleanCode, "：", ":")
		cleanCode = strings.TrimSpace(cleanCode)

		var pureShareCode, pureAccessCode string

		if req.ShareAccessCode == "" {
			if codeMatch := reAccessCode.FindStringSubmatch(cleanCode); len(codeMatch) > 1 {
				pureAccessCode = codeMatch[1]
			} else {
				parts := strings.Fields(cleanCode)
				if len(parts) > 1 {
					lastPart := strings.Trim(parts[len(parts)-1], "()")
					if len(lastPart) == 4 {
						pureAccessCode = lastPart
					}
				}
			}
		} else {
			pureAccessCode = req.ShareAccessCode
		}

		if matches := reShareLink.FindStringSubmatch(cleanCode); len(matches) > 1 {
			pureShareCode = matches[1]
		} else {
			if idx := strings.Index(cleanCode, "("); idx > -1 {
				pureShareCode = strings.TrimSpace(cleanCode[:idx])
			} else {
				parts := strings.Fields(cleanCode)
				if len(parts) > 0 {
					pureShareCode = strings.Trim(parts[0], "()")
				}
			}
		}

		shareInfo, err := h.cloudBridgeService.GetShareInfo(ctx.GetContext(), pureShareCode, pureAccessCode)
		if err != nil {
			ctx.Fail(codeStorageAdvanceGetShareInfoError.WithError(err))
			return
		}

		if shareInfo == nil {
			ctx.Fail(codeStorageAdvanceGetShareInfoError.WithMessage("未查询到分享信息"))
			return
		}

		ctx.Success(shareInfo)
	}
}
