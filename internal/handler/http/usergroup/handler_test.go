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
	usergroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockUserGroupService struct {
	usergroupSvi.Service
	deleteErr error
	queryErr  error
}

func (m *mockUserGroupService) Delete(ctx appContext.Context, req *usergroupSvi.DeleteRequest) error {
	return m.deleteErr
}

func (m *mockUserGroupService) Query(ctx appContext.Context, gid int64) (*models.UserGroup, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	return &models.UserGroup{ID: gid, Name: "test"}, nil
}

type mockUserGroupBindFileService struct {
	group2fileSvi.Service
	batchBindCalls int
	getBindCalls   int
}

func (m *mockUserGroupBindFileService) BatchBindFiles(ctx appContext.Context, groupID int64, fileIDs []int64) error {
	m.batchBindCalls++

	return nil
}

func (m *mockUserGroupBindFileService) GetBindFiles(ctx appContext.Context, groupID int64) ([]int64, error) {
	m.getBindCalls++

	return []int64{11, 22}, nil
}

func newUserGroupTestRouter(
	userGroupService usergroupSvi.Service,
	group2FileService group2fileSvi.Service,
) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(userGroupService, group2FileService, nil)

	router.POST("/delete", wrapper.Wrap(handler.Delete()))
	router.POST("/batch_bind_files", wrapper.Wrap(handler.BatchBindFiles()))
	router.GET("/bind_files", wrapper.Wrap(handler.GetBindFiles()))

	return router
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
