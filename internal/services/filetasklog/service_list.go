package filetasklog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultListCurrentPage = 1
	defaultListPageSize    = 10
	maxListPageSize        = 500
)

var allowedFileTaskLogSortColumns = map[string]struct{}{
	"id":         {},
	"type":       {},
	"status":     {},
	"file_id":    {},
	"user_id":    {},
	"title":      {},
	"begin_at":   {},
	"end_at":     {},
	"duration":   {},
	"total":      {},
	"completed":  {},
	"failed":     {},
	"created_at": {},
	"updated_at": {},
}

type ListRequest struct {
	Type    string    `form:"type" binding:"omitempty"`
	Status  string    `form:"status" binding:"omitempty"`
	FileId  int64     `form:"fileId" binding:"omitempty"`
	UserId  int64     `form:"userId" binding:"omitempty"`
	BeginAt time.Time `form:"beginAt" binding:"omitempty"`
	EndAt   time.Time `form:"endAt" binding:"omitempty"`

	CurrentPage int    `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"` // 当前页码，默认为1
	PageSize    int    `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`  // 每页大小，默认为10
	NoPaginate  bool   `form:"-"`
	Title       string `form:"title" binding:"omitempty"`

	AscList  []string `form:"-"`
	DescList []string `form:"-"`

	FileIdList []int64 `form:"-"`
}

func (s *service) List(ctx context.Context, req *ListRequest) ([]*models.FileTaskLog, error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(req.AscList) > 0 {
		for _, k := range req.AscList {
			if _, ok := allowedFileTaskLogSortColumns[k]; !ok {
				return nil, errInvalidFileTaskLogSortField
			}

			query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: k}, Desc: false})
		}
	} else {
		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true})
	}

	for _, k := range req.DescList {
		if _, ok := allowedFileTaskLogSortColumns[k]; !ok {
			return nil, errInvalidFileTaskLogSortField
		}

		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: k}, Desc: true})
	}

	if !req.NoPaginate {
		normalizeListPagination(req)
		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	list := make([]*models.FileTaskLog, 0)

	return list, query.Find(&list).Error
}

func (s *service) Count(ctx context.Context, req *ListRequest) (count int64, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return 0, err
	}

	err = query.Count(&count).Error

	return count, err
}

// ListLatestFileIDsByStatus 查询最新任务日志状态匹配的文件 ID。
// 同一个 file_id 只看最新一条任务日志，避免旧失败日志影响当前状态筛选。
func (s *service) ListLatestFileIDsByStatus(ctx context.Context, status string) ([]int64, error) {
	if status == "" {
		return []int64{}, nil
	}

	latestLogIDs := s.getDB(ctx).
		Select("MAX(id)").
		Where("file_id > 0").
		Group("file_id")

	fileIDs := make([]int64, 0)
	if err := s.getDB(ctx).
		Where("id IN (?)", latestLogIDs).
		Where("status = ?", status).
		Pluck("file_id", &fileIDs).Error; err != nil {
		ctx.Error("查询最新任务日志状态文件 ID 失败", zap.String("status", status), zap.Error(err))

		return nil, err
	}

	return fileIDs, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) (*gorm.DB, error) {
	query := s.getDB(ctx)

	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}

	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	if req.FileIdList != nil {
		fileIDs, err := normalizeFileTaskLogFileIDs(req.FileIdList)
		if err != nil {
			return nil, err
		}

		if len(fileIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("file_id IN (?)", fileIDs)
		}
	} else if req.FileId > 0 {
		query = query.Where("file_id = ?", req.FileId)
	} else if req.FileId < 0 {
		return nil, errInvalidFileTaskLogFileID
	}

	if req.UserId > 0 {
		query = query.Where("user_id = ?", req.UserId)
	}

	if !req.BeginAt.IsZero() {
		query = query.Where("begin_at >= ?", req.BeginAt)
	}

	if !req.EndAt.IsZero() {
		query = query.Where("end_at <= ?", req.EndAt)
	}

	if req.Title != "" {
		query = query.Where("title like ?", "%"+req.Title+"%")
	}

	return query, nil
}

// FindStaleTasksByDuration 查询未完成的文件任务
func (s *service) FindStaleTasksByDuration(ctx context.Context, duration time.Duration) ([]*models.FileTaskLog, error) {
	cutoffTime := s.getDB(ctx).NowFunc().Add(-duration)

	var list []*models.FileTaskLog
	if err := s.getDB(ctx).
		Where("updated_at < ?", cutoffTime).
		Where("status NOT IN (?)", []string{models.StatusCompleted, models.StatusFailed}).
		Find(&list).Error; err != nil {
		ctx.Error("查询未完成的文件任务失败", zap.Error(err), zap.Duration("duration", duration))

		return nil, err
	}

	return list, nil
}

// FindByFileID 根据文件ID查询相关任务
func (s *service) FindByFileID(ctx context.Context, fileID int64) ([]*models.FileTaskLog, error) {
	if fileID <= 0 {
		return nil, errInvalidFileTaskLogFileID
	}

	var list []*models.FileTaskLog
	if err := s.getDB(ctx).
		Where("file_id = ?", fileID).
		Order("created_at DESC").
		Find(&list).Error; err != nil {
		ctx.Error("查询文件任务日志失败", zap.Error(err), zap.Int64("file_id", fileID))

		return nil, err
	}

	return list, nil
}

func normalizeListPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultListCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultListPageSize
	}

	if req.PageSize > maxListPageSize {
		req.PageSize = maxListPageSize
	}
}

func normalizeFileTaskLogFileIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))

	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errInvalidFileTaskLogFileID
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}
