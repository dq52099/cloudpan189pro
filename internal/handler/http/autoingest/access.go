package autoingest

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func ensurePlanAccess(ctx *httpcontext.Context, plan *models.AutoIngestPlan) bool {
	if plan == nil || plan.ID == 0 {
		ctx.Fail(codePlanNotFound)

		return false
	}

	if ctx.GetBool(consts.CtxKeyIsAdmin) {
		return true
	}

	userID := ctx.GetInt64(consts.CtxKeyUserId)
	if userID <= 0 || plan.UserID != userID {
		ctx.Unauthorized("无权限操作该自动入库计划")

		return false
	}

	return true
}

func filterAccessiblePlans(ctx *httpcontext.Context, requestIDs []int64, plans []*models.AutoIngestPlan) ([]*models.AutoIngestPlan, int) {
	deniedOrMissingCount := len(requestIDs) - len(plans)
	if ctx.GetBool(consts.CtxKeyIsAdmin) {
		return plans, deniedOrMissingCount
	}

	userID := ctx.GetInt64(consts.CtxKeyUserId)

	accessiblePlans := make([]*models.AutoIngestPlan, 0, len(plans))
	for _, plan := range plans {
		if plan != nil && userID > 0 && plan.UserID == userID {
			accessiblePlans = append(accessiblePlans, plan)

			continue
		}

		deniedOrMissingCount++
	}

	return accessiblePlans, deniedOrMissingCount
}
