package file

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

type mockDownloadVerifyService struct {
	verifySvi.Service
	fileID int64
}

func (m *mockDownloadVerifyService) VerifyV1(
	ctx appContext.Context,
	fileID int64,
	sign string,
	uuid string,
	timestamp string,
	signer string,
) error {
	m.fileID = fileID

	return nil
}

type mockDownloadVirtualFileService struct {
	virtualfileSvi.Service
	file    *models.VirtualFile
	queried int64
}

func (m *mockDownloadVirtualFileService) Query(ctx appContext.Context, fileID int64) (*models.VirtualFile, error) {
	m.queried = fileID

	return m.file, nil
}

type mockDownloadMountPointService struct {
	mountpointSvi.Service
	mountPoint *models.MountPoint
	queried    int64
}

func (m *mockDownloadMountPointService) Query(ctx appContext.Context, fileID int64) (*models.MountPoint, error) {
	m.queried = fileID

	return m.mountPoint, nil
}

type mockDownloadUserMountPointTokenService struct {
	userMountPointTokenSvi.Service
	anyTokens         map[int64]int64
	anyMountPointIDs  []int64
	getTokenCallCount int
}

func (m *mockDownloadUserMountPointTokenService) GetTokenID(ctx appContext.Context, userID, mountPointID int64) (int64, error) {
	m.getTokenCallCount++

	return 0, nil
}

func (m *mockDownloadUserMountPointTokenService) GetAnyUserTokens(ctx appContext.Context, mountPointIDs []int64) (map[int64]int64, error) {
	m.anyMountPointIDs = append([]int64(nil), mountPointIDs...)

	return m.anyTokens, nil
}

type mockDownloadCloudTokenService struct {
	cloudtokenSvi.Service
	token   *models.CloudToken
	queried int64
}

func (m *mockDownloadCloudTokenService) Query(ctx appContext.Context, id int64) (*models.CloudToken, error) {
	m.queried = id

	return m.token, nil
}

type mockDownloadCloudBridgeService struct {
	cloudbridgeSvi.Service
	link         string
	personFileID string
	shareID      int64
	shareFileID  string
}

func (m *mockDownloadCloudBridgeService) PersonDownloadLink(ctx appContext.Context, token cloudbridgeSvi.AuthToken, fileID string) (string, error) {
	m.personFileID = fileID

	return m.link, nil
}

func (m *mockDownloadCloudBridgeService) ShareDownloadLink(ctx appContext.Context, token cloudbridgeSvi.AuthToken, shareID int64, fileID string) (string, error) {
	m.shareID = shareID
	m.shareFileID = fileID

	return m.link, nil
}

func TestDownloadUsesAnyBoundTokenForPublicSignedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldSettingAddition := shared.SettingAddition
	shared.SettingAddition = models.SettingAddition{}

	defer func() {
		shared.SettingAddition = oldSettingAddition
	}()

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 0},
	}
	userTokenService := &mockDownloadUserMountPointTokenService{
		anyTokens: map[int64]int64{77: 555},
	}
	cloudTokenService := &mockDownloadCloudTokenService{
		token: &models.CloudToken{
			ID:          555,
			AccessToken: "access-token",
			ExpiresIn:   3600,
		},
	}
	cloudBridgeService := &mockDownloadCloudBridgeService{
		link: "https://download.example.test/file",
	}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		userTokenService,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if location := recorder.Header().Get("Location"); location != cloudBridgeService.link {
		t.Fatalf("expected redirect to %q, got %q", cloudBridgeService.link, location)
	}

	if verifyService.fileID != 99 {
		t.Fatalf("expected file 99 to be verified, got %d", verifyService.fileID)
	}

	if userTokenService.getTokenCallCount != 0 {
		t.Fatalf("expected public signed URL not to query current user token, got %d calls", userTokenService.getTokenCallCount)
	}

	if len(userTokenService.anyMountPointIDs) != 1 || userTokenService.anyMountPointIDs[0] != 77 {
		t.Fatalf("expected any-token lookup for mount point 77, got %v", userTokenService.anyMountPointIDs)
	}

	if cloudTokenService.queried != 555 {
		t.Fatalf("expected cloud token 555 to be queried, got %d", cloudTokenService.queried)
	}

	if cloudBridgeService.personFileID != "cloud-file" {
		t.Fatalf("expected person download link for cloud-file, got %q", cloudBridgeService.personFileID)
	}
}

func TestDownloadUsesMountPointTokenForShareFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldSettingAddition := shared.SettingAddition
	shared.SettingAddition = models.SettingAddition{}

	defer func() {
		shared.SettingAddition = oldSettingAddition
	}()

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "share-file",
			OsType:  models.OsTypeShareFile,
			Addition: datatypes.JSONMap{
				consts.FileAdditionKeyShareId: int64(12345),
			},
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}
	cloudTokenService := &mockDownloadCloudTokenService{
		token: &models.CloudToken{
			ID:          555,
			AccessToken: "access-token",
			ExpiresIn:   3600,
		},
	}
	cloudBridgeService := &mockDownloadCloudBridgeService{
		link: "https://download.example.test/share-file",
	}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if location := recorder.Header().Get("Location"); location != cloudBridgeService.link {
		t.Fatalf("expected redirect to %q, got %q", cloudBridgeService.link, location)
	}

	if cloudTokenService.queried != 555 {
		t.Fatalf("expected cloud token 555 to be queried, got %d", cloudTokenService.queried)
	}

	if cloudBridgeService.shareID != 12345 || cloudBridgeService.shareFileID != "share-file" {
		t.Fatalf("expected share download link for share 12345 file share-file, got share=%d file=%q", cloudBridgeService.shareID, cloudBridgeService.shareFileID)
	}
}

func TestDownloadTreatsNilCloudTokenAsUnbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}
	cloudTokenService := &mockDownloadCloudTokenService{}
	cloudBridgeService := &mockDownloadCloudBridgeService{
		link: "https://download.example.test/file",
	}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeTokenNotBind.GetCode() {
		t.Fatalf("expected token-not-bind code %d, got %d", busCodeTokenNotBind.GetCode(), response.Code)
	}

	if cloudTokenService.queried != 555 {
		t.Fatalf("expected cloud token 555 to be queried, got %d", cloudTokenService.queried)
	}

	if cloudBridgeService.personFileID != "" {
		t.Fatalf("expected nil token not to request remote download link, got %q", cloudBridgeService.personFileID)
	}
}

func TestDownloadReturnsVerifyErrorWhenVerifyServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}

	router := newDownloadTestRouter(
		virtualFileService,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileVerifyError)

	if virtualFileService.queried != 0 {
		t.Fatalf("expected missing verify service to stop before file query, got file id %d", virtualFileService.queried)
	}
}

func TestDownloadReturnsQueryErrorWhenVirtualFileServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}

	router := newDownloadTestRouter(
		nil,
		verifyService,
		nil,
		nil,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileQueryError)

	if verifyService.fileID != 99 {
		t.Fatalf("expected file 99 to be verified before file lookup, got %d", verifyService.fileID)
	}
}

func TestDownloadReturnsNotFoundWhenFileQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{}
	cloudTokenService := &mockDownloadCloudTokenService{}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		nil,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if verifyService.fileID != 99 {
		t.Fatalf("expected signed file 99 to be verified, got %d", verifyService.fileID)
	}

	if cloudTokenService.queried != 0 {
		t.Fatalf("expected nil file not to query token, got token id %d", cloudTokenService.queried)
	}
}

func TestDownloadReturnsQueryErrorWhenMountPointServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	cloudTokenService := &mockDownloadCloudTokenService{}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		nil,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileQueryError)

	if cloudTokenService.queried != 0 {
		t.Fatalf("expected missing mount point service not to query token, got token id %d", cloudTokenService.queried)
	}
}

func TestDownloadReturnsQueryErrorWhenMountPointQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{}
	cloudTokenService := &mockDownloadCloudTokenService{}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		nil,
		mountPointService,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeFileQueryError.GetCode() {
		t.Fatalf("expected file query code %d, got %d", busCodeFileQueryError.GetCode(), response.Code)
	}

	if cloudTokenService.queried != 0 {
		t.Fatalf("expected nil mount point not to query token, got token id %d", cloudTokenService.queried)
	}
}

func TestDownloadReturnsTokenQueryErrorWhenCloudTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}
	cloudBridgeService := &mockDownloadCloudBridgeService{
		link: "https://download.example.test/file",
	}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		nil,
		cloudBridgeService,
		mountPointService,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeTokenQueryError)

	if cloudBridgeService.personFileID != "" {
		t.Fatalf("expected missing token service not to request remote download link, got %q", cloudBridgeService.personFileID)
	}
}

func TestDownloadReturnsLinkErrorWhenCloudBridgeServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}
	cloudTokenService := &mockDownloadCloudTokenService{
		token: &models.CloudToken{
			ID:          555,
			AccessToken: "access-token",
			ExpiresIn:   3600,
		},
	}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		nil,
		mountPointService,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeGetDownloadLinkError)

	if cloudTokenService.queried != 555 {
		t.Fatalf("expected cloud token 555 to be queried before remote link lookup, got %d", cloudTokenService.queried)
	}
}

func TestDownloadReturnsVerifyErrorWhenVerifyServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var verifyService *mockDownloadVerifyService

	virtualFileService := &mockDownloadVirtualFileService{}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		nil,
		nil,
		nil,
		nil,
	)

	recorder := performDownloadTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileVerifyError)

	if virtualFileService.queried != 0 {
		t.Fatalf("expected typed nil verify service to stop before file query, got file id %d", virtualFileService.queried)
	}
}

func TestDownloadReturnsQueryErrorWhenVirtualFileServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}

	var virtualFileService *mockDownloadVirtualFileService

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		nil,
		nil,
		nil,
		nil,
	)

	recorder := performDownloadTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileQueryError)

	if verifyService.fileID != 99 {
		t.Fatalf("expected file 99 to be verified before file lookup, got %d", verifyService.fileID)
	}
}

func TestDownloadReturnsQueryErrorWhenMountPointServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}

	var mountPointService *mockDownloadMountPointService

	cloudTokenService := &mockDownloadCloudTokenService{}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		nil,
		mountPointService,
		nil,
	)

	recorder := performDownloadTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileQueryError)

	if cloudTokenService.queried != 0 {
		t.Fatalf("expected typed nil mount point service not to query token, got token id %d", cloudTokenService.queried)
	}
}

func TestDownloadSkipsTypedNilOptionalUserMountPointTokenService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}

	var userTokenService *mockDownloadUserMountPointTokenService

	cloudTokenService := &mockDownloadCloudTokenService{
		token: &models.CloudToken{
			ID:          555,
			AccessToken: "access-token",
			ExpiresIn:   3600,
		},
	}
	cloudBridgeService := &mockDownloadCloudBridgeService{
		link: "https://download.example.test/file",
	}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		userTokenService,
	)

	recorder := performDownloadTestRequest(t, router)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if cloudTokenService.queried != 555 {
		t.Fatalf("expected mount point token 555 to be queried, got %d", cloudTokenService.queried)
	}

	if cloudBridgeService.personFileID != "cloud-file" {
		t.Fatalf("expected person download link for cloud-file, got %q", cloudBridgeService.personFileID)
	}
}

func TestDownloadReturnsTokenQueryErrorWhenCloudTokenServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}

	var cloudTokenService *mockDownloadCloudTokenService

	cloudBridgeService := &mockDownloadCloudBridgeService{}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		nil,
	)

	recorder := performDownloadTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeTokenQueryError)

	if cloudBridgeService.personFileID != "" {
		t.Fatalf("expected typed nil token service not to request remote download link, got %q", cloudBridgeService.personFileID)
	}
}

func TestDownloadReturnsLinkErrorWhenCloudBridgeServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}
	cloudTokenService := &mockDownloadCloudTokenService{
		token: &models.CloudToken{
			ID:          555,
			AccessToken: "access-token",
			ExpiresIn:   3600,
		},
	}

	var cloudBridgeService *mockDownloadCloudBridgeService

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		nil,
	)

	recorder := performDownloadTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeGetDownloadLinkError)

	if cloudTokenService.queried != 555 {
		t.Fatalf("expected cloud token 555 to be queried before remote link lookup, got %d", cloudTokenService.queried)
	}
}

func TestDownloadRejectsInvalidFileIDBeforeVerifyingSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyService := &mockDownloadVerifyService{}
	router := newDownloadTestRouter(
		nil,
		verifyService,
		nil,
		nil,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/-1?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if verifyService.fileID != 0 {
		t.Fatalf("expected invalid file id not to be verified, got %d", verifyService.fileID)
	}
}

func TestDownloadAllowsSuffixLimitedFileAfterSignatureVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldSettingAddition := shared.SettingAddition
	shared.SettingAddition = models.SettingAddition{
		WebDAVUserStrmOnly:    true,
		WebDAVAllowedSuffixes: []string{".mp4"},
	}

	defer func() {
		shared.SettingAddition = oldSettingAddition
	}()

	verifyService := &mockDownloadVerifyService{}
	virtualFileService := &mockDownloadVirtualFileService{
		file: &models.VirtualFile{
			ID:      99,
			TopId:   300,
			CloudId: "cloud-file",
			Name:    "private.txt",
			OsType:  models.OsTypePersonFile,
		},
	}
	mountPointService := &mockDownloadMountPointService{
		mountPoint: &models.MountPoint{ID: 77, FileId: 300, TokenId: 555},
	}
	cloudTokenService := &mockDownloadCloudTokenService{
		token: &models.CloudToken{
			ID:          555,
			AccessToken: "access-token",
			ExpiresIn:   3600,
		},
	}
	cloudBridgeService := &mockDownloadCloudBridgeService{
		link: "https://download.example.test/file",
	}

	router := newDownloadTestRouter(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		nil,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if location := recorder.Header().Get("Location"); location != cloudBridgeService.link {
		t.Fatalf("expected redirect to %q, got %q", cloudBridgeService.link, location)
	}

	if verifyService.fileID != 99 {
		t.Fatalf("expected signed file 99 to be verified, got %d", verifyService.fileID)
	}

	if cloudTokenService.queried != 555 {
		t.Fatalf("expected suffix-limited signed URL to query token 555, got token id %d", cloudTokenService.queried)
	}

	if cloudBridgeService.personFileID != "cloud-file" {
		t.Fatalf("expected suffix-limited signed URL to request remote download link, got %q", cloudBridgeService.personFileID)
	}
}

func newDownloadTestRouter(
	virtualFileService virtualfileSvi.Service,
	verifyService verifySvi.Service,
	cloudTokenService cloudtokenSvi.Service,
	cloudBridgeService cloudbridgeSvi.Service,
	mountPointService mountpointSvi.Service,
	userMountPointTokenService userMountPointTokenSvi.Service,
) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/download/:fileId", wrapper.Wrap(NewHandler(
		virtualFileService,
		verifyService,
		cloudTokenService,
		cloudBridgeService,
		mountPointService,
		nil,
		userMountPointTokenService,
		nil,
	).Download()))

	return router
}

func performDownloadTestRequest(t *testing.T, router *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/download/99?sign=signed&uuid=uuid-value&timestamp=1234567890&signer=v1",
		nil,
	)
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestCopyOptimizedHeadersDoesNotForwardClientAuthorization(t *testing.T) {
	src := http.Header{}
	src.Set("Authorization", "Bearer user-session-token")
	src.Set("Cookie", "session=secret")
	src.Set("Content-Type", "application/json")
	src.Set("Range", "bytes=0-1023")
	src.Set("User-Agent", "cloudpan-test")
	src.Set("Accept", "video/mp4")

	dst := http.Header{}
	new(handler).copyOptimizedHeaders(src, dst)

	if got := dst.Get("Authorization"); got != "" {
		t.Fatalf("expected Authorization not to be forwarded to download upstream, got %q", got)
	}

	if got := dst.Get("Cookie"); got != "" {
		t.Fatalf("expected Cookie not to be forwarded to download upstream, got %q", got)
	}

	if got := dst.Get("Content-Type"); got != "" {
		t.Fatalf("expected Content-Type not to be forwarded to download upstream, got %q", got)
	}

	if got := dst.Get("Range"); got != "bytes=0-1023" {
		t.Fatalf("expected Range to be forwarded, got %q", got)
	}

	if got := dst.Get("User-Agent"); got != "cloudpan-test" {
		t.Fatalf("expected User-Agent to be forwarded, got %q", got)
	}

	if got := dst.Get("Accept"); got != "video/mp4" {
		t.Fatalf("expected Accept to be forwarded, got %q", got)
	}
}

func TestSanitizeDownloadProxyErrorRedactsSensitiveValues(t *testing.T) {
	rawErr := errors.New(`GET "https://proxy-user:proxy-pass@download.example.test/file?sign=real-secret-sign&access_token=query-secret&filename=private-name.mkv#token=fragment-secret": accessCode=abcd Authorization: Bearer bearer-secret`)

	got := sanitizeDownloadProxyError(rawErr)

	if !strings.Contains(got, "download.example.test") {
		t.Fatalf("expected sanitized error to keep host context, got %q", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected sanitized error to contain redaction marker, got %q", got)
	}

	for _, secret := range []string{
		"proxy-user",
		"proxy-pass",
		"real-secret-sign",
		"query-secret",
		"private-name.mkv",
		"fragment-secret",
		"accessCode=abcd",
		"bearer-secret",
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("expected sanitized error not to contain %q, got %q", secret, got)
		}
	}
}

func TestSanitizeDownloadProxyErrorNil(t *testing.T) {
	if got := sanitizeDownloadProxyError(nil); got != "" {
		t.Fatalf("expected empty string for nil error, got %q", got)
	}
}

func TestLocalProxyHTTPClientUsesGlobalClientWhenProxyEmpty(t *testing.T) {
	client, err := localProxyHTTPClient("   ")
	if err != nil {
		t.Fatalf("expected empty proxy URL to be accepted, got %v", err)
	}

	if client != globalHTTPClient {
		t.Fatal("expected empty proxy URL to reuse global HTTP client")
	}
}

func TestLocalProxyHTTPTransportUsesConfiguredProxyURL(t *testing.T) {
	transport, err := localProxyHTTPTransport(" http://proxy-user:proxy-pass@127.0.0.1:7890 ")
	if err != nil {
		t.Fatalf("build local proxy transport: %v", err)
	}

	if transport == nil || transport.Proxy == nil {
		t.Fatal("expected configured proxy transport")
	}

	proxyURL, err := transport.Proxy(httptest.NewRequest(http.MethodGet, "https://download.example.test/file", nil))
	if err != nil {
		t.Fatalf("resolve proxy URL: %v", err)
	}

	want, err := url.Parse("http://proxy-user:proxy-pass@127.0.0.1:7890")
	if err != nil {
		t.Fatalf("parse expected proxy URL: %v", err)
	}

	if proxyURL.String() != want.String() {
		t.Fatalf("expected proxy URL %q, got %q", want, proxyURL)
	}

	if transport.MaxIdleConns != globalHTTPTransport.MaxIdleConns ||
		transport.MaxIdleConnsPerHost != globalHTTPTransport.MaxIdleConnsPerHost ||
		transport.MaxConnsPerHost != globalHTTPTransport.MaxConnsPerHost ||
		transport.TLSHandshakeTimeout != globalHTTPTransport.TLSHandshakeTimeout ||
		transport.DisableCompression != globalHTTPTransport.DisableCompression ||
		transport.ForceAttemptHTTP2 != globalHTTPTransport.ForceAttemptHTTP2 {
		t.Fatal("expected proxy transport to preserve optimized global transport settings")
	}
}

func TestLocalProxyHTTPTransportRejectsInvalidProxyURL(t *testing.T) {
	rawProxyURL := "http://proxy-user:proxy-pass@%zz?token=secret-token#session=secret-session"

	transport, err := localProxyHTTPTransport(rawProxyURL)
	if err == nil {
		t.Fatal("expected invalid proxy URL error")
	}

	if transport != nil {
		t.Fatalf("expected nil transport for invalid proxy URL, got %#v", transport)
	}

	sanitized := sanitizeDownloadProxyError(err)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-token", "secret-session"} {
		if strings.Contains(sanitized, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, sanitized)
		}
	}
}

func TestCopyOptimizedResponseHeadersFiltersProxyAndMultiStreamHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	src := http.Header{}
	src.Set("Content-Type", "video/mp4")
	src.Set("Content-Length", "1024")
	src.Set("Content-Disposition", `attachment; filename="movie.mp4"`)
	src.Set("Connection", "close")
	src.Set("Keep-Alive", "timeout=5")
	src.Set("Set-Cookie", "download-session=secret")

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/headers", wrapper.Wrap(func(ctx *httpcontext.Context) {
		new(handler).copyOptimizedResponseHeaders(src, ctx)
		ctx.Status(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/headers", nil)
	router.ServeHTTP(recorder, req)

	if got := recorder.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("expected Content-Type to be forwarded, got %q", got)
	}

	if got := recorder.Header().Get("Content-Length"); got != "1024" {
		t.Fatalf("expected Content-Length to be forwarded, got %q", got)
	}

	if got := recorder.Header().Get("Content-Disposition"); got != `attachment; filename="movie.mp4"` {
		t.Fatalf("expected Content-Disposition to be forwarded, got %q", got)
	}

	for _, header := range []string{"Connection", "Keep-Alive", "Set-Cookie"} {
		if got := recorder.Header().Get(header); got != "" {
			t.Fatalf("expected %s not to be forwarded or set, got %q", header, got)
		}
	}
}
