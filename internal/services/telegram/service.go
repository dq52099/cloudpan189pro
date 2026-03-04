package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Service interface {
	SendMessage(msg string) error
	SendNotification(title, content string) error
	IsEnabled() bool
	GetProxyURL() string
	GetBotToken() string
	StartBot() error
	StopBot()
	TestConnection() error
}

type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      *Chat  `json:"chat"`
	Text      string `json:"text"`
	Date      int64  `json:"date"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type service struct {
	botToken   string
	chatID     string
	proxyURL   string
	proxyType  string
	apiURL     string
	enabled    bool
	logger     *zap.Logger
	client     *http.Client
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	lastOffset int64
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

func NewService(botToken, chatID, proxyURL, proxyType, apiURL string, logger *zap.Logger) Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &service{
		botToken:  botToken,
		chatID:    chatID,
		proxyURL:  proxyURL,
		proxyType: proxyType,
		apiURL:    apiURL,
		logger:    logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		ctx:      ctx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
	}

	if botToken != "" {
		s.enabled = true
	}

	if proxyURL == "" {
		proxyURL = os.Getenv("TG_PROXY")
	}
	if proxyURL != "" && s.proxyType == "" {
		proxyType = os.Getenv("TG_PROXY_TYPE")
		if proxyType == "" {
			if strings.HasPrefix(proxyURL, "socks5://") {
				proxyType = "socks5"
			} else {
				proxyType = "http"
			}
		}
		s.proxyURL = proxyURL
		s.proxyType = proxyType
		s.setupProxy()
	}

	return s
}

func NewServiceFromConfig(db interface{}, logger *zap.Logger, botToken, chatID, proxyURL, proxyType, apiURL string) Service {
	return NewService(botToken, chatID, proxyURL, proxyType, apiURL, logger)
}

func (s *service) setupProxy() {
	if s.proxyURL == "" {
		return
	}

	var proxyFunc func(*http.Request) (*url.URL, error)

	switch s.proxyType {
	case "socks5":
		proxyFunc = s.getSocks5Proxy()
	default:
		proxyURL, err := url.Parse(s.proxyURL)
		if err != nil {
			s.logger.Warn("Failed to parse proxy URL", zap.String("proxy", s.proxyURL), zap.Error(err))
			return
		}
		proxyFunc = http.ProxyURL(proxyURL)
	}

	s.client.Transport = &http.Transport{
		Proxy: proxyFunc,
	}
	s.logger.Info("Telegram proxy configured", zap.String("proxy", s.proxyURL), zap.String("type", s.proxyType))
}

func (s *service) getSocks5Proxy() func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		u, err := url.Parse(s.proxyURL)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
}

func (s *service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *service) GetProxyURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proxyURL
}

func (s *service) GetBotToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.botToken
}

func (s *service) GetClient() *http.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *service) buildURL(method string) string {
	if s.apiURL == "" {
		s.apiURL = "https://api.telegram.org"
	}
	return fmt.Sprintf("%s/bot%s/%s", s.apiURL, s.botToken, method)
}

func (s *service) SendMessage(msg string) error {
	s.mu.RLock()
	if !s.enabled || s.botToken == "" {
		s.mu.RUnlock()
		return nil
	}
	chatID := s.chatID
	s.mu.RUnlock()

	if chatID == "" {
		return nil
	}

	message := telegramMessage{
		ChatID:    chatID,
		Text:      msg,
		ParseMode: "HTML",
	}

	return s.doRequestWithRetry("sendMessage", message, 3)
}

func (s *service) doRequestWithRetry(method string, body interface{}, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := s.doRequest(method, body)
		if err == nil {
			return nil
		}
		lastErr = err
		s.logger.Warn("Telegram request failed, retrying", zap.Int("retry", i+1), zap.Error(err))
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return lastErr
}

func (s *service) doRequest(method string, body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", s.buildURL(method), bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	okVal, ok := result["ok"].(bool)
	if !ok || !okVal {
		description, _ := result["description"].(string)
		return fmt.Errorf("telegram API error: %s", description)
	}

	return nil
}

func (s *service) SendNotification(title, content string) error {
	message := fmt.Sprintf("<b>%s</b>\n\n%s", title, content)
	return s.SendMessage(message)
}

func (s *service) StartBot() error {
	if !s.enabled {
		return nil
	}

	s.logger.Info("Starting Telegram bot polling...")
	s.wg.Add(1)
	go s.polling()

	return nil
}

func (s *service) StopBot() {
	s.logger.Info("Stopping Telegram bot...")
	s.cancel()
	close(s.stopChan)
	s.wg.Wait()
	s.logger.Info("Telegram bot stopped")
}

func (s *service) polling() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.fetchUpdates()
		}
	}
}

func (s *service) fetchUpdates() {
	offset := s.lastOffset + 1

	params := map[string]interface{}{
		"offset":  offset,
		"timeout": 30,
	}

	jsonBody, err := json.Marshal(params)
	if err != nil {
		s.logger.Error("Failed to marshal params", zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", s.buildURL("getUpdates"), bytes.NewReader(jsonBody))
	if err != nil {
		s.logger.Error("Failed to create request", zap.Error(err))
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to fetch updates", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Telegram API returned non-OK status", zap.Int("status", resp.StatusCode))
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.logger.Error("Failed to decode response", zap.Error(err))
		return
	}

	ok, _ := result["ok"].(bool)
	if !ok {
		s.logger.Error("Telegram API returned error", zap.Any("result", result))
		return
	}

	updates, ok := result["result"].([]interface{})
	if !ok {
		return
	}

	for _, u := range updates {
		updateMap, ok := u.(map[string]interface{})
		if !ok {
			continue
		}

		updateID, ok := updateMap["update_id"].(float64)
		if !ok {
			continue
		}

		s.lastOffset = int64(updateID)

		if msg, ok := updateMap["message"].(map[string]interface{}); ok {
			s.handleMessage(msg)
		}
	}
}

func (s *service) handleMessage(msg map[string]interface{}) {
	text, _ := msg["text"].(string)
	if text == "" {
		return
	}

	from, _ := msg["from"].(map[string]interface{})
	chat, _ := msg["chat"].(map[string]interface{})

	userID, _ := from["id"].(float64)
	username, _ := from["username"].(string)
	firstName, _ := from["first_name"].(string)
	chatID, _ := chat["id"].(float64)

	s.logger.Info("Received message",
		zap.Int64("user_id", int64(userID)),
		zap.String("username", username),
		zap.String("first_name", firstName),
		zap.String("text", text))

	switch {
	case text == "/start" || text == "/help":
		s.sendHelpMessage(int64(chatID))
	case strings.HasPrefix(text, "/"):
		s.handleCommand(text, int64(chatID), int64(userID))
	default:
		if strings.Contains(text, "189.cn") || strings.Contains(text, "cloud.189.cn") {
			s.handleShareLink(text, int64(chatID), int64(userID))
		}
	}
}

func (s *service) sendHelpMessage(chatID int64) {
	helpText := `<b>天翼云盘智能管理 Bot</b>

<b>命令列表：</b>
/start - 开始使用
/help - 查看帮助

<b>使用方式：</b>
直接发送 189 分享链接给我，我会自动识别并挂载！

<b>说明：</b>
- 支持批量转发多个链接
- 自动生成 STRM 文件
- 支持自定义挂载路径`

	msg := telegramMessage{
		ChatID:    fmt.Sprintf("%d", chatID),
		Text:      helpText,
		ParseMode: "HTML",
	}
	_ = s.doRequest("sendMessage", msg)
}

func (s *service) handleCommand(cmd string, chatID, userID int64) {
	helpText := fmt.Sprintf(`未知命令: %s`, cmd)
	msg := telegramMessage{
		ChatID:    fmt.Sprintf("%d", chatID),
		Text:      helpText,
		ParseMode: "HTML",
	}
	_ = s.doRequest("sendMessage", msg)
}

func (s *service) handleShareLink(link string, chatID, userID int64) {
	s.sendMessageToChat(chatID, "收到分享链接，正在处理...")

	// 这里调用挂载服务处理链接
	// TODO: 集成挂载服务
	s.sendMessageToChat(chatID, fmt.Sprintf("链接已收到: %s\n\n注意: 挂载功能需要通过 Web 界面配置", link))
}

func (s *service) sendMessageToChat(chatID int64, text string) {
	msg := telegramMessage{
		ChatID:    fmt.Sprintf("%d", chatID),
		Text:      text,
		ParseMode: "HTML",
	}
	_ = s.doRequest("sendMessage", msg)
}

func (s *service) TestConnection() error {
	if !s.enabled {
		return fmt.Errorf("telegram bot is not enabled")
	}

	params := map[string]interface{}{
		"chat_id": s.chatID,
		"text":    "🔔 测试消息 - Bot 运行正常",
	}

	return s.doRequest("sendMessage", params)
}

func (s *service) GetMe() (map[string]interface{}, error) {
	var result map[string]interface{}
	err := s.doRequest("getMe", nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}
