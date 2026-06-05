package telegram

import (
	"bytes"
	stdCtx "context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

const maxTelegramResponseSize = 5 << 20

var (
	telegramCloudShareLinkRegex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9.-])(?:https?:\/\/)?(?:www\.)?cloud\.189\.cn(?::\d+)?\/t\/([a-z0-9]+)`)
	telegramShareCodeRegex      = regexp.MustCompile(`(?i)((?:分享码|share[ _-]?code)\s*[:：]\s*)([a-z0-9]+)`)
	telegramMessageURLRegex     = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)
)

// resolveBackendURL 解析本地后端基础地址，优先使用环境变量 LOCAL_BACKEND_URL，
// 否则回退到默认端口 12395。
func resolveBackendURL() string {
	if envURL := strings.TrimSpace(os.Getenv("LOCAL_BACKEND_URL")); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}

	return "http://127.0.0.1:12395"
}

func decodeLimitedJSONResponse(body io.Reader, target interface{}) error {
	data, err := io.ReadAll(io.LimitReader(body, maxTelegramResponseSize+1))
	if err != nil {
		return err
	}

	if len(data) > maxTelegramResponseSize {
		return fmt.Errorf("telegram 响应体过大，已拒绝")
	}

	return json.Unmarshal(data, target)
}

func sanitizeTelegramLogError(err error) string {
	if err == nil {
		return ""
	}

	return sanitizeTelegramMessageText(err.Error())
}

func sanitizeTelegramAPIResultForLog(result map[string]interface{}) string {
	data, err := json.Marshal(result)
	if err != nil {
		return sanitizeTelegramMessageText(fmt.Sprint(result))
	}

	return sanitizeTelegramMessageText(string(data))
}

func sanitizeTelegramMessageText(text string) string {
	text = telegramMessageURLRegex.ReplaceAllStringFunc(text, utils.RedactURLForLog)

	return utils.RedactSensitiveText(text)
}

func sanitizeTelegramIncomingMessageForLog(text string) string {
	text = sanitizeTelegramMessageText(text)
	text = telegramCloudShareLinkRegex.ReplaceAllStringFunc(text, func(match string) string {
		shareCode, _ := utils.ParseCloud189ShareCode(match, "")
		if shareCode == "" {
			return match
		}

		return strings.Replace(match, shareCode, utils.MaskShareCodeForLog(shareCode), 1)
	})

	return telegramShareCodeRegex.ReplaceAllStringFunc(text, func(match string) string {
		parts := telegramShareCodeRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		return parts[1] + utils.MaskShareCodeForLog(parts[2])
	})
}

func hasTelegramCloudShareLink(text string) bool {
	return utils.IsCloud189ShareLink(text)
}

func parseTelegramCloudShareLink(text string) (string, string, bool) {
	if !hasTelegramCloudShareLink(text) {
		return "", "", false
	}

	shareCode, accessCode := utils.ParseCloud189ShareCode(text, "")
	if !utils.IsCloud189ShareCode(shareCode) {
		return "", "", false
	}

	if accessCode != "" && !utils.IsCloud189AccessCode(accessCode) {
		return "", "", false
	}

	return shareCode, accessCode, true
}

func buildTelegramShareInfoURL(baseURL, path, shareCode, accessCode string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimRight(baseURL, "/") + path)
	if err != nil {
		return "", err
	}

	query := parsedURL.Query()
	query.Set("shareCode", shareCode)

	if accessCode != "" {
		query.Set("shareAccessCode", accessCode)
	}

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

func resolveTelegramMountPath(mountPath, fallbackName, shareCode string) (string, error) {
	mountPath = strings.TrimSpace(mountPath)
	if mountPath != "" {
		normalizedPath, err := utils.NormalizeStoragePath(mountPath)
		if err != nil {
			return "", fmt.Errorf("挂载路径不合法: %w", err)
		}

		if len(utils.SplitNormalizedStoragePath(normalizedPath)) == 0 {
			return "", fmt.Errorf("不允许挂载根路径")
		}

		return normalizedPath, nil
	}

	name := fallbackName
	if strings.TrimSpace(name) == "" {
		name = shareCode
	}

	return utils.JoinStoragePath("/Telegram", name)
}

func telegramErrorMessage(prefix string, err error) string {
	if err == nil {
		return prefix
	}

	return fmt.Sprintf("%s: %s", prefix, sanitizeTelegramMessageText(err.Error()))
}

func telegramTextMessage(prefix string, text string) string {
	if text == "" {
		return prefix
	}

	return fmt.Sprintf("%s: %s", prefix, sanitizeTelegramMessageText(text))
}

func telegramHTMLEscape(text string) string {
	return html.EscapeString(text)
}

func telegramHTMLTextMessage(prefix string, text string) string {
	return telegramHTMLEscape(telegramTextMessage(prefix, text))
}

func telegramHTMLErrorMessage(prefix string, err error) string {
	if err == nil {
		return telegramHTMLEscape(prefix)
	}

	return telegramHTMLTextMessage(prefix, err.Error())
}

type Service interface {
	SendMessage(msg string) error
	SendNotification(title, content string) error
	IsEnabled() bool
	GetProxyURL() string
	GetBotToken() string
	StartBot() error
	StopBot()
	UpdateConfig(config Config) error
	TestConnection() error
	ParseAndMountShareLink(shareURL, mountPath string, autoMount bool) (*MountResult, error)
	SetMountDependencies(fetcher ShareInfoFetcher, mounter StorageMounter)
}

type Config struct {
	BotToken  string
	ChatID    string
	ProxyURL  string
	ProxyType string
	APIURL    string
	Enable    bool
}

// ShareInfoFetcher 分享信息获取接口（用于 Telegram 收到链接时解析）
type ShareInfoFetcher interface {
	GetShareInfo(ctx stdCtx.Context, shareCode, accessCode string) (*ShareInfo, error)
}

// StorageMounter 存储挂载接口
type StorageMounter interface {
	CreateMountPoint(ctx stdCtx.Context, req *MountRequest) (int64, error)
}

// ShareInfo Telegram 模块需要的分享元数据子集
type ShareInfo struct {
	Name       string
	ShareId    int64
	ShareMode  int
	FileId     string
	IsFolder   bool
	AccessCode string
}

// MountRequest 挂载请求
type MountRequest struct {
	LocalPath         string
	OsType            string
	ShareCode         string
	ShareID           int64
	ShareMode         int
	AccessCode        string
	FileID            string
	IsFolder          bool
	EnableDeepRefresh bool
}

type MountResult struct {
	Success   bool
	Message   string
	MountPath string
	ShareID   int64
	FileID    string
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
	backendURL string
	enabled    bool
	logger     *zap.Logger
	client     *http.Client
	mu         sync.RWMutex
	ctx        stdCtx.Context
	botCancel  stdCtx.CancelFunc
	running    bool

	lastOffset int64
	wg         sync.WaitGroup

	shareFetcher ShareInfoFetcher
	mounter      StorageMounter
}

func NewService(botToken, chatID, proxyURL, proxyType, apiURL string, logger *zap.Logger) Service {
	return NewServiceWithConfig(Config{
		BotToken:  botToken,
		ChatID:    chatID,
		ProxyURL:  proxyURL,
		ProxyType: proxyType,
		APIURL:    apiURL,
		Enable:    botToken != "",
	}, logger)
}

func NewServiceWithConfig(config Config, logger *zap.Logger) Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	s := &service{
		backendURL: resolveBackendURL(),
		logger:     logger,
		ctx:        stdCtx.Background(),
	}

	s.applyConfigLocked(config)

	return s
}

func NewServiceFromConfig(db interface{}, logger *zap.Logger, botToken, chatID, proxyURL, proxyType, apiURL string) Service {
	return NewService(botToken, chatID, proxyURL, proxyType, apiURL, logger)
}

func (s *service) UpdateConfig(config Config) error {
	cancel := s.stopPollingForConfigUpdate()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}

	shouldStart := s.applyUpdatedConfig(config)
	if shouldStart {
		return s.StartBot()
	}

	return nil
}

func (s *service) stopPollingForConfigUpdate() stdCtx.CancelFunc {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	cancel := s.botCancel
	s.botCancel = nil

	return cancel
}

func (s *service) applyUpdatedConfig(config Config) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldBotToken := s.botToken
	s.applyConfigLocked(config)

	if oldBotToken != s.botToken {
		s.lastOffset = 0
	}

	return s.enabled
}

func (s *service) applyConfigLocked(config Config) {
	proxyURL, proxyType := normalizeProxyConfig(config.ProxyURL, config.ProxyType)

	s.botToken = config.BotToken
	s.chatID = config.ChatID
	s.proxyURL = proxyURL
	s.proxyType = proxyType
	s.apiURL = strings.TrimRight(config.APIURL, "/")
	s.enabled = config.Enable && config.BotToken != ""
	s.client = &http.Client{
		Timeout: 35 * time.Second,
	}
	s.setupProxy()
}

func normalizeProxyConfig(proxyURL, proxyType string) (string, string) {
	if proxyURL == "" {
		proxyURL = os.Getenv("TG_PROXY")
	}

	if proxyURL == "" {
		return "", ""
	}

	if proxyType == "" {
		proxyType = os.Getenv("TG_PROXY_TYPE")
	}

	if proxyType == "" {
		if strings.HasPrefix(proxyURL, "socks5://") {
			proxyType = "socks5"
		} else {
			proxyType = "http"
		}
	}

	return proxyURL, proxyType
}

func (s *service) setupProxy() {
	proxyURL := s.proxyURL
	if proxyURL == "" {
		return
	}

	var proxyFunc func(*http.Request) (*url.URL, error)

	switch s.proxyType {
	case "socks5":
		proxyFunc = socks5Proxy(proxyURL)
	default:
		parsedProxyURL, err := url.Parse(proxyURL)
		if err != nil {
			s.logger.Warn("Failed to parse proxy URL",
				zap.String("proxy", utils.RedactURLForLog(proxyURL)),
				zap.String("error", sanitizeTelegramLogError(err)),
			)

			return
		}

		proxyFunc = http.ProxyURL(parsedProxyURL)
	}

	transport := proxyHTTPTransport(s.client.Transport)
	transport.Proxy = proxyFunc
	s.client.Transport = transport
	s.logger.Info("Telegram proxy configured", zap.String("proxy", utils.RedactURLForLog(proxyURL)), zap.String("type", s.proxyType))
}

func proxyHTTPTransport(base http.RoundTripper) *http.Transport {
	if transport, ok := base.(*http.Transport); ok && transport != nil {
		return transport.Clone()
	}

	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		return transport.Clone()
	}

	return &http.Transport{}
}

func socks5Proxy(proxyURL string) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		u, err := url.Parse(proxyURL)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	return buildTelegramURL(s.apiURL, s.botToken, method)
}

func buildTelegramURL(apiURL, botToken, method string) string {
	if apiURL == "" {
		apiURL = "https://api.telegram.org"
	}

	return fmt.Sprintf("%s/bot%s/%s", strings.TrimRight(apiURL, "/"), botToken, method)
}

func (s *service) requestSnapshot(method string) (stdCtx.Context, *http.Client, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ctx, s.client, buildTelegramURL(s.apiURL, s.botToken, method)
}

func (s *service) requestContext(ctx stdCtx.Context) stdCtx.Context {
	if ctx != nil {
		return ctx
	}

	if s.ctx != nil {
		return s.ctx
	}

	return stdCtx.Background()
}

func (s *service) nextUpdateOffset() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastOffset + 1
}

func (s *service) setLastOffset(updateID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastOffset = updateID
}

// SetMountDependencies 注入挂载相关依赖，避免 Bot 处理消息时通过 HTTP 回调自身。
func (s *service) SetMountDependencies(fetcher ShareInfoFetcher, mounter StorageMounter) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.shareFetcher = fetcher
	s.mounter = mounter
}

func (s *service) SendMessage(msg string) error {
	return s.sendConfiguredTelegramMessage(msg, "")
}

func (s *service) sendConfiguredTelegramMessage(text, parseMode string) error {
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
		Text:      text,
		ParseMode: parseMode,
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
		s.logger.Warn("Telegram request failed, retrying", zap.Int("retry", i+1), zap.String("error", sanitizeTelegramLogError(err)))
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return lastErr
}

func (s *service) doRequest(method string, body interface{}) error {
	_, err := s.doRequestWithData(method, body)

	return err
}

func (s *service) doRequestWithData(method string, body interface{}) (map[string]interface{}, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	ctx, client, requestURL := s.requestSnapshot(method)

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建 Telegram 请求失败: %s", sanitizeTelegramLogError(err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram 请求失败: %s", sanitizeTelegramLogError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram API returned status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := decodeLimitedJSONResponse(resp.Body, &result); err != nil {
		return nil, err
	}

	okVal, ok := result["ok"].(bool)
	if !ok || !okVal {
		description, _ := result["description"].(string)

		return nil, fmt.Errorf("telegram API error: %s", description)
	}

	return result, nil
}

func (s *service) SendNotification(title, content string) error {
	message := fmt.Sprintf("<b>%s</b>\n\n%s", telegramHTMLEscape(title), telegramHTMLEscape(content))

	return s.sendConfiguredTelegramMessage(message, "HTML")
}

func (s *service) StartBot() error {
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()

		return nil
	}

	if s.running {
		s.mu.Unlock()

		return nil
	}

	botCtx, cancel := stdCtx.WithCancel(s.ctx)
	s.botCancel = cancel
	s.running = true

	s.wg.Add(1)
	s.mu.Unlock()

	s.logger.Info("Starting Telegram bot polling...")

	go s.polling(botCtx)

	return nil
}

func (s *service) StopBot() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()

		return
	}

	cancel := s.botCancel
	s.botCancel = nil
	s.running = false
	s.mu.Unlock()

	s.logger.Info("Stopping Telegram bot...")

	if cancel != nil {
		cancel()
	}

	s.wg.Wait()
	s.logger.Info("Telegram bot stopped")
}

func (s *service) polling(ctx stdCtx.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchUpdates(ctx)
		}
	}
}

func (s *service) fetchUpdates(ctx stdCtx.Context) {
	offset := s.nextUpdateOffset()

	params := map[string]interface{}{
		"offset":  offset,
		"timeout": 30,
	}

	jsonBody, err := json.Marshal(params)
	if err != nil {
		s.logger.Error("Failed to marshal params", zap.Error(err))

		return
	}

	_, client, requestURL := s.requestSnapshot("getUpdates")

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		s.logger.Error("Failed to create request", zap.String("error", sanitizeTelegramLogError(err)))

		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Failed to fetch updates", zap.String("error", sanitizeTelegramLogError(err)))

		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Telegram API returned non-OK status", zap.Int("status", resp.StatusCode))

		return
	}

	var result map[string]interface{}
	if err := decodeLimitedJSONResponse(resp.Body, &result); err != nil {
		s.logger.Error("Failed to decode response", zap.Error(err))

		return
	}

	ok, _ := result["ok"].(bool)
	if !ok {
		s.logger.Error("Telegram API returned error", zap.String("result", sanitizeTelegramAPIResultForLog(result)))

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

		s.setLastOffset(int64(updateID))

		if msg, ok := updateMap["message"].(map[string]interface{}); ok {
			s.handleMessage(ctx, msg)
		}
	}
}

func (s *service) handleMessage(ctx stdCtx.Context, msg map[string]interface{}) {
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
		zap.String("username", utils.MaskSecret(username)),
		zap.String("first_name", utils.MaskSecret(firstName)),
		zap.String("text", sanitizeTelegramIncomingMessageForLog(text)))

	switch {
	case text == "/start" || text == "/help":
		s.sendHelpMessage(int64(chatID))
	case strings.HasPrefix(text, "/"):
		s.handleCommand(text, int64(chatID), int64(userID))
	default:
		if hasTelegramCloudShareLink(text) {
			s.handleShareLinkWithContext(ctx, text, int64(chatID), int64(userID))
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
	helpText := fmt.Sprintf(`未知命令: %s`, telegramHTMLEscape(cmd))
	msg := telegramMessage{
		ChatID:    fmt.Sprintf("%d", chatID),
		Text:      helpText,
		ParseMode: "HTML",
	}
	_ = s.doRequest("sendMessage", msg)
}

func (s *service) handleShareLink(link string, chatID, userID int64) {
	s.handleShareLinkWithContext(s.ctx, link, chatID, userID)
}

func (s *service) handleShareLinkWithContext(ctx stdCtx.Context, link string, chatID, userID int64) {
	ctx = s.requestContext(ctx)

	s.sendMessageToChat(chatID, "收到分享链接，正在处理...")

	shareCode, accessCode, ok := parseTelegramCloudShareLink(link)
	if !ok {
		s.sendMessageToChat(chatID, "无法识别分享链接，请检查链接格式")

		return
	}

	// 若已注入分享/挂载依赖，则直接调用内部服务，避免 HTTP 自调
	if s.shareFetcher != nil && s.mounter != nil {
		s.handleShareLinkDirectWithContext(ctx, shareCode, accessCode, chatID)

		return
	}

	// 兼容模式：通过本地 HTTP 调用（保留原有行为）
	shareInfoURL, err := buildTelegramShareInfoURL(s.backendURL, "/api/storage/advance/share_info", shareCode, accessCode)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("解析失败", err))

		return
	}

	req, err := http.NewRequestWithContext(ctx, "GET", shareInfoURL, nil)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("解析失败", err))

		return
	}

	resp, err := s.GetClient().Do(req)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("解析失败", err))

		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result map[string]interface{}
	if err := decodeLimitedJSONResponse(resp.Body, &result); err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("解析失败", err))

		return
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		s.sendMessageToChat(chatID, "无法获取分享信息")

		return
	}

	name, _ := data["name"].(string)

	shareIDFloat, ok := data["shareId"].(float64)
	if !ok {
		s.sendMessageToChat(chatID, "无法获取ShareID")

		return
	}

	shareID := int64(shareIDFloat)
	fileID, _ := data["id"].(string)

	s.sendMessageToChat(chatID, fmt.Sprintf(`📁 名称: %s\n🔗 ShareID: %d\n📂 FileID: %s\n\n正在自动创建挂载点...`, telegramHTMLEscape(name), shareID, telegramHTMLEscape(fileID)))

	localPath, err := resolveTelegramMountPath("", name, shareCode)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("创建挂载点失败", err))

		return
	}

	batchAddReq := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"localPath":         localPath,
				"osType":            "subscribe_share_folder",
				"shareCode":         shareCode,
				"shareAccessCode":   accessCode,
				"fileId":            fileID,
				"enableDeepRefresh": true,
			},
		},
	}

	batchAddJSON, err := json.Marshal(batchAddReq)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("创建挂载点失败", err))

		return
	}

	batchAddURL := s.backendURL + "/api/storage/batch_add"

	req, err = http.NewRequestWithContext(ctx, "POST", batchAddURL, bytes.NewReader(batchAddJSON))
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("创建挂载点失败", err))

		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err = s.GetClient().Do(req)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("创建挂载点失败", err))

		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var batchResult map[string]interface{}
	if err := decodeLimitedJSONResponse(resp.Body, &batchResult); err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("创建挂载点失败", err))

		return
	}

	if mountID, _, ok := parseBatchAddMountResult(batchResult); ok {
		msg := fmt.Sprintf(`✅ <b>挂载成功！</b>

📁 本地路径: %s
🔗 ShareID: %d
🆔 挂载ID: %d

请通过文件管理器查看，或等待后台扫描完成后使用。`, telegramHTMLEscape(localPath), shareID, mountID)
		s.sendMessageToChat(chatID, msg)

		return
	}

	_, errMsg, _ := parseBatchAddMountResult(batchResult)
	s.sendMessageToChat(chatID, telegramHTMLTextMessage("❌ 创建挂载点失败", errMsg))
}

func (s *service) handleShareLinkDirectWithContext(ctx stdCtx.Context, shareCode, accessCode string, chatID int64) {
	ctx = s.requestContext(ctx)

	info, err := s.shareFetcher.GetShareInfo(ctx, shareCode, accessCode)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("获取分享信息失败", err))

		return
	}

	name := info.Name
	if name == "" {
		name = shareCode
	}

	localPath, err := resolveTelegramMountPath("", name, shareCode)
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("❌ 创建挂载点失败", err))

		return
	}

	s.sendMessageToChat(chatID, fmt.Sprintf(`📁 名称: %s
🔗 ShareID: %d
📂 FileID: %s

正在自动创建挂载点...`, telegramHTMLEscape(name), info.ShareId, telegramHTMLEscape(info.FileId)))

	resolvedAccessCode := info.AccessCode
	if resolvedAccessCode == "" {
		resolvedAccessCode = accessCode
	}

	mountID, err := s.mounter.CreateMountPoint(ctx, &MountRequest{
		LocalPath:         localPath,
		OsType:            "subscribe_share_folder",
		ShareCode:         shareCode,
		ShareID:           info.ShareId,
		ShareMode:         info.ShareMode,
		AccessCode:        resolvedAccessCode,
		FileID:            info.FileId,
		IsFolder:          info.IsFolder,
		EnableDeepRefresh: true,
	})
	if err != nil {
		s.sendMessageToChat(chatID, telegramHTMLErrorMessage("❌ 创建挂载点失败", err))

		return
	}

	s.sendMessageToChat(chatID, fmt.Sprintf(`✅ <b>挂载成功！</b>

📁 本地路径: %s
🔗 ShareID: %d
🆔 挂载ID: %d

请通过文件管理器查看，或等待后台扫描完成后使用。`, telegramHTMLEscape(localPath), info.ShareId, mountID))
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
	s.mu.RLock()
	enabled := s.enabled
	chatID := s.chatID
	s.mu.RUnlock()

	if !enabled {
		return fmt.Errorf("telegram bot is not enabled")
	}

	params := map[string]interface{}{
		"chat_id": chatID,
		"text":    "🔔 测试消息 - Bot 运行正常",
	}

	return s.doRequest("sendMessage", params)
}

func (s *service) GetMe() (map[string]interface{}, error) {
	result, err := s.doRequestWithData("getMe", nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *service) ParseAndMountShareLink(shareURL, mountPath string, autoMount bool) (*MountResult, error) {
	return s.ParseAndMountShareLinkWithContext(s.ctx, shareURL, mountPath, autoMount)
}

func (s *service) ParseAndMountShareLinkWithContext(ctx stdCtx.Context, shareURL, mountPath string, autoMount bool) (*MountResult, error) {
	ctx = s.requestContext(ctx)

	shareCode, accessCode, ok := parseTelegramCloudShareLink(shareURL)
	if !ok {
		return &MountResult{Success: false, Message: "无法识别分享链接，请检查链接格式"}, nil
	}

	// 优先使用内部服务
	if s.shareFetcher != nil {
		return s.parseAndMountDirect(ctx, shareCode, accessCode, mountPath, autoMount)
	}

	// 兼容模式：通过本地 HTTP 调用
	apiURL, err := buildTelegramShareInfoURL(s.backendURL, "/api/public/share_info", shareCode, accessCode)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("解析失败", err)}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("解析失败", err)}, nil
	}

	resp, err := s.GetClient().Do(req)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("解析失败", err)}, nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result map[string]interface{}
	if err := decodeLimitedJSONResponse(resp.Body, &result); err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("解析失败", err)}, nil
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return &MountResult{Success: false, Message: "无法获取分享信息"}, nil
	}

	name, nameOk := data["name"].(string)
	if !nameOk {
		name = ""
	}

	shareIDFloat, ok := data["shareId"].(float64)
	if !ok {
		return &MountResult{Success: false, Message: "无法获取ShareID"}, nil
	}

	shareID := int64(shareIDFloat)

	fileID, fileIDOk := data["id"].(string)
	if !fileIDOk {
		fileID = ""
	}

	if !autoMount {
		return &MountResult{
			Success:   true,
			Message:   "分享链接解析成功",
			MountPath: mountPath,
			ShareID:   shareID,
			FileID:    fileID,
		}, nil
	}

	localPath, err := resolveTelegramMountPath(mountPath, name, shareCode)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("创建挂载点失败", err)}, nil
	}

	batchAddReq := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"localPath":         localPath,
				"osType":            "subscribe_share_folder",
				"shareCode":         shareCode,
				"shareAccessCode":   accessCode,
				"fileId":            fileID,
				"enableDeepRefresh": true,
			},
		},
	}

	batchAddJSON, err := json.Marshal(batchAddReq)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("创建挂载点失败", err)}, nil
	}

	batchAddURL := s.backendURL + "/api/storage/batch_add"

	req, err = http.NewRequestWithContext(ctx, "POST", batchAddURL, bytes.NewReader(batchAddJSON))
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("创建挂载点失败", err)}, nil
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err = s.GetClient().Do(req)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("创建挂载点失败", err)}, nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var batchResult map[string]interface{}
	if err := decodeLimitedJSONResponse(resp.Body, &batchResult); err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("创建挂载点失败", err)}, nil
	}

	if _, _, ok := parseBatchAddMountResult(batchResult); ok {
		return &MountResult{
			Success:   true,
			Message:   "挂载成功",
			MountPath: localPath,
			ShareID:   shareID,
			FileID:    fileID,
		}, nil
	}

	_, errMsg, _ := parseBatchAddMountResult(batchResult)

	return &MountResult{Success: false, Message: telegramTextMessage("创建挂载点失败", errMsg)}, nil
}

func parseBatchAddMountResult(batchResult map[string]interface{}) (int64, string, bool) {
	results := getBatchAddResults(batchResult)
	if isSuccessfulBatchCode(batchResult["code"]) && len(results) > 0 {
		if firstResult, ok := results[0].(map[string]interface{}); ok {
			if success, ok := firstResult["success"].(bool); ok && success {
				mountID, _ := toInt64(firstResult["id"])

				return mountID, "", true
			}
		}
	}

	errMsg, _ := batchResult["msg"].(string)
	if errMsg == "" && len(results) > 0 {
		if firstResult, ok := results[0].(map[string]interface{}); ok {
			errMsg, _ = firstResult["error"].(string)
		}
	}

	if errMsg == "" {
		errMsg = "响应结构异常"
	}

	return 0, sanitizeTelegramMessageText(errMsg), false
}

func getBatchAddResults(batchResult map[string]interface{}) []interface{} {
	data, ok := batchResult["data"].(map[string]interface{})
	if !ok {
		return nil
	}

	results, ok := data["results"].([]interface{})
	if !ok {
		return nil
	}

	return results
}

func isSuccessfulBatchCode(v interface{}) bool {
	code, ok := toInt64(v)
	if !ok {
		return false
	}

	return code == 0 || code == 200
}

func toInt64(v interface{}) (int64, bool) {
	switch value := v.(type) {
	case float64:
		return int64(value), true
	case int:
		return int64(value), true
	case int64:
		return value, true
	case json.Number:
		n, err := value.Int64()
		if err != nil {
			return 0, false
		}

		return n, true
	default:
		return 0, false
	}
}

// parseAndMountDirect 使用内部服务完成分享解析与挂载。
func (s *service) parseAndMountDirect(ctx stdCtx.Context, shareCode, accessCode, mountPath string, autoMount bool) (*MountResult, error) {
	ctx = s.requestContext(ctx)

	info, err := s.shareFetcher.GetShareInfo(ctx, shareCode, accessCode)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("解析失败", err)}, nil
	}

	if !autoMount {
		return &MountResult{
			Success:   true,
			Message:   "分享链接解析成功",
			MountPath: mountPath,
			ShareID:   info.ShareId,
			FileID:    info.FileId,
		}, nil
	}

	if s.mounter == nil {
		return &MountResult{Success: false, Message: "挂载服务未初始化"}, nil
	}

	localPath, err := resolveTelegramMountPath(mountPath, info.Name, shareCode)
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("创建挂载点失败", err)}, nil
	}

	resolvedAccessCode := info.AccessCode
	if resolvedAccessCode == "" {
		resolvedAccessCode = accessCode
	}

	mountID, err := s.mounter.CreateMountPoint(ctx, &MountRequest{
		LocalPath:         localPath,
		OsType:            "subscribe_share_folder",
		ShareCode:         shareCode,
		ShareID:           info.ShareId,
		ShareMode:         info.ShareMode,
		AccessCode:        resolvedAccessCode,
		FileID:            info.FileId,
		IsFolder:          info.IsFolder,
		EnableDeepRefresh: true,
	})
	if err != nil {
		return &MountResult{Success: false, Message: telegramErrorMessage("创建挂载点失败", err)}, nil
	}

	_ = mountID

	return &MountResult{
		Success:   true,
		Message:   "挂载成功",
		MountPath: localPath,
		ShareID:   info.ShareId,
		FileID:    info.FileId,
	}, nil
}
