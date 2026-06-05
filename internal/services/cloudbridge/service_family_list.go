package cloudbridge

import (
	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

func (s *service) FamilyList(ctx context.Context, token client.AuthToken) (*GetFamilyListResponse, error) {
	resp, err := client.New().
		WithClient(ctx.HTTPClient()).
		WithToken(token).
		GetFamilyList(ctx)
	if err != nil {
		return nil, logCloudbridgeError(ctx, "获取家庭云列表失败", err)
	}

	return resp, nil
}
