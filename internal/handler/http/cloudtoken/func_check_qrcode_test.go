package cloudtoken

import (
	stdctx "context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockCheckQrcodeCloudTokenService struct {
	cloudtokenSvi.Service
	err error
}

func (m *mockCheckQrcodeCloudTokenService) CheckQrcode(ctx appContext.Context, req *cloudtokenSvi.CheckQrcodeRequest) error {
	return m.err
}

func TestCheckQrcodeReturnsNotFoundWhenCloudTokenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(88))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/check_qrcode", wrapper.Wrap(NewHandler(&mockCheckQrcodeCloudTokenService{
		err: errors.Join(errors.New("missing token"), gorm.ErrRecordNotFound),
	}, nil, nil).CheckQrcode()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/check_qrcode",
		strings.NewReader(`{"id":123,"uuid":"uuid"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertCloudTokenNotFoundResponse(t, recorder)
}
