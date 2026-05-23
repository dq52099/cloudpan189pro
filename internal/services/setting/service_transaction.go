package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"gorm.io/gorm"
)

func (s *service) RunInTransaction(ctx context.Context, run func(context.Context) error) error {
	if run == nil {
		return nil
	}

	if bootstrap.HasTransactionDB(ctx) {
		return run(ctx)
	}

	var txCtx context.Context

	if err := s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx = bootstrap.WithTransactionDB(ctx, tx)

		return run(txCtx)
	}); err != nil {
		return err
	}

	bootstrap.RunAfterCommitHooks(txCtx)

	return nil
}
