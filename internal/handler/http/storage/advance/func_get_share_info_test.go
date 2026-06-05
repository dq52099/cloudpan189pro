package advance

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"go.uber.org/zap"
)

type mockGetShareInfoCloudBridge struct {
	cloudbridgeSvi.Service
	shareCodes  []string
	accessCodes []string
}

func (m *mockGetShareInfoCloudBridge) GetShareInfo(ctx appContext.Context, shareCode string, accessCode string) (*cloudbridgeSvi.ShareInfo, error) {
	m.shareCodes = append(m.shareCodes, shareCode)
	m.accessCodes = append(m.accessCodes, accessCode)

	return &cloudbridgeSvi.ShareInfo{Name: "分享_" + shareCode}, nil
}

func TestGetShareInfoNormalizesLabeledCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		shareCode      string
		shareAccess    string
		wantShareCode  string
		wantAccessCode string
	}{
		{
			name:          "chinese share label only",
			shareCode:     "分享码：abcDEF",
			wantShareCode: "abcDEF",
		},
		{
			name:           "chinese share and access labels",
			shareCode:      "分享码：abcDEF 提取码：wxyz",
			wantShareCode:  "abcDEF",
			wantAccessCode: "wxyz",
		},
		{
			name:           "parenthesized access code",
			shareCode:      "abcDEF（访问码：wxyz）",
			wantShareCode:  "abcDEF",
			wantAccessCode: "wxyz",
		},
		{
			name:           "camel case english labels",
			shareCode:      "shareCode:abc accessCode:wxyz",
			wantShareCode:  "abc",
			wantAccessCode: "wxyz",
		},
		{
			name:           "explicit access overrides embedded access",
			shareCode:      "分享码：abcDEF 提取码：wxyz",
			shareAccess:    "zzzz",
			wantShareCode:  "abcDEF",
			wantAccessCode: "zzzz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloudBridge := &mockGetShareInfoCloudBridge{}
			router := gin.New()
			wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
			router.GET("/share_info", wrapper.Wrap(NewHandler(cloudBridge, nil).GetShareInfo()))

			values := url.Values{}
			values.Set("shareCode", tt.shareCode)

			if tt.shareAccess != "" {
				values.Set("shareAccessCode", tt.shareAccess)
			}

			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/share_info?"+values.Encode(), nil)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if len(cloudBridge.shareCodes) != 1 || len(cloudBridge.accessCodes) != 1 {
				t.Fatalf("expected one get share info call, got share=%v access=%v", cloudBridge.shareCodes, cloudBridge.accessCodes)
			}

			if cloudBridge.shareCodes[0] != tt.wantShareCode || cloudBridge.accessCodes[0] != tt.wantAccessCode {
				t.Fatalf("expected share/access %q/%q, got %q/%q", tt.wantShareCode, tt.wantAccessCode, cloudBridge.shareCodes[0], cloudBridge.accessCodes[0])
			}
		})
	}
}

func TestGetShareInfoRejectsInvalidShareCodeBeforeCloudBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockGetShareInfoCloudBridge{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/share_info", wrapper.Wrap(NewHandler(cloudBridge, nil).GetShareInfo()))

	values := url.Values{}
	values.Set("shareCode", "https://example.com/share?token=secret-token")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/share_info?"+values.Encode(), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.shareCodes) != 0 || len(cloudBridge.accessCodes) != 0 {
		t.Fatalf("expected cloud bridge not to be called, got share=%v access=%v", cloudBridge.shareCodes, cloudBridge.accessCodes)
	}

	text := recorder.Body.String()
	if !strings.Contains(text, "有效分享码") {
		t.Fatalf("expected invalid share code message, got body=%s", text)
	}

	if strings.Contains(text, "secret-token") || strings.Contains(text, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got body=%s", text)
	}
}

func TestGetShareInfoRejectsInvalidAccessCodeBeforeCloudBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockGetShareInfoCloudBridge{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/share_info", wrapper.Wrap(NewHandler(cloudBridge, nil).GetShareInfo()))

	values := url.Values{}
	values.Set("shareCode", "https://cloud.189.cn/t/abcDEF")
	values.Set("shareAccessCode", "https://example.com/share?token=secret-token")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/share_info?"+values.Encode(), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.shareCodes) != 0 || len(cloudBridge.accessCodes) != 0 {
		t.Fatalf("expected cloud bridge not to be called, got share=%v access=%v", cloudBridge.shareCodes, cloudBridge.accessCodes)
	}

	text := recorder.Body.String()
	if !strings.Contains(text, "访问码格式无效") {
		t.Fatalf("expected invalid access code message, got body=%s", text)
	}

	if strings.Contains(text, "secret-token") || strings.Contains(text, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got body=%s", text)
	}
}

func TestGetShareInfoReturnsErrorWhenCloudBridgeServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/share_info", wrapper.Wrap(NewHandler(nil, nil).GetShareInfo()))

	values := url.Values{}
	values.Set("shareCode", "abcDEF")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/share_info?"+values.Encode(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceGetShareInfoError)
}
