package media

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediaconfigSvc "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	"go.uber.org/zap"
)

type mockConfigUpdateMediaConfigService struct {
	mediaconfigSvc.Service
	fields []utils.Field
}

func (m *mockConfigUpdateMediaConfigService) Update(ctx appContext.Context, fields ...utils.Field) error {
	m.fields = fields

	return nil
}

func (m *mockConfigUpdateMediaConfigService) Query(ctx appContext.Context) (*models.MediaConfig, error) {
	return nil, nil
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
