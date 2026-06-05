package filetasklog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

type NewOptionFunc func(log *models.FileTaskLog)

func WithTitle(title string) NewOptionFunc {
	return func(b *models.FileTaskLog) {
		b.Title = title
	}
}

func WithType(typ string) NewOptionFunc {
	return func(b *models.FileTaskLog) {
		b.Type = typ
	}
}

func WithFile(fid int64) NewOptionFunc {
	return func(b *models.FileTaskLog) {
		b.FileId = fid
	}
}

func WithDesc(desc string) NewOptionFunc {
	return func(b *models.FileTaskLog) {
		b.Desc = sanitizeTaskLogText(desc)
	}
}

func WithUser(uid int64) NewOptionFunc {
	return func(b *models.FileTaskLog) {
		b.UserID = uid
	}
}

func (s *service) Create(ctx context.Context, typ, title string, opts ...NewOptionFunc) (*Tracker, error) {
	now := time.Now()

	log := &models.FileTaskLog{
		Title:    title,
		Type:     typ,
		BeginAt:  now,
		Status:   models.StatusPending,
		Addition: make(datatypes.JSONMap),
	}

	for _, opt := range opts {
		opt(log)
	}

	log.Desc = sanitizeTaskLogText(log.Desc)
	log.Result = sanitizeTaskLogText(log.Result)
	log.ErrorMsg = sanitizeTaskLogText(log.ErrorMsg)

	if err := s.getDB(ctx).Create(log).Error; err != nil {
		ctx.Error("创建文件任务日志失败", zap.Error(err))

		return nil, err
	}

	return newTracker(log.ID, now), nil
}
