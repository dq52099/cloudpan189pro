package virtualfile

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func (s *service) ClearUnusedAncestorFolder(ctx context.Context, subId int64) error {
	// 定位当前节点与其父节点
	cur, err := s.Query(ctx, subId)
	if err != nil {
		return err
	}

	childId := cur.ID
	parentId := cur.ParentId

	// 向上迭代清理空祖先目录
	for parentId != 0 {
		// 删除前先查询父节点以获取祖父ID，避免删除后再查已删除记录
		parentFile, err := s.Query(ctx, parentId)
		if err != nil {
			return err
		}

		if !parentFile.IsDir || parentFile.IsTop || parentFile.OsType != models.OsTypeFolder {
			break
		}

		grandParentId := parentFile.ParentId

		// 统计父节点在排除当前 childId 后的剩余子节点数量
		count, err := s.Count(ctx, &ListRequest{
			ParentId:      &parentId,
			ExcludeIdList: []int64{childId},
		})
		if err != nil {
			return err
		}

		// 当父节点无其它子项时，删除父节点并继续向上推进
		if count == 0 {
			if err := s.Delete(ctx, parentId); err != nil {
				return err
			}

			// 将刚删除的父节点作为“刚被删除的子节点”，继续检查其父节点
			childId = parentId
			parentId = grandParentId

			continue
		}

		// 父节点仍有其它子项，停止清理
		break
	}

	return nil
}
