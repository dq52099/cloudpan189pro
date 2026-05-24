package telegram

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/telegram"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Handler struct {
	service telegram.Service
	db      *gorm.DB
	logger  *zap.Logger
}

func NewHandler(db *gorm.DB, tgService telegram.Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: tgService,
		db:      db,
		logger:  logger,
	}
}

func invalidParams(err error) httpcontext.BusinessError {
	return &customBusinessError{
		httpCode:     http.StatusBadRequest,
		businessCode: 400,
		message:      err.Error(),
	}
}

func notFound(msg string) httpcontext.BusinessError {
	return &customBusinessError{
		httpCode:     http.StatusNotFound,
		businessCode: 404,
		message:      msg,
	}
}

type customBusinessError struct {
	httpCode     int
	businessCode int
	message      string
}

func (e *customBusinessError) GetHTTPCode() int   { return e.httpCode }
func (e *customBusinessError) GetCode() int       { return e.businessCode }
func (e *customBusinessError) GetMessage() string { return e.message }
func (e *customBusinessError) GetError() error    { return nil }
func (e *customBusinessError) Error() string      { return e.message }
func (e *customBusinessError) WithError(err error) httpcontext.BusinessError {
	return e
}
func (e *customBusinessError) WithHTTPCode(code int) httpcontext.BusinessError {
	e.httpCode = code

	return e
}
func (e *customBusinessError) WithMessage(msg string) httpcontext.BusinessError {
	e.message = msg

	return e
}
func (e *customBusinessError) WithBusinessCode(code int) httpcontext.BusinessError {
	e.businessCode = code

	return e
}

// GetSetting 获取 Telegram 配置
// @Summary 获取 Telegram 配置
// @Description 获取 Telegram 机器人配置，未初始化时返回默认配置
// @Tags Telegram 管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=models.TelegramSetting} "获取成功"
// @Failure 400 {object} httpcontext.Response "获取 Telegram 配置失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/telegram/setting [get]
func (h *Handler) GetSetting() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var setting models.TelegramSetting

		result := h.db.First(&setting)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.Success(models.TelegramSetting{
					Enable:           false,
					DefaultMountPath: "/转存",
					APIURL:           "https://api.telegram.org",
				})

				return
			}

			c.Fail(invalidParams(result.Error))

			return
		}

		setting.BotToken = setting.BotTokenEncrypted
		setting.BotTokenEncrypted = ""
		c.Success(setting)
	}
}

type UpdateSettingReq struct {
	BotToken         *string `json:"botToken"`
	ProxyURL         *string `json:"proxyURL"`
	ProxyType        *string `json:"proxyType"`
	APIURL           *string `json:"apiURL"`
	ChatID           *string `json:"chatID"`
	DefaultMountPath *string `json:"defaultMountPath"`
	EnableNotify     *bool   `json:"enableNotify"`
	Enable           *bool   `json:"enable"`
}

// UpdateSetting 更新 Telegram 配置
// @Summary 更新 Telegram 配置
// @Description 创建或更新 Telegram 机器人配置，支持只提交需要修改的字段
// @Tags Telegram 管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body UpdateSettingReq true "Telegram 配置"
// @Success 200 {object} httpcontext.Response{data=models.TelegramSetting} "更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败或更新失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "Telegram 配置不存在"
// @Router /api/telegram/setting [post]
func (h *Handler) UpdateSetting() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req UpdateSettingReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))

			return
		}

		var setting models.TelegramSetting

		result := h.db.First(&setting)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.Fail(invalidParams(result.Error))

			return
		}

		if setting.ID == 0 {
			setting.APIURL = "https://api.telegram.org"
			setting.DefaultMountPath = "/转存"
			setting.EnableNotify = true
		}

		applyTelegramSettingUpdate(&setting, &req)

		updates := telegramSettingUpdateMap(&req)

		if setting.ID == 0 {
			result = h.db.Create(&setting)
		} else {
			if len(updates) == 0 {
				result = &gorm.DB{RowsAffected: 0}
			} else {
				result = h.db.Model(&models.TelegramSetting{}).
					Where("id = ?", setting.ID).
					Updates(updates)
			}
		}

		if result.Error != nil {
			c.Fail(invalidParams(result.Error))

			return
		}

		if setting.ID != 0 && result.RowsAffected == 0 {
			if err := h.checkTelegramSettingUpdateResult(result, setting.ID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.Fail(notFound("Setting not found"))
				} else {
					c.Fail(invalidParams(err))
				}

				return
			}
		}

		setting.BotToken = setting.BotTokenEncrypted
		setting.BotTokenEncrypted = ""
		c.Success(setting)
	}
}

func applyTelegramSettingUpdate(setting *models.TelegramSetting, req *UpdateSettingReq) {
	if req.BotToken != nil {
		setting.BotTokenEncrypted = *req.BotToken
	}

	if req.ProxyURL != nil {
		setting.ProxyURL = *req.ProxyURL
	}

	if req.ProxyType != nil {
		setting.ProxyType = *req.ProxyType
	}

	if req.APIURL != nil {
		setting.APIURL = *req.APIURL
	}

	if req.ChatID != nil {
		setting.ChatID = *req.ChatID
	}

	if req.DefaultMountPath != nil {
		setting.DefaultMountPath = *req.DefaultMountPath
	}

	if req.EnableNotify != nil {
		setting.EnableNotify = *req.EnableNotify
	}

	if req.Enable != nil {
		setting.Enable = *req.Enable
	}
}

func telegramSettingUpdateMap(req *UpdateSettingReq) map[string]interface{} {
	updates := make(map[string]interface{})
	if req.BotToken != nil {
		updates["bot_token_encrypted"] = *req.BotToken
	}

	if req.ProxyURL != nil {
		updates["proxy_url"] = *req.ProxyURL
	}

	if req.ProxyType != nil {
		updates["proxy_type"] = *req.ProxyType
	}

	if req.APIURL != nil {
		updates["api_url"] = *req.APIURL
	}

	if req.ChatID != nil {
		updates["chat_id"] = *req.ChatID
	}

	if req.DefaultMountPath != nil {
		updates["default_mount_path"] = *req.DefaultMountPath
	}

	if req.EnableNotify != nil {
		updates["enable_notify"] = *req.EnableNotify
	}

	if req.Enable != nil {
		updates["enable"] = *req.Enable
	}

	return updates
}

// TestConnection 测试 Telegram 连接
// @Summary 测试 Telegram 连接
// @Description 使用当前 Telegram 配置测试机器人连接状态
// @Tags Telegram 管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response "测试成功"
// @Failure 400 {object} httpcontext.Response "Telegram 未启用、令牌未配置或连接测试失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "Telegram 配置未初始化"
// @Router /api/telegram/test [post]
func (h *Handler) TestConnection() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var setting models.TelegramSetting
		if err := h.db.First(&setting).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.Fail(notFound("Telegram 配置未初始化"))
			} else {
				c.Fail(invalidParams(fmt.Errorf("failed to get settings: %w", err)))
			}

			return
		}

		if setting.BotToken == "" && setting.BotTokenEncrypted == "" || !setting.Enable {
			c.Fail(invalidParams(&customBusinessError{httpCode: http.StatusBadRequest, message: "Telegram bot is not enabled or token not configured"}))

			return
		}

		token := setting.BotToken
		if token == "" {
			token = setting.BotTokenEncrypted
		}

		testService := telegram.NewService(
			token,
			setting.ChatID,
			setting.ProxyURL,
			setting.ProxyType,
			setting.APIURL,
			h.logger,
		)

		err := testService.TestConnection()
		if err != nil {
			c.Fail(invalidParams(err))

			return
		}

		c.Success(gin.H{"message": "Connection test successful"})
	}
}

// GetUserList 获取 Telegram 用户列表
// @Summary 获取 Telegram 用户列表
// @Description 获取已记录的 Telegram 用户列表
// @Tags Telegram 管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=[]models.TelegramUser} "获取成功"
// @Failure 400 {object} httpcontext.Response "获取 Telegram 用户列表失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/telegram/users [get]
func (h *Handler) GetUserList() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var users []models.TelegramUser

		result := h.db.Order("last_seen_at desc").Find(&users)
		if result.Error != nil {
			c.Fail(invalidParams(result.Error))

			return
		}

		c.Success(users)
	}
}

type UpdateUserReq struct {
	UserID    int64  `json:"userID"`
	MountPath string `json:"mountPath"`
	IsAdmin   bool   `json:"isAdmin"`
}

// UpdateUser 更新 Telegram 用户
// @Summary 更新 Telegram 用户
// @Description 更新 Telegram 用户的默认挂载路径和管理员标记
// @Tags Telegram 管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body UpdateUserReq true "Telegram 用户配置"
// @Success 200 {object} httpcontext.Response{data=models.TelegramUser} "更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败或更新失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "Telegram 用户不存在"
// @Router /api/telegram/user [post]
func (h *Handler) UpdateUser() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req UpdateUserReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))

			return
		}

		var user models.TelegramUser

		result := h.db.Where("user_id = ?", req.UserID).First(&user)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.Fail(notFound("User not found"))
			} else {
				c.Fail(invalidParams(result.Error))
			}

			return
		}

		user.MountPath = req.MountPath
		user.IsAdmin = req.IsAdmin

		result = h.db.Model(&models.TelegramUser{}).
			Where("id = ?", user.ID).
			Select("mount_path", "is_admin").
			Updates(user)
		if result.Error != nil {
			c.Fail(invalidParams(result.Error))

			return
		}

		if result.RowsAffected == 0 {
			if err := h.checkTelegramUserUpdateResult(result, user.ID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.Fail(notFound("User not found"))
				} else {
					c.Fail(invalidParams(err))
				}

				return
			}
		}

		c.Success(user)
	}
}

func (h *Handler) checkTelegramSettingUpdateResult(result *gorm.DB, id int64) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 0 {
		return nil
	}

	return h.ensureTelegramSettingExists(id)
}

func (h *Handler) checkTelegramUserUpdateResult(result *gorm.DB, id int64) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 0 {
		return nil
	}

	return h.ensureTelegramUserExists(id)
}

func (h *Handler) ensureTelegramSettingExists(id int64) error {
	var count int64
	if err := h.db.Model(&models.TelegramSetting{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (h *Handler) ensureTelegramUserExists(id int64) error {
	var count int64
	if err := h.db.Model(&models.TelegramUser{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

type sendMessageReq struct {
	Message string `json:"message" binding:"required"`
}

// SendMessage 发送 Telegram 测试消息
// @Summary 发送 Telegram 消息
// @Description 使用当前 Telegram 配置发送一条消息
// @Tags Telegram 管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body sendMessageReq true "消息内容"
// @Success 200 {object} httpcontext.Response "发送成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败、Telegram 未启用或发送失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "Telegram 配置未初始化"
// @Router /api/telegram/send [post]
func (h *Handler) SendMessage() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req sendMessageReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))

			return
		}

		var setting models.TelegramSetting
		if err := h.db.First(&setting).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.Fail(notFound("Telegram 配置未初始化"))
			} else {
				c.Fail(invalidParams(fmt.Errorf("failed to get settings: %w", err)))
			}

			return
		}

		if (setting.BotToken == "" && setting.BotTokenEncrypted == "") || !setting.Enable {
			c.Fail(invalidParams(&customBusinessError{httpCode: http.StatusBadRequest, message: "Telegram bot is not enabled or token not configured"}))

			return
		}

		token := setting.BotToken
		if token == "" {
			token = setting.BotTokenEncrypted
		}

		msgService := telegram.NewService(
			token,
			setting.ChatID,
			setting.ProxyURL,
			setting.ProxyType,
			setting.APIURL,
			h.logger,
		)

		err := msgService.SendMessage(req.Message)
		if err != nil {
			c.Fail(invalidParams(err))

			return
		}

		c.Success(gin.H{"message": "Message sent successfully"})
	}
}

type ProcessShareLinkReq struct {
	ShareURL  string `json:"shareUrl" binding:"required"`
	ChatID    string `json:"chatID"`
	MountPath string `json:"mountPath"`
	AutoMount bool   `json:"autoMount"`
}

// ProcessShareLink 处理 Telegram 分享链接
// @Summary 处理 Telegram 分享链接
// @Description 解析分享链接，可按配置将资源自动挂载到指定路径
// @Tags Telegram 管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body ProcessShareLinkReq true "分享链接处理参数"
// @Success 200 {object} httpcontext.Response "处理成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败或处理失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "Telegram 配置未初始化"
// @Router /api/telegram/process_share [post]
func (h *Handler) ProcessShareLink() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req ProcessShareLinkReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))

			return
		}

		// 优先使用已启动的 Telegram 服务（具有挂载依赖注入）
		if h.service != nil && h.service.IsEnabled() {
			result, err := h.service.ParseAndMountShareLink(req.ShareURL, req.MountPath, req.AutoMount)
			if err != nil {
				c.Fail(invalidParams(err))

				return
			}

			if result.Success {
				c.Success(gin.H{
					"success":   true,
					"message":   result.Message,
					"mountPath": result.MountPath,
					"shareID":   result.ShareID,
					"fileID":    result.FileID,
				})

				return
			}

			c.Fail(invalidParams(fmt.Errorf("%s", result.Message)))

			return
		}

		// 回退：根据数据库中的配置临时创建服务（无挂载依赖，将走 HTTP 自调）
		var setting models.TelegramSetting
		if err := h.db.First(&setting).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.Fail(notFound("Telegram 配置未初始化"))
			} else {
				c.Fail(invalidParams(fmt.Errorf("failed to get settings: %w", err)))
			}

			return
		}

		token := setting.BotToken
		if token == "" {
			token = setting.BotTokenEncrypted
		}

		targetChatID := req.ChatID
		if targetChatID == "" {
			targetChatID = setting.ChatID
		}

		msgService := telegram.NewService(
			token,
			targetChatID,
			setting.ProxyURL,
			setting.ProxyType,
			setting.APIURL,
			h.logger,
		)

		result, err := msgService.ParseAndMountShareLink(req.ShareURL, req.MountPath, req.AutoMount)
		if err != nil {
			c.Fail(invalidParams(err))

			return
		}

		if result.Success {
			c.Success(gin.H{
				"success":   true,
				"message":   result.Message,
				"mountPath": result.MountPath,
				"shareID":   result.ShareID,
				"fileID":    result.FileID,
			})
		} else {
			errMsg := result.Message
			c.Fail(invalidParams(fmt.Errorf("%s", errMsg)))
		}
	}
}
