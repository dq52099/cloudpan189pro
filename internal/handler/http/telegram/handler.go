package telegram

import (
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

func serverError(msg string) httpcontext.BusinessError {
	return &customBusinessError{
		httpCode:     http.StatusInternalServerError,
		businessCode: 500,
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

func (h *Handler) GetSetting() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var setting models.TelegramSetting
		result := h.db.First(&setting)
		if result.Error != nil {
			if result.Error.Error() == "record not found" {
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
	BotToken         string `json:"botToken"`
	ProxyURL         string `json:"proxyURL"`
	ProxyType        string `json:"proxyType"`
	APIURL           string `json:"apiURL"`
	ChatID           string `json:"chatID"`
	DefaultMountPath string `json:"defaultMountPath"`
	EnableNotify     bool   `json:"enableNotify"`
	Enable           bool   `json:"enable"`
}

func (h *Handler) UpdateSetting() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req UpdateSettingReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))
			return
		}

		var setting models.TelegramSetting
		result := h.db.First(&setting)
		if result.Error != nil && result.Error.Error() != "record not found" {
			c.Fail(invalidParams(result.Error))
			return
		}

		setting.BotTokenEncrypted = req.BotToken
		setting.ProxyURL = req.ProxyURL
		setting.ProxyType = req.ProxyType
		setting.APIURL = req.APIURL
		setting.ChatID = req.ChatID
		setting.DefaultMountPath = req.DefaultMountPath
		setting.EnableNotify = req.EnableNotify
		setting.Enable = req.Enable

		if setting.ID == 0 {
			result = h.db.Create(&setting)
		} else {
			result = h.db.Save(&setting)
		}

		if result.Error != nil {
			c.Fail(invalidParams(result.Error))
			return
		}

		setting.BotToken = setting.BotTokenEncrypted
		setting.BotTokenEncrypted = ""
		c.Success(setting)
	}
}

func (h *Handler) TestConnection() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		if h.service == nil || !h.service.IsEnabled() {
			c.Fail(invalidParams(&customBusinessError{httpCode: http.StatusBadRequest, message: "Telegram bot is not enabled"}))
			return
		}

		err := h.service.TestConnection()
		if err != nil {
			c.Fail(invalidParams(err))
			return
		}

		c.Success(gin.H{"message": "Connection test successful"})
	}
}

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
			c.Fail(notFound("User not found"))
			return
		}

		user.MountPath = req.MountPath
		user.IsAdmin = req.IsAdmin

		result = h.db.Save(&user)
		if result.Error != nil {
			c.Fail(invalidParams(result.Error))
			return
		}

		c.Success(user)
	}
}

func (h *Handler) SendMessage() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req struct {
			Message string `json:"message" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))
			return
		}

		if h.service == nil || !h.service.IsEnabled() {
			c.Fail(invalidParams(&customBusinessError{httpCode: http.StatusBadRequest, message: "Telegram bot is not enabled"}))
			return
		}

		err := h.service.SendMessage(req.Message)
		if err != nil {
			c.Fail(invalidParams(err))
			return
		}

		c.Success(gin.H{"message": "Message sent successfully"})
	}
}
