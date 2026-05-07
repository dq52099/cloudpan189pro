package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Service interface {
	GenerateUpgradeKeyword(title, category string) (string, error)
}

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

type Message struct {
	Content string `json:"content"`
}

type service struct {
	config *Config
	logger *zap.Logger
	client *http.Client
}

func NewService(logger *zap.Logger, apiKey, baseURL, model string) Service {
	s := &service{
		config: &Config{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		},
		logger: logger,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	if s.config.APIKey == "" {
		s.config.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if s.config.BaseURL == "" {
		s.config.BaseURL = os.Getenv("OPENAI_BASE_URL")
		if s.config.BaseURL == "" {
			s.config.BaseURL = "https://api.openai.com"
		}
	}
	if s.config.Model == "" {
		s.config.Model = os.Getenv("OPENAI_MODEL")
		if s.config.Model == "" {
			s.config.Model = "gpt-4o-mini"
		}
	}

	return s
}

func (s *service) GenerateUpgradeKeyword(title, category string) (string, error) {
	if s.config.APIKey == "" {
		return "", fmt.Errorf("OpenAI API key is not set")
	}

	prompt := fmt.Sprintf(`请为以下%s生成优化搜索关键词，用于在资源网站搜索更高品质的资源（优先4K、高分辨率、高码率版本）。
要求：
1. 只返回搜索关键词，不要其他解释
2. 关键词要简洁，包含电影名、年份、品质要求
3. 如果是剧集，可以加上"第一季"或"全集"

%s信息：
`, category, category)

	if category == "tv" || category == "anime" {
		prompt += fmt.Sprintf("剧集名称：%s\n", title)
	} else {
		prompt += fmt.Sprintf("电影名称：%s\n", title)
	}

	req := ChatRequest{
		Model: s.config.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	apiURL := buildOpenAIURL(s.config.BaseURL, "/chat/completions")
	reqHTTP, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	reqHTTP.Header.Set("Content-Type", "application/json")
	reqHTTP.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	resp, err := s.client.Do(reqHTTP)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API returned status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("No response from OpenAI")
	}

	keyword := strings.TrimSpace(result.Choices[0].Message.Content)
	keyword = strings.Trim(keyword, "\"")

	s.logger.Info("Generated upgrade keyword", zap.String("title", title), zap.String("keyword", keyword))

	return keyword, nil
}

// buildOpenAIURL 构造 OpenAI 兼容接口 URL。
// 若 base 已包含 /v1 前缀，则直接拼接 path，否则插入 /v1，
// 以兼容 https://api.openai.com 与 https://proxy.example.com/v1 两种形式。
func buildOpenAIURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "https://api.openai.com"
	}

	if strings.HasSuffix(base, "/v1") {
		return base + path
	}

	return base + "/v1" + path
}
