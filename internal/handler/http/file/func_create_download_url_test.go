package file

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

type mockCreateDownloadURLVirtualFileService struct {
	virtualfileSvi.Service
	file    *models.VirtualFile
	queried int64
}

func (m *mockCreateDownloadURLVirtualFileService) Query(ctx appContext.Context, fid int64) (*models.VirtualFile, error) {
	m.queried = fid

	return m.file, nil
}

type mockCreateDownloadURLMountPointService struct {
	mountpointSvi.Service
	userID        int64
	isAdmin       bool
	groupFileIDs  []int64
	accessibleIDs []int64
}

func (m *mockCreateDownloadURLMountPointService) GetAccessibleMountPointIDs(
	ctx appContext.Context,
	userID int64,
	isAdmin bool,
	groupFileIDs []int64,
) ([]int64, error) {
	m.userID = userID
	m.isAdmin = isAdmin
	m.groupFileIDs = groupFileIDs

	return m.accessibleIDs, nil
}

type mockCreateDownloadURLGroup2FileService struct {
	group2fileSvi.Service
	groupID int64
	fileIDs []int64
}

func (m *mockCreateDownloadURLGroup2FileService) GetBindFiles(ctx appContext.Context, groupID int64) ([]int64, error) {
	m.groupID = groupID

	return m.fileIDs, nil
}

type mockCreateDownloadURLVerifyService struct {
	verifySvi.Service
	signedFileID int64
	values       url.Values
}

func (m *mockCreateDownloadURLVerifyService) SignV1(ctx appContext.Context, fileId int64, opts ...verifySvi.SignV1OptionFunc) (url.Values, error) {
	m.signedFileID = fileId

	return m.values, nil
}

func TestCreateDownloadURLRejectsInaccessibleFileBeforeSigning(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "private.mp4"},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{200}}
	verifyService := &mockCreateDownloadURLVerifyService{}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		false,
		0,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if virtualFileService.queried != 99 {
		t.Fatalf("expected file 99 to be queried, got %d", virtualFileService.queried)
	}

	if mountPointService.userID != 100 || mountPointService.isAdmin {
		t.Fatalf("expected accessible lookup for non-admin user 100, got userID=%d isAdmin=%v", mountPointService.userID, mountPointService.isAdmin)
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected inaccessible file not to be signed, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLReturnsNotFoundWhenFileQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		false,
		0,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if virtualFileService.queried != 99 {
		t.Fatalf("expected file 99 to be queried, got %d", virtualFileService.queried)
	}

	if mountPointService.userID != 0 {
		t.Fatalf("expected nil file to stop before access lookup, got userID=%d", mountPointService.userID)
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected nil file not to be signed, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLReturnsErrorWhenGroupBindingServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "public.mp4"},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		nil,
		100,
		false,
		7,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeQueryTopIdError)

	if mountPointService.userID != 0 {
		t.Fatalf("expected missing group binding service to stop before access lookup, got userID=%d", mountPointService.userID)
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected missing group binding service not to sign file, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLReturnsErrorWhenGroupBindingServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "public.mp4"},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{}

	var group2FileService *mockCreateDownloadURLGroup2FileService

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		group2FileService,
		100,
		false,
		7,
	)

	recorder := performCreateDownloadURLTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeQueryTopIdError)

	if mountPointService.userID != 0 {
		t.Fatalf("expected typed nil group binding service to stop before access lookup, got userID=%d", mountPointService.userID)
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected typed nil group binding service not to sign file, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLReturnsQueryErrorWhenVirtualFileServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var virtualFileService *mockCreateDownloadURLVirtualFileService

	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		false,
		0,
	)

	recorder := performCreateDownloadURLTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileQueryError)

	if mountPointService.userID != 0 {
		t.Fatalf("expected typed nil virtual file service to stop before access lookup, got userID=%d", mountPointService.userID)
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected typed nil virtual file service not to sign file, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLReturnsQueryErrorWhenMountPointServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "public.mp4"},
	}

	var mountPointService *mockCreateDownloadURLMountPointService

	verifyService := &mockCreateDownloadURLVerifyService{}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		false,
		0,
	)

	recorder := performCreateDownloadURLTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeQueryTopIdError)

	if virtualFileService.queried != 99 {
		t.Fatalf("expected file 99 to be queried before access lookup, got %d", virtualFileService.queried)
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected typed nil mount point service not to sign file, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLReturnsSignErrorWhenVerifyServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "public.mp4"},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}

	var verifyService *mockCreateDownloadURLVerifyService

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		false,
		0,
	)

	recorder := performCreateDownloadURLTestRequest(t, router)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileSignError)

	if mountPointService.userID != 100 || mountPointService.isAdmin {
		t.Fatalf("expected access lookup before signing, got userID=%d isAdmin=%v", mountPointService.userID, mountPointService.isAdmin)
	}
}

func TestCreateDownloadURLRejectsDirectoryBeforeSigning(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "folder", IsDir: true},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		false,
		0,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected directory not to be signed, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLRejectsSuffixLimitedFileBeforeSigning(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldSettingAddition := shared.SettingAddition
	shared.SettingAddition = models.SettingAddition{
		WebDAVUserStrmOnly:    true,
		WebDAVAllowedSuffixes: []string{".mp4"},
	}

	defer func() {
		shared.SettingAddition = oldSettingAddition
	}()

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "private.txt"},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		false,
		0,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if verifyService.signedFileID != 0 {
		t.Fatalf("expected suffix-limited file not to be signed, got file id %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLAllowsAdminWhenSuffixLimitEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldSettingAddition := shared.SettingAddition
	shared.SettingAddition = models.SettingAddition{
		WebDAVUserStrmOnly:    true,
		WebDAVAllowedSuffixes: []string{".mp4"},
	}

	defer func() {
		shared.SettingAddition = oldSettingAddition
	}()

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "private.txt"},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{
		values: url.Values{
			"sign":      []string{"signed-value"},
			"uuid":      []string{"uuid-value"},
			"timestamp": []string{"1234567890"},
			"signer":    []string{"v1"},
		},
	}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		&mockCreateDownloadURLGroup2FileService{},
		100,
		true,
		0,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if verifyService.signedFileID != 99 {
		t.Fatalf("expected admin-visible file 99 to be signed, got %d", verifyService.signedFileID)
	}
}

func TestCreateDownloadURLSignsAccessibleFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockCreateDownloadURLVirtualFileService{
		file: &models.VirtualFile{ID: 99, TopId: 300, Name: "public.mp4"},
	}
	mountPointService := &mockCreateDownloadURLMountPointService{accessibleIDs: []int64{300}}
	verifyService := &mockCreateDownloadURLVerifyService{
		values: url.Values{
			"sign":      []string{"signed-value"},
			"uuid":      []string{"uuid-value"},
			"timestamp": []string{"1234567890"},
			"signer":    []string{"v1"},
		},
	}
	group2FileService := &mockCreateDownloadURLGroup2FileService{fileIDs: []int64{300}}

	router := newCreateDownloadURLTestRouter(
		virtualFileService,
		verifyService,
		mountPointService,
		group2FileService,
		100,
		false,
		7,
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if group2FileService.groupID != 7 {
		t.Fatalf("expected group 7 bind files lookup, got %d", group2FileService.groupID)
	}

	if len(mountPointService.groupFileIDs) != 1 || mountPointService.groupFileIDs[0] != 300 {
		t.Fatalf("expected group file ids [300], got %v", mountPointService.groupFileIDs)
	}

	if verifyService.signedFileID != 99 {
		t.Fatalf("expected file 99 to be signed, got %d", verifyService.signedFileID)
	}

	var response struct {
		Code int                       `json:"code"`
		Data createDownloadURLResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.DownloadURL == "" || !strings.Contains(response.Data.DownloadURL, "/api/file/download/99?") {
		t.Fatalf("expected download URL for file 99, got %q", response.Data.DownloadURL)
	}

	for _, value := range []string{"sign=signed-value", "uuid=uuid-value", "timestamp=1234567890", "signer=v1"} {
		if !strings.Contains(response.Data.DownloadURL, value) {
			t.Fatalf("expected download URL to contain %q, got %q", value, response.Data.DownloadURL)
		}
	}
}

func newCreateDownloadURLTestRouter(
	virtualFileService virtualfileSvi.Service,
	verifyService verifySvi.Service,
	mountPointService mountpointSvi.Service,
	group2FileService group2fileSvi.Service,
	userID int64,
	isAdmin bool,
	userGroupID int64,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, userID)
		ctx.Set(consts.CtxKeyIsAdmin, isAdmin)
		ctx.Set(consts.CtxKeyUserGroupId, userGroupID)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create_download_url", wrapper.Wrap(NewHandler(
		virtualFileService,
		verifyService,
		nil,
		nil,
		mountPointService,
		group2FileService,
		nil,
		nil,
	).CreateDownloadURL()))

	return router
}

func performCreateDownloadURLTestRequest(t *testing.T, router *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/create_download_url", strings.NewReader(`{"fileId":99}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	return recorder
}
