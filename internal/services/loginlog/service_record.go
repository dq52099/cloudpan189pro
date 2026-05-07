package loginlog

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
)

// RecordLoginInput 登录事件记录输入
type RecordLoginInput struct {
	UserId    int64           `form:"userId"  binding:"omitempty" json:"userId"`
	Username  string          `form:"username" binding:"omitempty" json:"username"`
	Addr      string          `form:"addr"    binding:"omitempty" json:"addr"`
	Location  string          `form:"location" binding:"omitempty" json:"location"`
	Method    loginlog.Method `form:"method"  binding:"omitempty" json:"method"`
	Status    loginlog.Status `form:"status"  binding:"omitempty" json:"status"`
	Reason    string          `form:"reason"  binding:"omitempty" json:"reason"`
	UserAgent string          `form:"userAgent" binding:"omitempty" json:"userAgent"`
}

// RecordRefreshInput 刷新令牌事件记录输入
type RecordRefreshInput struct {
	UserId    int64           `form:"userId"  binding:"omitempty" json:"userId"`
	Username  string          `form:"username" binding:"omitempty" json:"username"`
	Addr      string          `form:"addr"    binding:"omitempty" json:"addr"`
	Location  string          `form:"location" binding:"omitempty" json:"location"`
	Method    loginlog.Method `form:"method"  binding:"omitempty" json:"method"`
	Status    loginlog.Status `form:"status"  binding:"omitempty" json:"status"`
	Reason    string          `form:"reason"  binding:"omitempty" json:"reason"`
	UserAgent string          `form:"userAgent" binding:"omitempty" json:"userAgent"`
}

// RecordLogin 记录登录事件
func (s *service) RecordLogin(ctx context.Context, in *RecordLoginInput) (int64, error) {
	if in == nil {
		return 0, errors.New("input is nil")
	}

	// 默认值
	method := in.Method
	if method == "" {
		method = loginlog.MethodWeb
	}

	status := in.Status
	if status == "" {
		// 调用方未显式指定状态视为成功（调用该函数通常代表业务流程已成功触发记录）
		status = loginlog.StatusSuccess
	}

	log := &models.LoginLog{
		UserId:    in.UserId,
		Username:  in.Username,
		Addr:      in.Addr,
		Location:  in.Location,
		Method:    method,
		Event:     loginlog.EventLogin,
		Status:    status,
		Reason:    in.Reason,
		UserAgent: in.UserAgent,
		TraceId:   ctx.ID(),
	}

	return s.Create(ctx, log)
}

// RecordRefreshToken 记录刷新令牌事件
func (s *service) RecordRefreshToken(ctx context.Context, in *RecordRefreshInput) (int64, error) {
	if in == nil {
		return 0, errors.New("input is nil")
	}

	// 默认值
	method := in.Method
	if method == "" {
		method = loginlog.MethodWeb
	}

	status := in.Status
	if status == "" {
		status = loginlog.StatusSuccess
	}

	log := &models.LoginLog{
		UserId:    in.UserId,
		Username:  in.Username,
		Addr:      in.Addr,
		Location:  in.Location,
		Method:    method,
		Event:     loginlog.EventRefreshToken,
		Status:    status,
		Reason:    in.Reason,
		UserAgent: in.UserAgent,
		TraceId:   ctx.ID(),
	}

	return s.Create(ctx, log)
}
