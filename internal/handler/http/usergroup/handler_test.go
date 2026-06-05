package usergroup

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	usergroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockUserGroupService struct {
	usergroupSvi.Service
	deleteErr     error
	modifyNameErr error
	queryErr      error
	returnNil     bool
	list          []*models.UserGroup
	count         int64
}

func (m *mockUserGroupService) Delete(ctx appContext.Context, req *usergroupSvi.DeleteRequest) error {
	return m.deleteErr
}

func (m *mockUserGroupService) ModifyName(ctx appContext.Context, req *usergroupSvi.ModifyNameRequest) error {
	return m.modifyNameErr
}

func (m *mockUserGroupService) Query(ctx appContext.Context, gid int64) (*models.UserGroup, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	if m.returnNil {
		return nil, nil
	}

	return &models.UserGroup{ID: gid, Name: "test"}, nil
}

func (m *mockUserGroupService) List(ctx appContext.Context, req *usergroupSvi.ListRequest) ([]*models.UserGroup, error) {
	return m.list, nil
}

func (m *mockUserGroupService) Count(ctx appContext.Context, req *usergroupSvi.ListRequest) (int64, error) {
	return m.count, nil
}

type mockUserGroupBindFileService struct {
	group2fileSvi.Service
	batchBindCalls   int
	batchBindGroupID int64
	batchBindFileIDs []int64
	batchBindErr     error
	getBindCalls     int
}

func (m *mockUserGroupBindFileService) BatchBindFiles(ctx appContext.Context, groupID int64, fileIDs []int64) error {
	m.batchBindCalls++
	m.batchBindGroupID = groupID

	m.batchBindFileIDs = append([]int64(nil), fileIDs...)

	return m.batchBindErr
}

func (m *mockUserGroupBindFileService) GetBindFiles(ctx appContext.Context, groupID int64) ([]int64, error) {
	m.getBindCalls++

	return []int64{11, 22}, nil
}

type mockUserGroupListUserService struct {
	userSvi.Service
	counts   map[int64]int64
	groupIDs []int64
}

func (m *mockUserGroupListUserService) Count(ctx appContext.Context, req *userSvi.ListRequest) (int64, error) {
	if req == nil || req.GroupId == nil {
		return 0, nil
	}

	groupID := *req.GroupId
	m.groupIDs = append(m.groupIDs, groupID)

	return m.counts[groupID], nil
}

func newUserGroupTestRouter(
	userGroupService usergroupSvi.Service,
	group2FileService group2fileSvi.Service,
) *gin.Engine {
	return newUserGroupDependencyRouter(userGroupService, group2FileService, nil)
}

func newUserGroupDependencyRouter(
	userGroupService usergroupSvi.Service,
	group2FileService group2fileSvi.Service,
	userService userSvi.Service,
) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(userGroupService, group2FileService, userService)

	router.POST("/add", wrapper.Wrap(handler.Add()))
	router.POST("/delete", wrapper.Wrap(handler.Delete()))
	router.POST("/modify_name", wrapper.Wrap(handler.ModifyName()))
	router.POST("/batch_bind_files", wrapper.Wrap(handler.BatchBindFiles()))
	router.GET("/bind_files", wrapper.Wrap(handler.GetBindFiles()))
	router.GET("/list", wrapper.Wrap(handler.List()))

	return router
}

func assertUserGroupHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != expectedErr.GetCode() {
		t.Fatalf("expected business code %d, got %d", expectedErr.GetCode(), response.Code)
	}
}

func assertUserGroupNotFoundResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeUserGroupNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeUserGroupNotFound.GetCode(), response.Code)
	}

	if response.Msg != codeUserGroupNotFound.GetMessage() {
		t.Fatalf("expected message %q, got %q", codeUserGroupNotFound.GetMessage(), response.Msg)
	}
}

func assertBindFileNotFoundResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeBindFileNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeBindFileNotFound.GetCode(), response.Code)
	}

	if response.Msg != codeBindFileNotFound.GetMessage() {
		t.Fatalf("expected message %q, got %q", codeBindFileNotFound.GetMessage(), response.Msg)
	}
}

func TestAddReturnsErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"name":"test"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(nil, nil, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeAddUserGroupFailed)
}

func TestAddReturnsErrorWhenUserGroupServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var userGroupService *mockUserGroupService

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"name":"test"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(userGroupService, nil, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeAddUserGroupFailed)
}

func TestDeleteReturnsErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/delete",
		strings.NewReader(`{"id":1}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(nil, nil, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeDeleteUserGroupFailed)
}

func TestModifyNameReturnsErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/modify_name",
		strings.NewReader(`{"id":1,"name":"new-name"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(nil, nil, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeModifyNameFailed)
}

func TestBatchBindFilesReturnsErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":1,"fileIds":[11]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(nil, group2FileService, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeBatchBindFilesFailed)

	if group2FileService.batchBindCalls != 0 {
		t.Fatalf("expected group binding service not called, got %d calls", group2FileService.batchBindCalls)
	}
}

func TestBatchBindFilesReturnsErrorWhenGroupBindingServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":1,"fileIds":[11]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(&mockUserGroupService{}, nil, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeBatchBindFilesFailed)
}

func TestBatchBindFilesReturnsErrorWhenGroupBindingServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var group2FileService *mockUserGroupBindFileService

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":1,"fileIds":[11]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(&mockUserGroupService{}, group2FileService, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeBatchBindFilesFailed)
}

func TestGetBindFilesReturnsErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/bind_files?groupId=1", nil)

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(nil, group2FileService, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeGetBindFilesFailed)

	if group2FileService.getBindCalls != 0 {
		t.Fatalf("expected bind file lookup not called, got %d calls", group2FileService.getBindCalls)
	}
}

func TestGetBindFilesReturnsErrorWhenGroupBindingServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/bind_files?groupId=1", nil)

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(&mockUserGroupService{}, nil, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeGetBindFilesFailed)
}

func TestListReturnsErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(nil, nil, &mockUserGroupListUserService{}).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeListUserGroupFailed)
}

func TestListReturnsErrorWhenUserServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(&mockUserGroupService{}, nil, nil).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeListUserGroupFailed)
}

func TestListReturnsErrorWhenUserServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var userService *mockUserGroupListUserService

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)

	recorder := httptest.NewRecorder()
	newUserGroupDependencyRouter(&mockUserGroupService{}, nil, userService).ServeHTTP(recorder, req)

	assertUserGroupHTTPError(t, recorder, http.StatusBadRequest, codeListUserGroupFailed)
}

func TestListSkipsNilUserGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userService := &mockUserGroupListUserService{
		counts: map[int64]int64{
			1: 3,
			2: 5,
		},
	}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		&mockUserGroupService{
			list: []*models.UserGroup{
				nil,
				{ID: 1, Name: "default"},
				nil,
				{ID: 2, Name: "vip"},
			},
		},
		&mockUserGroupBindFileService{},
		userService,
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?noPaginate=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			Total int64 `json:"total"`
			Data  []struct {
				ID        int64  `json:"id"`
				Name      string `json:"name"`
				UserCount int64  `json:"userCount"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 2 || len(response.Data.Data) != 2 {
		t.Fatalf("expected two non-nil user groups, got total=%d len=%d", response.Data.Total, len(response.Data.Data))
	}

	if len(userService.groupIDs) != 2 || userService.groupIDs[0] != 1 || userService.groupIDs[1] != 2 {
		t.Fatalf("expected user counts for groups [1 2], got %v", userService.groupIDs)
	}

	if response.Data.Data[0].ID != 1 || response.Data.Data[0].UserCount != 3 {
		t.Fatalf("unexpected first group: %+v", response.Data.Data[0])
	}

	if response.Data.Data[1].ID != 2 || response.Data.Data[1].UserCount != 5 {
		t.Fatalf("unexpected second group: %+v", response.Data.Data[1])
	}
}

func TestDeleteReturnsNotFoundWhenUserGroupMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserGroupTestRouter(
		&mockUserGroupService{deleteErr: gorm.ErrRecordNotFound},
		&mockUserGroupBindFileService{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/delete",
		strings.NewReader(`{"id":999}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserGroupNotFoundResponse(t, recorder)
}

func TestModifyNameReturnsNotFoundWhenUserGroupMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserGroupTestRouter(
		&mockUserGroupService{modifyNameErr: gorm.ErrRecordNotFound},
		&mockUserGroupBindFileService{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/modify_name",
		strings.NewReader(`{"id":999,"name":"missing"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserGroupNotFoundResponse(t, recorder)
}

func TestBatchBindFilesReturnsNotFoundWhenUserGroupMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	router := newUserGroupTestRouter(
		&mockUserGroupService{queryErr: gorm.ErrRecordNotFound},
		group2FileService,
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":999,"fileIds":[11,22]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserGroupNotFoundResponse(t, recorder)

	if group2FileService.batchBindCalls != 0 {
		t.Fatalf("expected group binding service not called, got %d calls", group2FileService.batchBindCalls)
	}
}

func TestBatchBindFilesReturnsNotFoundWhenUserGroupQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	router := newUserGroupTestRouter(
		&mockUserGroupService{returnNil: true},
		group2FileService,
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":999,"fileIds":[11,22]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserGroupNotFoundResponse(t, recorder)

	if group2FileService.batchBindCalls != 0 {
		t.Fatalf("expected group binding service not called, got %d calls", group2FileService.batchBindCalls)
	}
}

func TestGetBindFilesReturnsNotFoundWhenUserGroupMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	router := newUserGroupTestRouter(
		&mockUserGroupService{queryErr: gorm.ErrRecordNotFound},
		group2FileService,
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/bind_files?groupId=999",
		nil,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserGroupNotFoundResponse(t, recorder)

	if group2FileService.getBindCalls != 0 {
		t.Fatalf("expected bind file lookup not called, got %d calls", group2FileService.getBindCalls)
	}
}

func TestGetBindFilesReturnsNotFoundWhenUserGroupQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	router := newUserGroupTestRouter(
		&mockUserGroupService{returnNil: true},
		group2FileService,
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodGet,
		"/bind_files?groupId=999",
		nil,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserGroupNotFoundResponse(t, recorder)

	if group2FileService.getBindCalls != 0 {
		t.Fatalf("expected bind file lookup not called, got %d calls", group2FileService.getBindCalls)
	}
}

func TestBatchBindFilesReturnsGenericFailureForQueryErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	router := newUserGroupTestRouter(
		&mockUserGroupService{queryErr: errors.New("db down")},
		group2FileService,
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":1,"fileIds":[11,22]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if group2FileService.batchBindCalls != 0 {
		t.Fatalf("expected group binding service not called, got %d calls", group2FileService.batchBindCalls)
	}
}

func TestBatchBindFilesReturnsNotFoundWhenBoundFileMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{batchBindErr: gorm.ErrRecordNotFound}
	router := newUserGroupTestRouter(
		&mockUserGroupService{},
		group2FileService,
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":1,"fileIds":[999]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertBindFileNotFoundResponse(t, recorder)

	if group2FileService.batchBindCalls != 1 {
		t.Fatalf("expected group binding service called once, got %d calls", group2FileService.batchBindCalls)
	}
}

func TestBatchBindFilesAllowsEmptyFileIDsForClearingBindings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group2FileService := &mockUserGroupBindFileService{}
	router := newUserGroupTestRouter(
		&mockUserGroupService{},
		group2FileService,
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_bind_files",
		strings.NewReader(`{"groupId":1,"fileIds":[]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if group2FileService.batchBindCalls != 1 {
		t.Fatalf("expected group binding service called once, got %d calls", group2FileService.batchBindCalls)
	}

	if group2FileService.batchBindGroupID != 1 {
		t.Fatalf("expected group id 1, got %d", group2FileService.batchBindGroupID)
	}

	if len(group2FileService.batchBindFileIDs) != 0 {
		t.Fatalf("expected empty file ids for clearing bindings, got %v", group2FileService.batchBindFileIDs)
	}
}
