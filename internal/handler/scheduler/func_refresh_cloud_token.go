package scheduler

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"go.uber.org/zap"
)

type RefreshCloudTokenScheduler struct {
	running           bool
	stopping          bool
	mu                sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	done              chan struct{}
	cloudTokenService cloudtoken.Service
	firstRunCompleted bool
	refreshInterval   time.Duration
}

const defaultCloudTokenRefreshInterval = 6 * time.Hour

func NewRefreshCloudTokenScheduler(cloudTokenService cloudtoken.Service) Scheduler {
	return &RefreshCloudTokenScheduler{
		cloudTokenService: cloudTokenService,
		running:           false,
		refreshInterval:   defaultCloudTokenRefreshInterval,
	}
}

func (s *RefreshCloudTokenScheduler) Start(ctx context.Context) error {
	if !s.mu.TryLock() {
		return ErrSchedulerRunning
	}
	defer s.mu.Unlock()

	shouldStart, err := shouldStartScheduler(s.running, s.stopping)
	if err != nil {
		return err
	}

	if !shouldStart {
		return nil
	}

	if isNilDependency(s.cloudTokenService) {
		return ErrSchedulerCloudTokenServiceMissing
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.firstRunCompleted = false
	done := markSchedulerRunStarted(&s.running, &s.stopping, &s.done)

	gopool.Go(func() {
		defer finishSchedulerRun(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done, done)

		for s.doJob() {
		}

		ctx.Info("云盘令牌刷新执行器已停止~")
	})

	return nil
}

func (s *RefreshCloudTokenScheduler) Stop() {
	cancel, done, ok := beginSchedulerStop(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done)
	if !ok {
		return
	}

	waitSchedulerStop(cancel, done)
}

func (s *RefreshCloudTokenScheduler) doJob() (keepRunning bool) {
	ctx := s.ctx
	keepRunning = true

	defer func() {
		if r := recover(); r != nil {
			ctx.Error("云盘令牌刷新执行器发生异常",
				zap.String("panic", sanitizeSchedulerPanicValue(r)),
				zap.String("stack", string(debug.Stack())))

			keepRunning = ctx.Err() == nil
		}
	}()

	if !s.waitNextRun(ctx) {
		ctx.Info("云盘令牌刷新执行器停止")

		return false
	}

	ctx.Info("开始执行云盘令牌自动刷新检查")

	// 获取所有使用密码登录的云盘令牌。
	tokens, err := s.getPasswordLoginTokens(ctx)
	if err != nil {
		ctx.Error("查询密码登录令牌失败", zap.Error(err))

		return ctx.Err() == nil
	}

	ctx.Info("查询到密码登录令牌数量", zap.Int("count", len(tokens)))

	// 检查并刷新即将过期或已过期的令牌。
	refreshedCount := 0
	failedCount := 0
	expiredCount := 0
	notExpiringCount := 0

	for _, token := range tokens {
		if token == nil {
			ctx.Warn("密码登录令牌列表包含空记录，跳过")

			continue
		}

		if s.isExpiredOrWillExpireInThreeDays(token) {
			if s.isExpired(token) {
				expiredCount++

				ctx.Info("检测到已过期的令牌，尝试刷新",
					zap.Int64("token_id", token.ID),
					zap.String("token_name", token.Name),
					zap.String("username", utils.MaskSecret(token.Username)))
			} else {
				ctx.Info("检测到即将过期的令牌，尝试刷新",
					zap.Int64("token_id", token.ID),
					zap.String("token_name", token.Name),
					zap.String("username", utils.MaskSecret(token.Username)))
			}

			// 检查失败次数，防止账号被锁定。
			if s.hasTooManyFailures(token) {
				ctx.Warn("令牌刷新失败次数过多，跳过刷新",
					zap.Int64("token_id", token.ID),
					zap.String("token_name", token.Name))

				continue
			}

			if err := s.refreshToken(ctx, token); err != nil {
				ctx.Error("刷新令牌失败",
					zap.Int64("token_id", token.ID),
					zap.String("token_name", token.Name),
					zap.Error(err))

				failedCount++

				s.recordFailure(token)
			} else {
				ctx.Info("刷新令牌成功",
					zap.Int64("token_id", token.ID),
					zap.String("token_name", token.Name))

				refreshedCount++

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

	return ctx.Err() == nil
}

func (s *RefreshCloudTokenScheduler) waitNextRun(ctx context.Context) bool {
	delay := s.nextRunDelay()
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return false
		default:
			s.markFirstRunCompleted()

			return true
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *RefreshCloudTokenScheduler) nextRunDelay() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.firstRunCompleted {
		return 0
	}

	if s.refreshInterval <= 0 {
		return defaultCloudTokenRefreshInterval
	}

	return s.refreshInterval
}

func (s *RefreshCloudTokenScheduler) markFirstRunCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.firstRunCompleted = true
}

// getPasswordLoginTokens 获取所有使用密码登录的云盘令牌
func (s *RefreshCloudTokenScheduler) getPasswordLoginTokens(ctx context.Context) ([]*models.CloudToken, error) {
	return s.cloudTokenService.ListPasswordLoginTokens(ctx)
}

// isExpired 检查令牌是否已过期
func (s *RefreshCloudTokenScheduler) isExpired(token *models.CloudToken) bool {
	return token == nil || token.IsExpired(time.Now())
}

// willExpireInThreeDays 检查令牌是否将在3天内过期
func (s *RefreshCloudTokenScheduler) willExpireInThreeDays(token *models.CloudToken) bool {
	if token == nil {
		return false
	}

	return token.WillExpireWithin(time.Now(), 3*24*time.Hour)
}

// isExpiredOrWillExpireInThreeDays 检查令牌是否已过期或将在3天内过期
func (s *RefreshCloudTokenScheduler) isExpiredOrWillExpireInThreeDays(token *models.CloudToken) bool {
	return s.isExpired(token) || s.willExpireInThreeDays(token)
}

// hasTooManyFailures 检查令牌刷新失败次数是否过多
func (s *RefreshCloudTokenScheduler) hasTooManyFailures(token *models.CloudToken) bool {
	return cloudTokenAutoLoginTimes(token) >= 3
}

func cloudTokenAutoLoginTimes(token *models.CloudToken) int {
	if token == nil || token.Addition == nil {
		return 0
	}

	times, ok := parseCloudTokenAutoLoginTimes(token.Addition[models.CloudTokenAdditionAutoLoginTimes])
	if !ok || times < 0 {
		return 0
	}

	return times
}

func parseCloudTokenAutoLoginTimes(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}

		return int(i), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}

		return i, true
	default:
		return 0, false
	}
}

// recordFailure 记录刷新失败
func (s *RefreshCloudTokenScheduler) recordFailure(token *models.CloudToken) {
	ctx := s.ctx

	currentTimes := cloudTokenAutoLoginTimes(token)

	// 增加失败次数 - 直接使用token.Addition，它已经是datatypes.JSONMap类型
	addition := token.Addition
	if addition == nil {
		addition = make(map[string]interface{})
		token.Addition = addition
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
		token.Addition = addition
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
	if err != nil {
		return err
	}

	token.SetIssuedAt(time.Now())

	return nil
}
