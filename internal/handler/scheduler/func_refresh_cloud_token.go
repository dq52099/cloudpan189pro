package scheduler

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"go.uber.org/zap"
)

type RefreshCloudTokenScheduler struct {
	running           bool
	mu                sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	cloudTokenService cloudtoken.Service
}

func NewRefreshCloudTokenScheduler(cloudTokenService cloudtoken.Service) Scheduler {
	return &RefreshCloudTokenScheduler{
		cloudTokenService: cloudTokenService,
		running:           false,
	}
}

func (s *RefreshCloudTokenScheduler) Start(ctx context.Context) error {
	if !s.mu.TryLock() {
		return ErrSchedulerRunning
	}
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	s.running = true

	gopool.Go(func() {
		for s.doJob() {
		}

		ctx.Info("云盘令牌刷新执行器已停止~")
	})

	return nil
}

func (s *RefreshCloudTokenScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false
}

func (s *RefreshCloudTokenScheduler) doJob() bool {
	ctx := s.ctx

	defer func() {
		if r := recover(); r != nil {
			ctx.Error("云盘令牌刷新执行器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	// 首次启动时立即执行一次检查
	firstRun := true

	for {
		select {
		case <-ctx.Done():
			ctx.Info("云盘令牌刷新执行器停止")

			return false
		case <-time.After(func() time.Duration {
			if firstRun {
				firstRun = false

				return 0 // 立即执行
			}

			return 6 * time.Hour // 后续每6小时执行一次
		}()):
			ctx.Info("开始执行云盘令牌自动刷新检查")

			// 获取所有使用密码登录的云盘令牌
			tokens, err := s.getPasswordLoginTokens(ctx)
			if err != nil {
				ctx.Error("查询密码登录令牌失败", zap.Error(err))

				continue
			}

			ctx.Info("查询到密码登录令牌数量", zap.Int("count", len(tokens)))

			// 检查并刷新即将过期或已过期的令牌
			refreshedCount := 0
			failedCount := 0
			expiredCount := 0
			notExpiringCount := 0

			for _, token := range tokens {
				// 检查令牌是否已过期或将在3天内过期
				if s.isExpiredOrWillExpireInThreeDays(token) {
					if s.isExpired(token) {
						expiredCount++

						ctx.Info("检测到已过期的令牌，尝试刷新",
							zap.Int64("token_id", token.ID),
							zap.String("token_name", token.Name),
							zap.String("username", token.Username))
					} else {
						ctx.Info("检测到即将过期的令牌，尝试刷新",
							zap.Int64("token_id", token.ID),
							zap.String("token_name", token.Name),
							zap.String("username", token.Username))
					}

					// 检查失败次数，防止账号被锁定
					if s.hasTooManyFailures(token) {
						ctx.Warn("令牌刷新失败次数过多，跳过刷新",
							zap.Int64("token_id", token.ID),
							zap.String("token_name", token.Name))

						continue
					}

					// 尝试刷新令牌
					if err := s.refreshToken(ctx, token); err != nil {
						ctx.Error("刷新令牌失败",
							zap.Int64("token_id", token.ID),
							zap.String("token_name", token.Name),
							zap.Error(err))

						failedCount++

						// 记录失败次数
						s.recordFailure(token)
					} else {
						ctx.Info("刷新令牌成功",
							zap.Int64("token_id", token.ID),
							zap.String("token_name", token.Name))

						refreshedCount++

						// 重置失败次数
						s.resetFailureCount(token)
					}
				} else {
					notExpiringCount++
				}
			}

			ctx.Info("云盘令牌自动刷新完成",
				zap.Int("refreshed_count", refreshedCount),
				zap.Int("failed_count", failedCount),
				zap.Int("expired_count", expiredCount),
				zap.Int("not_expiring_count", notExpiringCount),
				zap.Int("total_checked", len(tokens)))
		}
	}
}

// getPasswordLoginTokens 获取所有使用密码登录的云盘令牌
func (s *RefreshCloudTokenScheduler) getPasswordLoginTokens(ctx context.Context) ([]*models.CloudToken, error) {
	return s.cloudTokenService.ListPasswordLoginTokens(ctx)
}

// isExpired 检查令牌是否已过期
func (s *RefreshCloudTokenScheduler) isExpired(token *models.CloudToken) bool {
	if token.ExpiresIn <= 0 {
		return true // 没有有效过期时间，视为已过期
	}

	now := time.Now()

	var expireTime time.Time

	if token.ExpiresIn > 10000000000 { // 超过10年的秒数，可能是毫秒时间戳
		// 毫秒时间戳
		expireTime = time.Unix(token.ExpiresIn/1000, 0)
	} else {
		// 剩余秒数，计算过期时间
		// 需要知道令牌创建时间，这里使用当前时间加上剩余秒数作为近似
		expireTime = now.Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	return expireTime.Before(now)
}

// willExpireInThreeDays 检查令牌是否将在3天内过期
func (s *RefreshCloudTokenScheduler) willExpireInThreeDays(token *models.CloudToken) bool {
	if token.ExpiresIn <= 0 {
		return false
	}

	now := time.Now()

	var expireTime time.Time

	if token.ExpiresIn > 10000000000 { // 超过10年的秒数，可能是毫秒时间戳
		// 毫秒时间戳
		expireTime = time.Unix(token.ExpiresIn/1000, 0)
	} else {
		// 剩余秒数，计算过期时间
		// 需要知道令牌创建时间，这里使用当前时间加上剩余秒数作为近似
		expireTime = now.Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	threeDaysLater := now.Add(3 * 24 * time.Hour)

	return expireTime.Before(threeDaysLater) && expireTime.After(now)
}

// isExpiredOrWillExpireInThreeDays 检查令牌是否已过期或将在3天内过期
func (s *RefreshCloudTokenScheduler) isExpiredOrWillExpireInThreeDays(token *models.CloudToken) bool {
	return s.isExpired(token) || s.willExpireInThreeDays(token)
}

// hasTooManyFailures 检查令牌刷新失败次数是否过多
func (s *RefreshCloudTokenScheduler) hasTooManyFailures(token *models.CloudToken) bool {
	// 从addition字段获取失败次数
	if autoLoginTimes, ok := token.Addition[models.CloudTokenAdditionAutoLoginTimes]; ok {
		if times, ok := autoLoginTimes.(float64); ok && times >= 3 {
			return true
		}

		if times, ok := autoLoginTimes.(int64); ok && times >= 3 {
			return true
		}
	}

	return false
}

// recordFailure 记录刷新失败
func (s *RefreshCloudTokenScheduler) recordFailure(token *models.CloudToken) {
	ctx := s.ctx

	// 获取当前失败次数
	currentTimes := 0

	if autoLoginTimes, ok := token.Addition[models.CloudTokenAdditionAutoLoginTimes]; ok {
		if times, ok := autoLoginTimes.(float64); ok {
			currentTimes = int(times)
		}

		if times, ok := autoLoginTimes.(int64); ok {
			currentTimes = int(times)
		}
	}

	// 增加失败次数 - 直接使用token.Addition，它已经是datatypes.JSONMap类型
	addition := token.Addition
	if addition == nil {
		addition = make(map[string]interface{})
	}

	addition[models.CloudTokenAdditionAutoLoginTimes] = currentTimes + 1
	addition[models.CloudTokenAdditionAutoLoginResultKey] = fmt.Sprintf("%s, 自动刷新失败", time.Now().Format(time.DateTime))

	// 更新数据库
	if err := s.cloudTokenService.UpdateAddition(ctx, token.ID, addition); err != nil {
		ctx.Error("更新令牌失败次数失败", zap.Error(err), zap.Int64("token_id", token.ID))
	}
}

// resetFailureCount 重置失败次数
func (s *RefreshCloudTokenScheduler) resetFailureCount(token *models.CloudToken) {
	ctx := s.ctx

	addition := token.Addition
	if addition == nil {
		addition = make(map[string]interface{})
	}

	addition[models.CloudTokenAdditionAutoLoginTimes] = 0
	addition[models.CloudTokenAdditionAutoLoginResultKey] = fmt.Sprintf("%s, 自动刷新成功", time.Now().Format(time.DateTime))

	// 更新数据库
	if err := s.cloudTokenService.UpdateAddition(ctx, token.ID, addition); err != nil {
		ctx.Error("重置令牌失败次数失败", zap.Error(err), zap.Int64("token_id", token.ID))
	}
}

// refreshToken 刷新令牌
func (s *RefreshCloudTokenScheduler) refreshToken(ctx context.Context, token *models.CloudToken) error {
	// 使用UsernameLogin方法刷新令牌
	req := &cloudtoken.UsernameLoginRequest{
		ID:       token.ID,
		Username: token.Username,
		Password: token.Password,
		Name:     token.Name,
		IsAdmin:  true,
	}

	_, err := s.cloudTokenService.UsernameLogin(ctx, req)

	return err
}
