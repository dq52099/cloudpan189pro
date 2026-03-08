package mountpoint

import "github.com/xxcheng123/cloudpan189-share/internal/framework/context"

func (s *service) ModifyToken(ctx context.Context, id int64, tokenId int64) error {
	return s.getDB(ctx).Where("id = ?", id).Update("token_id", tokenId).Error
}
