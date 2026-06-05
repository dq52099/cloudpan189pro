package virtualfile

import (
	"strings"

	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxVirtualFilePageSize = 500
)

var allowedVirtualFileSortColumns = map[string]struct{}{
	"id":          {},
	"parent_id":   {},
	"top_id":      {},
	"name":        {},
	"is_dir":      {},
	"is_top":      {},
	"size":        {},
	"rev":         {},
	"os_type":     {},
	"cloud_id":    {},
	"create_date": {},
	"modify_date": {},
	"created_at":  {},
	"updated_at":  {},
}

type ListRequest struct {
	ParentId *int64 `form:"parentId" binding:"omitempty,min=1"`
	TopId    *int64 `form:"topId" binding:"omitempty,min=1"`
	LinkId   int64  `form:"-"`
	IsFolder *int8  `form:"-"`
	IsTop    *int8  `form:"-"`

	CurrentPage int    `form:"currentPage" binding:"omitempty,min=1"`
	PageSize    int    `form:"pageSize" binding:"omitempty,min=1"`
	Name        string `form:"name" binding:"omitempty"`

	// ExcludeIdList 排除ID
	ExcludeIdList []int64 `form:"-"`
	TopIdList     []int64 `form:"-"`
	// AllowedSuffixes 限制非目录文件后缀；目录始终保留。
	AllowedSuffixes []string `form:"-"`

	AscList  []string `form:"-"`
	DescList []string `form:"-"`
}

func (r *ListRequest) WithIsTop(isTops ...bool) *ListRequest {
	var (
		zero int8 = 0
		one  int8 = 1
	)

	if len(isTops) > 0 && isTops[0] {
		r.IsTop = &one
	} else {
		r.IsTop = &zero
	}

	return r
}

func (s *service) List(ctx context.Context, req *ListRequest) ([]*models.VirtualFile, error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		ctx.Error("构建文件列表查询失败", zap.Error(err))

		return nil, err
	}

	for _, k := range req.AscList {
		if _, ok := allowedVirtualFileSortColumns[k]; !ok {
			return nil, errInvalidVirtualFileSortField
		}

		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: k}, Desc: false})
	}

	for _, k := range req.DescList {
		if _, ok := allowedVirtualFileSortColumns[k]; !ok {
			return nil, errInvalidVirtualFileSortField
		}

		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: k}, Desc: true})
	}

	if shouldPaginateVirtualFileList(req) {
		normalizeVirtualFilePagination(req)
		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	list := make([]*models.VirtualFile, 0)

	return list, query.Find(&list).Error
}

func (s *service) Count(ctx context.Context, req *ListRequest) (count int64, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		ctx.Error("构建文件数量查询失败", zap.Error(err))

		return 0, err
	}

	if err = query.Count(&count).Error; err != nil {
		ctx.Error("查询文件数量失败", zap.Error(err))
	}

	return count, err
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) (*gorm.DB, error) {
	query := s.getDB(ctx)

	if req.ParentId != nil {
		if *req.ParentId < 0 {
			return nil, errInvalidVirtualFileID
		}

		query = query.Where("parent_id = ?", *req.ParentId)
	}

	if req.LinkId != 0 {
		return nil, errUnsupportedVirtualFileLinkID
	}

	if req.IsFolder != nil {
		query = query.Where("is_dir = ?", *req.IsFolder)
	}

	if req.IsTop != nil {
		query = query.Where("is_top = ?", *req.IsTop)
	}

	if req.Name != "" {
		query = query.Where("name like ?", "%"+req.Name+"%")
	}

	if req.TopId != nil {
		if *req.TopId <= 0 {
			return nil, errInvalidVirtualFileID
		}

		query = query.Where("top_id = ?", *req.TopId)
	}

	if len(req.ExcludeIdList) > 0 {
		excludeIds, err := normalizeVirtualFileIDs(req.ExcludeIdList)
		if err != nil {
			return nil, err
		}

		query = query.Where("id not in (?)", excludeIds)
	}

	if req.TopIdList != nil {
		topIds, err := normalizeVirtualFileIDs(req.TopIdList)
		if err != nil {
			return nil, err
		}

		if len(topIds) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("top_id in (?)", topIds)
		}
	}

	if req.AllowedSuffixes != nil {
		query = applyAllowedSuffixFilter(query, req.AllowedSuffixes)
	}

	return query, nil
}

func applyAllowedSuffixFilter(query *gorm.DB, suffixes []string) *gorm.DB {
	normalizedSuffixes := models.NormalizeSuffixes(suffixes)
	if len(normalizedSuffixes) == 0 {
		return query.Where("is_dir = ?", true)
	}

	conditions := make([]string, 0, len(normalizedSuffixes)+1)
	args := make([]interface{}, 0, len(normalizedSuffixes)+1)

	conditions = append(conditions, "is_dir = ?")
	args = append(args, true)

	for _, suffix := range normalizedSuffixes {
		conditions = append(conditions, "LOWER(name) LIKE ?")
		args = append(args, "%"+suffix)
	}

	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

func shouldPaginateVirtualFileList(req *ListRequest) bool {
	return req.CurrentPage > 0 || req.PageSize > 0
}

func normalizeVirtualFilePagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = 1
	}

	if req.PageSize <= 0 {
		req.PageSize = maxVirtualFilePageSize
	}

	if req.PageSize > maxVirtualFilePageSize {
		req.PageSize = maxVirtualFilePageSize
	}
}

func normalizeVirtualFileIDs(ids []int64) ([]int64, error) {
	for _, id := range ids {
		if id <= 0 {
			return nil, errInvalidVirtualFileID
		}
	}

	return lo.Uniq(ids), nil
}
