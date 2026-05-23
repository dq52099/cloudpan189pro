package bootstrap

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"gorm.io/gorm"
)

type transactionDBContextKey struct{}

type transactionContextState struct {
	db               *gorm.DB
	afterCommitHooks []func()
}

func WithTransactionDB(ctx context.Context, tx *gorm.DB) context.Context {
	if tx == nil {
		return ctx
	}

	return ctx.WithValue(transactionDBContextKey{}, &transactionContextState{db: tx})
}

func DBFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	state := transactionStateFromContext(ctx)
	if state != nil && state.db != nil {
		return state.db.WithContext(ctx)
	}

	if fallback == nil {
		return nil
	}

	return fallback.WithContext(ctx)
}

func HasTransactionDB(ctx context.Context) bool {
	state := transactionStateFromContext(ctx)

	return state != nil && state.db != nil
}

func AddAfterCommitHook(ctx context.Context, hook func()) bool {
	if hook == nil {
		return false
	}

	state := transactionStateFromContext(ctx)
	if state == nil {
		return false
	}

	state.afterCommitHooks = append(state.afterCommitHooks, hook)

	return true
}

func RunAfterCommitHooks(ctx context.Context) {
	state := transactionStateFromContext(ctx)
	if state == nil {
		return
	}

	hooks := append([]func(){}, state.afterCommitHooks...)
	state.afterCommitHooks = nil

	for _, hook := range hooks {
		hook()
	}
}

func transactionStateFromContext(ctx context.Context) *transactionContextState {
	state, _ := ctx.Value(transactionDBContextKey{}).(*transactionContextState)

	return state
}
