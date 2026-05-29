package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

// Enable 启用自动挂载计划
func (s *service) Enable(ctx context.Context, id int64) error {
	return s.Update(ctx, id, utils.Field{Key: "enabled", Value: true})
}

// Disable 停用自动挂载计划
func (s *service) Disable(ctx context.Context, id int64) error {
	return s.Update(ctx, id, utils.Field{Key: "enabled", Value: false})
}

// EnableByOwner 按当前用户权限启用自动挂载计划。
func (s *service) EnableByOwner(ctx context.Context, req *UpdateRequest) error {
	return s.UpdateByOwner(ctx, req, utils.Field{Key: "enabled", Value: true})
}

// DisableByOwner 按当前用户权限停用自动挂载计划。
func (s *service) DisableByOwner(ctx context.Context, req *UpdateRequest) error {
	return s.UpdateByOwner(ctx, req, utils.Field{Key: "enabled", Value: false})
}
