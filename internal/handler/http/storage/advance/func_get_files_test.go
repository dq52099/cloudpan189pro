package advance

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"go.uber.org/zap"
)

type mockGetFilesCloudBridge struct {
	cloudbridgeSvi.Service
	personList      *cloudbridgeSvi.PersonFileListResponse
	familyList      *cloudbridgeSvi.FamilyFileListResponse
	familyCloudList *cloudbridgeSvi.GetFamilyListResponse
	personCount     int64
	familyCount     int64
}

func (m *mockGetFilesCloudBridge) PersonFileList(
	ctx appContext.Context,
	token cloudbridgeSvi.AuthToken,
	parentId string,
	pageNum int,
	pageSize int,
) (*cloudbridgeSvi.PersonFileListResponse, error) {
	return m.personList, nil
}

func (m *mockGetFilesCloudBridge) PersonFileCount(
	ctx appContext.Context,
	token cloudbridgeSvi.AuthToken,
	parentId string,
) (int64, error) {
	return m.personCount, nil
}

func (m *mockGetFilesCloudBridge) FamilyFileList(
	ctx appContext.Context,
	token cloudbridgeSvi.AuthToken,
	familyId string,
	parentId string,
	pageNum int,
	pageSize int,
) (*cloudbridgeSvi.FamilyFileListResponse, error) {
	return m.familyList, nil
}

func (m *mockGetFilesCloudBridge) FamilyFileCount(
	ctx appContext.Context,
	token cloudbridgeSvi.AuthToken,
	familyId string,
	parentId string,
) (int64, error) {
	return m.familyCount, nil
}

func (m *mockGetFilesCloudBridge) FamilyList(
	ctx appContext.Context,
	token cloudbridgeSvi.AuthToken,
) (*cloudbridgeSvi.GetFamilyListResponse, error) {
	return m.familyCloudList, nil
}

func TestFamilyListReturnsQueryErrorWhenCloudBridgeReturnsNilList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAdvanceFilesTestRouter("/family/list", NewHandler(
		&mockGetFilesCloudBridge{},
		newAdvanceValidCloudTokenService(),
	).FamilyList())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/family/list?cloudToken=123", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryPathFailed)
}

func TestFamilyListReturnsQueryErrorWhenCloudBridgeServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAdvanceFilesTestRouter("/family/list", NewHandler(
		nil,
		newAdvanceValidCloudTokenService(),
	).FamilyList())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/family/list?cloudToken=123", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryPathFailed)
}

func TestFamilyListReturnsQueryErrorWhenCloudBridgeServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var cloudBridgeService *mockGetFilesCloudBridge

	router := newAdvanceFilesTestRouter("/family/list", NewHandler(
		cloudBridgeService,
		newAdvanceValidCloudTokenService(),
	).FamilyList())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/family/list?cloudToken=123", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryPathFailed)
}

func TestGetPersonFilesReturnsQueryErrorWhenCloudBridgeReturnsNilList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAdvanceFilesTestRouter("/person/files", NewHandler(
		&mockGetFilesCloudBridge{},
		newAdvanceValidCloudTokenService(),
	).GetPersonFiles())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/person/files?cloudToken=123&pageNum=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryPathFailed)
}

func TestGetPersonFilesReturnsQueryErrorWhenCloudBridgeServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAdvanceFilesTestRouter("/person/files", NewHandler(
		nil,
		newAdvanceValidCloudTokenService(),
	).GetPersonFiles())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/person/files?cloudToken=123&pageNum=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryPathFailed)
}

func TestGetFamilyFilesReturnsQueryErrorWhenCloudBridgeReturnsNilList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAdvanceFilesTestRouter("/family/files", NewHandler(
		&mockGetFilesCloudBridge{},
		newAdvanceValidCloudTokenService(),
	).GetFamilyFiles())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/family/files?cloudToken=123&familyId=family-1&pageNum=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryPathFailed)
}

func TestGetFamilyFilesReturnsQueryErrorWhenCloudBridgeServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAdvanceFilesTestRouter("/family/files", NewHandler(
		nil,
		newAdvanceValidCloudTokenService(),
	).GetFamilyFiles())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/family/files?cloudToken=123&familyId=family-1&pageNum=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryPathFailed)
}

func newAdvanceFilesTestRouter(
	route string,
	handler httpcontext.HandlerFunc,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET(route, wrapper.Wrap(handler))

	return router
}

func newAdvanceValidCloudTokenService() *mockAdvanceCloudTokenService {
	return &mockAdvanceCloudTokenService{
		token: &models.CloudToken{
			ID:          123,
			AccessToken: "access-token",
			ExpiresIn:   3600,
			CreatedAt:   time.Now(),
		},
	}
}
