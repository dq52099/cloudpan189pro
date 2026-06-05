package media

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediaconfigSvc "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockConfigUpdateMediaConfigService struct {
	mediaconfigSvc.Service
	fields      []utils.Field
	err         error
	queryConfig *models.MediaConfig
	queryErr    error
	queryCalls  int
}

func (m *mockConfigUpdateMediaConfigService) Update(ctx appContext.Context, fields ...utils.Field) error {
	if m.err != nil {
		return m.err
	}

	m.fields = fields

	return nil
}

func (m *mockConfigUpdateMediaConfigService) Query(ctx appContext.Context) (*models.MediaConfig, error) {
	m.queryCalls++

	return m.queryConfig, m.queryErr
}

func TestConfigUpdatePassesAutoRebuildCron(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockConfigUpdateMediaConfigService{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config/update", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigUpdate()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config/update",
		strings.NewReader(`{"autoRebuildEnable":true,"autoRebuildCron":"0 5 * * *"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	fields := make(map[string]any, len(service.fields))
	for _, field := range service.fields {
		fields[field.Key] = field.Value
	}

	if fields["auto_rebuild_enable"] != true {
		t.Fatalf("expected auto_rebuild_enable true, got %#v", fields["auto_rebuild_enable"])
	}

	if fields["auto_rebuild_cron"] != "0 5 * * *" {
		t.Fatalf("expected auto_rebuild_cron field, got %#v", fields["auto_rebuild_cron"])
	}
}

func TestConfigUpdateRejectsInvalidAutoRebuildIntervalBeforeUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{name: "zero", body: `{"autoRebuildInterval":0}`},
		{name: "negative", body: `{"autoRebuildInterval":-1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockConfigUpdateMediaConfigService{}
			router := gin.New()
			wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
			router.POST("/config/update", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigUpdate()))

			req := httptest.NewRequestWithContext(
				stdctx.Background(),
				http.MethodPost,
				"/config/update",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if len(service.fields) != 0 {
				t.Fatalf("expected invalid interval not to update config, got %#v", service.fields)
			}
		})
	}
}

func TestConfigUpdateAcceptsEmptyBodyAsNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockConfigUpdateMediaConfigService{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config/update", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigUpdate()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config/update",
		nil,
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(service.fields) != 0 {
		t.Fatalf("expected no update fields, got %#v", service.fields)
	}
}

func TestConfigUpdateReturnsNotFoundWhenConfigMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockConfigUpdateMediaConfigService{err: gorm.ErrRecordNotFound}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config/update", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigUpdate()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config/update",
		strings.NewReader(`{"autoRebuildEnable":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != codeConfigNotInit.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeConfigNotInit.GetCode(), response.Code)
	}
}

func TestConfigUpdatePreservesBaseURLWhenRedactedPlaceholderSubmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawBaseURL := "https://media-user:media-pass@example.test/strm?access_token=secret-token#session=secret-session"
	service := &mockConfigUpdateMediaConfigService{
		queryConfig: &models.MediaConfig{
			ID:      1,
			BaseURL: rawBaseURL,
		},
	}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config/update", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigUpdate()))

	body := `{"baseURL":` + strconv.Quote(utils.RedactURLForLog(rawBaseURL)) + `,"autoClean":true}`
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config/update",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	fields := make(map[string]any, len(service.fields))
	for _, field := range service.fields {
		fields[field.Key] = field.Value
	}

	if service.queryCalls != 1 {
		t.Fatalf("expected current config to be queried once, got %d", service.queryCalls)
	}

	if _, ok := fields["base_url"]; ok {
		t.Fatalf("expected redacted baseURL placeholder not to be persisted, got %#v", fields["base_url"])
	}

	if fields["auto_clean"] != true {
		t.Fatalf("expected other update fields to be preserved, got %#v", fields)
	}
}

func TestConfigUpdateRejectsInvalidBaseURLBeforeUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "blank", baseURL: "   "},
		{name: "relative", baseURL: "/media"},
		{name: "unsupported scheme", baseURL: "mailto:media@example.test"},
		{name: "missing host", baseURL: "https:///media"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockConfigUpdateMediaConfigService{
				queryConfig: &models.MediaConfig{
					ID:      1,
					BaseURL: "https://old.example.test",
				},
			}
			router := gin.New()
			wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
			router.POST("/config/update", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigUpdate()))

			body := `{"baseURL":` + strconv.Quote(tt.baseURL) + `}`
			req := httptest.NewRequestWithContext(
				stdctx.Background(),
				http.MethodPost,
				"/config/update",
				strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if service.queryCalls != 1 {
				t.Fatalf("expected current config to be queried once, got %d", service.queryCalls)
			}

			if len(service.fields) != 0 {
				t.Fatalf("expected invalid baseURL not to update config, got %#v", service.fields)
			}
		})
	}
}

func TestConfigUpdateTrimsBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockConfigUpdateMediaConfigService{
		queryConfig: &models.MediaConfig{
			ID:      1,
			BaseURL: "https://old.example.test",
		},
	}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config/update", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigUpdate()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config/update",
		strings.NewReader(`{"baseURL":" https://new.example.test/media "}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(service.fields) != 1 {
		t.Fatalf("expected one update field, got %#v", service.fields)
	}

	if service.fields[0].Key != "base_url" || service.fields[0].Value != "https://new.example.test/media" {
		t.Fatalf("expected trimmed base_url update, got %#v", service.fields[0])
	}
}
