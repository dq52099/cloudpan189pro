package setting

import (
	"errors"
	"strings"

	"go.uber.org/zap"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type InitSystemRequest struct {
	Title      string
	EnableAuth bool
	BaseURL    string
}

func (s *service) InitSystem(ctx context.Context, req *InitSystemRequest) error {
	if req == nil {
		return errInvalidInitSystemRequest
	}

	title := strings.TrimSpace(req.Title)
	baseURL := strings.TrimSpace(req.BaseURL)

	if title == "" || baseURL == "" {
		return errInvalidInitSystemRequest
	}

	setting, err := s.Query(ctx)
	if err != nil {
		ctx.Error("设置查询失败", zap.Error(err))

		return err
	}

	if setting.Initialized {
		ctx.Error("系统已初始化")

		return errors.New("系统已初始化")
	}

	return s.Update(ctx,
		utils.WithField("title", title),
		utils.WithField("enable_auth", req.EnableAuth),
		utils.WithField("base_url", baseURL),
		utils.WithField("initialized", true),
	)
}
