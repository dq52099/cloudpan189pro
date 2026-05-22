package storage

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"go.uber.org/zap"
)

type mockListMountPointService struct {
	mountPointSvi.Service
	list         []*models.MountPoint
	listCalls    int
	countCalls   int
	lastListReq  *mountPointSvi.ListRequest
	lastCountReq *mountPointSvi.ListRequest
}

func (m *mockListMountPointService) List(ctx appContext.Context, req *mountPointSvi.ListRequest) ([]*models.MountPoint, error) {
	m.listCalls++
	m.lastListReq = cloneMountPointListRequest(req)

	return m.list, nil
}

func (m *mockListMountPointService) Count(ctx appContext.Context, req *mountPointSvi.ListRequest) (int64, error) {
	m.countCalls++
	m.lastCountReq = cloneMountPointListRequest(req)

	return int64(len(m.list)), nil
}

type mockListCloudTokenService struct {
	cloudtokenSvi.Service
	called  bool
	list    []*models.CloudToken
	lastReq *cloudtokenSvi.ListRequest
}

func (m *mockListCloudTokenService) List(ctx appContext.Context, req *cloudtokenSvi.ListRequest) ([]*models.CloudToken, error) {
	m.called = true

	if req != nil {
		cp := *req
		cp.IdList = append([]int64(nil), req.IdList...)
		m.lastReq = &cp
	}

	return m.list, nil
}

type mockListFileTaskLogService struct {
	filetasklogSvi.Service
	list           []*models.FileTaskLog
	listCalls      int
	lastListReq    *filetasklogSvi.ListRequest
	latestFileIDs  []int64
	latestErr      error
	latestCalls    int
	latestStatuses []string
}

func (m *mockListFileTaskLogService) List(ctx appContext.Context, req *filetasklogSvi.ListRequest) ([]*models.FileTaskLog, error) {
	m.listCalls++
	m.lastListReq = cloneFileTaskLogListRequest(req)

	return m.list, nil
}

func (m *mockListFileTaskLogService) ListLatestFileIDsByStatus(ctx appContext.Context, status string) ([]int64, error) {
	m.latestCalls++
	m.latestStatuses = append(m.latestStatuses, status)

	if m.latestErr != nil {
		return nil, m.latestErr
	}

	return append([]int64(nil), m.latestFileIDs...), nil
}

type mockListVirtualFileService struct {
	virtualfileSvi.Service
	counts          []*virtualfileSvi.GroupCountByTopId
	groupCountCalls int
	lastCountReq    *virtualfileSvi.GroupCountByTopIdRequest
	err             error
}

func (m *mockListVirtualFileService) GroupCountByTopId(ctx appContext.Context, req *virtualfileSvi.GroupCountByTopIdRequest) ([]*virtualfileSvi.GroupCountByTopId, error) {
	m.groupCountCalls++
	m.lastCountReq = cloneGroupCountByTopIdRequest(req)

	if m.err != nil {
		return nil, m.err
	}

	return m.counts, nil
}

type mockListUserMountPointTokenService struct {
	userMountPointTokenSvi.Service
	err    error
	tokens map[int64]int64
}

func (m *mockListUserMountPointTokenService) GetUserTokens(ctx appContext.Context, userID int64, mountPointIDs []int64) (map[int64]int64, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.tokens == nil {
		return map[int64]int64{}, nil
	}

	result := make(map[int64]int64, len(m.tokens))
	for mountPointID, tokenID := range m.tokens {
		result[mountPointID] = tokenID
	}

	return result, nil
}

type mockListGroup2FileService struct {
	group2fileSvi.Service
	err error
}

func (m *mockListGroup2FileService) GetBindFiles(ctx appContext.Context, groupID int64) ([]int64, error) {
	if m.err != nil {
		return nil, m.err
	}

	return []int64{}, nil
}

func cloneMountPointListRequest(req *mountPointSvi.ListRequest) *mountPointSvi.ListRequest {
	if req == nil {
		return nil
	}

	cp := *req
	cp.GroupFileIds = append([]int64(nil), req.GroupFileIds...)
	cp.FileIdList = append([]int64(nil), req.FileIdList...)

	return &cp
}

func cloneFileTaskLogListRequest(req *filetasklogSvi.ListRequest) *filetasklogSvi.ListRequest {
	if req == nil {
		return nil
	}

	cp := *req
	cp.FileIdList = append([]int64(nil), req.FileIdList...)

	return &cp
}

func cloneGroupCountByTopIdRequest(req *virtualfileSvi.GroupCountByTopIdRequest) *virtualfileSvi.GroupCountByTopIdRequest {
	if req == nil {
		return nil
	}

	cp := *req
	cp.TopIdList = append([]int64(nil), req.TopIdList...)

	return &cp
}

func requireInt64SliceEqual(t *testing.T, name string, got, want []int64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s length mismatch: got %v want %v", name, got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s mismatch: got %v want %v", name, got, want)
		}
	}
}

func requireTimeEqual(t *testing.T, name string, got *time.Time, want time.Time) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s is nil, want %s", name, want.Format(time.RFC3339))
	}

	if !got.Equal(want) {
		t.Fatalf("%s mismatch: got %s want %s", name, got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestNextAutoRefreshTime(t *testing.T) {
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	beginAt := now.Add(-2 * time.Hour)
	updatedAt := now.Add(-20 * time.Minute)
	futureBeginAt := now.Add(90 * time.Minute)
	expiredBeginAt := now.AddDate(0, 0, -8)

	tests := []struct {
		name string
		item *models.MountPoint
		want *time.Time
	}{
		{
			name: "disabled",
			item: &models.MountPoint{
				EnableAutoRefresh:  false,
				AutoRefreshBeginAt: &beginAt,
				AutoRefreshDays:    7,
				RefreshInterval:    30,
			},
		},
		{
			name: "future begin",
			item: &models.MountPoint{
				EnableAutoRefresh:  true,
				AutoRefreshBeginAt: &futureBeginAt,
				AutoRefreshDays:    7,
				RefreshInterval:    30,
			},
			want: &futureBeginAt,
		},
		{
			name: "uses updated at as last refresh marker",
			item: &models.MountPoint{
				EnableAutoRefresh:  true,
				AutoRefreshBeginAt: &beginAt,
				AutoRefreshDays:    7,
				RefreshInterval:    30,
				UpdatedAt:          updatedAt,
			},
			want: timePtr(updatedAt.Add(30 * time.Minute)),
		},
		{
			name: "advances stale schedule into the future",
			item: &models.MountPoint{
				EnableAutoRefresh:  true,
				AutoRefreshBeginAt: &beginAt,
				AutoRefreshDays:    7,
				RefreshInterval:    45,
			},
			want: timePtr(beginAt.Add(3 * 45 * time.Minute)),
		},
		{
			name: "expired",
			item: &models.MountPoint{
				EnableAutoRefresh:  true,
				AutoRefreshBeginAt: &expiredBeginAt,
				AutoRefreshDays:    7,
				RefreshInterval:    30,
			},
		},
		{
			name: "non-positive days stay compatible with scheduler query",
			item: &models.MountPoint{
				EnableAutoRefresh:  true,
				AutoRefreshBeginAt: &beginAt,
				AutoRefreshDays:    0,
				RefreshInterval:    30,
			},
			want: timePtr(beginAt.Add(5 * 30 * time.Minute)),
		},
		{
			name: "invalid interval",
			item: &models.MountPoint{
				EnableAutoRefresh:  true,
				AutoRefreshBeginAt: &beginAt,
				AutoRefreshDays:    7,
				RefreshInterval:    0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextAutoRefreshTime(tt.item, now)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil next refresh time, got %s", got.Format(time.RFC3339))
				}

				return
			}

			requireTimeEqual(t, "next refresh time", got, *tt.want)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestListFileCountLookupUsesCurrentPageFileIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockListVirtualFileService{
		counts: []*virtualfileSvi.GroupCountByTopId{
			{TopId: 1001, Count: 2},
			{TopId: 1002, Count: 5},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		nil,
		virtualFileService,
		nil,
		&mockListCloudTokenService{},
		&mockListMountPointService{list: []*models.MountPoint{
			{
				ID:            1,
				FileId:        1001,
				Name:          "movies",
				FullPath:      "/movies",
				TokenId:       0,
				CreatorUserID: 10,
			},
			{
				ID:            2,
				FileId:        1002,
				Name:          "shows",
				FullPath:      "/shows",
				TokenId:       0,
				CreatorUserID: 10,
			},
			{
				ID:            3,
				FileId:        1001,
				Name:          "movies-copy",
				FullPath:      "/movies-copy",
				TokenId:       0,
				CreatorUserID: 10,
			},
		}},
		&mockListFileTaskLogService{},
		nil,
		nil,
		&mockListGroup2FileService{},
		&mockListUserMountPointTokenService{},
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if virtualFileService.groupCountCalls != 1 {
		t.Fatalf("expected one file count lookup, got %d", virtualFileService.groupCountCalls)
	}

	if virtualFileService.lastCountReq == nil {
		t.Fatal("expected file count request")
	}

	requireInt64SliceEqual(t, "file count top ids", virtualFileService.lastCountReq.TopIdList, []int64{1001, 1002})

	var response struct {
		Code int          `json:"code"`
		Data listResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if len(response.Data.Data) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(response.Data.Data))
	}

	if response.Data.Data[0].ID != 1001 || response.Data.Data[0].MountPointID != 1 {
		t.Fatalf("expected id to remain file id and mountPointId to expose row id, got id=%d mountPointId=%d", response.Data.Data[0].ID, response.Data.Data[0].MountPointID)
	}

	gotCounts := []int64{
		response.Data.Data[0].FileCount,
		response.Data.Data[1].FileCount,
		response.Data.Data[2].FileCount,
	}
	wantCounts := []int64{2, 5, 2}

	for i := range wantCounts {
		if gotCounts[i] != wantCounts[i] {
			t.Fatalf("unexpected file counts: got %v want %v", gotCounts, wantCounts)
		}
	}
}

func TestListSkipsCloudTokenLookupWhenNoTokenIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockListCloudTokenService{}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		nil,
		&mockListVirtualFileService{},
		nil,
		cloudTokenService,
		&mockListMountPointService{list: []*models.MountPoint{{
			ID:            1,
			FileId:        1001,
			Name:          "movies",
			FullPath:      "/movies",
			TokenId:       0,
			CreatorUserID: 10,
		}}},
		&mockListFileTaskLogService{},
		nil,
		nil,
		&mockListGroup2FileService{},
		&mockListUserMountPointTokenService{},
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if cloudTokenService.called {
		t.Fatal("did not expect cloud token list lookup without token IDs")
	}

	var response struct {
		Code int          `json:"code"`
		Data listResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 1 || len(response.Data.Data) != 1 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
}

func TestListCloudTokenLookupUsesCurrentUserScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockListCloudTokenService{
		list: []*models.CloudToken{{
			ID:     3001,
			UserID: 10,
			Name:   "own-token",
		}},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		nil,
		&mockListVirtualFileService{},
		nil,
		cloudTokenService,
		&mockListMountPointService{list: []*models.MountPoint{{
			ID:            1,
			FileId:        1001,
			Name:          "movies",
			FullPath:      "/movies",
			TokenId:       0,
			CreatorUserID: 10,
		}}},
		&mockListFileTaskLogService{},
		nil,
		nil,
		&mockListGroup2FileService{},
		&mockListUserMountPointTokenService{tokens: map[int64]int64{1: 3001}},
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if cloudTokenService.lastReq == nil {
		t.Fatal("expected cloud token list lookup")
	}

	requireInt64SliceEqual(t, "cloud token ids", cloudTokenService.lastReq.IdList, []int64{3001})

	if cloudTokenService.lastReq.UserID != 10 || cloudTokenService.lastReq.IsAdmin {
		t.Fatalf("expected current non-admin user scope, got userID=%d isAdmin=%v", cloudTokenService.lastReq.UserID, cloudTokenService.lastReq.IsAdmin)
	}

	var response struct {
		Code int          `json:"code"`
		Data listResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if len(response.Data.Data) != 1 || response.Data.Data[0].TokenName != "own-token" {
		t.Fatalf("expected own token name in response, got %+v", response.Data.Data)
	}
}

func TestListTaskLogStatusUsesLatestFileIDsForDatabasePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockListMountPointService{list: []*models.MountPoint{{
		ID:            7,
		FileId:        1002,
		Name:          "movies",
		FullPath:      "/media/movies",
		TokenId:       0,
		CreatorUserID: 10,
	}}}
	fileTaskLogService := &mockListFileTaskLogService{
		latestFileIDs: []int64{1002, 1003},
		list: []*models.FileTaskLog{{
			ID:     99,
			FileId: 1002,
			Status: models.StatusFailed,
		}},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		nil,
		&mockListVirtualFileService{},
		nil,
		&mockListCloudTokenService{},
		mountPointService,
		fileTaskLogService,
		nil,
		nil,
		&mockListGroup2FileService{},
		&mockListUserMountPointTokenService{},
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?taskLogStatus=failed&currentPage=2&pageSize=1&path=/media", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if fileTaskLogService.latestCalls != 1 {
		t.Fatalf("expected latest status lookup once, got %d", fileTaskLogService.latestCalls)
	}

	if len(fileTaskLogService.latestStatuses) != 1 || fileTaskLogService.latestStatuses[0] != models.StatusFailed {
		t.Fatalf("unexpected latest statuses: %v", fileTaskLogService.latestStatuses)
	}

	if mountPointService.listCalls != 1 || mountPointService.countCalls != 1 {
		t.Fatalf("expected one list/count call, got list=%d count=%d", mountPointService.listCalls, mountPointService.countCalls)
	}

	if mountPointService.lastListReq.NoPaginate {
		t.Fatal("did not expect task log filtering to use unpaginated mount point list")
	}

	if mountPointService.lastListReq.CurrentPage != 2 || mountPointService.lastListReq.PageSize != 1 {
		t.Fatalf("unexpected pagination: %+v", mountPointService.lastListReq)
	}

	if mountPointService.lastListReq.FullPath != "/media" {
		t.Fatalf("expected full path filter /media, got %q", mountPointService.lastListReq.FullPath)
	}

	requireInt64SliceEqual(t, "mount list file ids", mountPointService.lastListReq.FileIdList, []int64{1002, 1003})
	requireInt64SliceEqual(t, "mount count file ids", mountPointService.lastCountReq.FileIdList, []int64{1002, 1003})

	if fileTaskLogService.listCalls != 1 {
		t.Fatalf("expected current page task log lookup once, got %d", fileTaskLogService.listCalls)
	}

	requireInt64SliceEqual(t, "task log file ids", fileTaskLogService.lastListReq.FileIdList, []int64{1002})
}

func TestListTaskLogStatusReturnsEmptyWhenLatestStatusHasNoFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockListMountPointService{list: []*models.MountPoint{{
		ID:            1,
		FileId:        1001,
		Name:          "movies",
		FullPath:      "/movies",
		TokenId:       0,
		CreatorUserID: 10,
	}}}
	fileTaskLogService := &mockListFileTaskLogService{}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		nil,
		&mockListVirtualFileService{},
		nil,
		&mockListCloudTokenService{},
		mountPointService,
		fileTaskLogService,
		nil,
		nil,
		&mockListGroup2FileService{},
		&mockListUserMountPointTokenService{},
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?taskLogStatus=failed", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.listCalls != 0 || mountPointService.countCalls != 0 {
		t.Fatalf("expected no mount point query when latest status has no files, got list=%d count=%d", mountPointService.listCalls, mountPointService.countCalls)
	}

	if fileTaskLogService.listCalls != 0 {
		t.Fatalf("expected no current page task log lookup for empty list, got %d", fileTaskLogService.listCalls)
	}

	var response struct {
		Code int          `json:"code"`
		Data listResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 0 || len(response.Data.Data) != 0 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
}

func TestListReturnsErrorWhenGroupBindingsFail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
		ctx.Set(consts.CtxKeyUserGroupId, int64(20))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		nil,
		&mockListVirtualFileService{},
		nil,
		&mockListCloudTokenService{},
		&mockListMountPointService{},
		&mockListFileTaskLogService{},
		nil,
		nil,
		&mockListGroup2FileService{err: errors.New("group query failed")},
		&mockListUserMountPointTokenService{},
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected error response, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestListReturnsErrorWhenUserTokenLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(
		nil,
		&mockListVirtualFileService{},
		nil,
		&mockListCloudTokenService{},
		&mockListMountPointService{list: []*models.MountPoint{{
			ID:            1,
			FileId:        1001,
			Name:          "movies",
			FullPath:      "/movies",
			TokenId:       0,
			CreatorUserID: 10,
		}}},
		&mockListFileTaskLogService{},
		nil,
		nil,
		&mockListGroup2FileService{},
		&mockListUserMountPointTokenService{err: errors.New("token query failed")},
	).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected error response, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSelectListReturnsErrorWhenGroupBindingsFail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
		ctx.Set(consts.CtxKeyUserGroupId, int64(20))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/select_list", wrapper.Wrap(NewHandler(
		nil,
		nil,
		nil,
		nil,
		&mockListMountPointService{},
		nil,
		nil,
		nil,
		&mockListGroup2FileService{err: errors.New("group query failed")},
		&mockListUserMountPointTokenService{},
	).SelectList()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/select_list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected error response, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSelectListReturnsErrorWhenUserTokenLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/select_list", wrapper.Wrap(NewHandler(
		nil,
		nil,
		nil,
		nil,
		&mockListMountPointService{list: []*models.MountPoint{{
			ID:            1,
			FileId:        1001,
			Name:          "movies",
			FullPath:      "/movies",
			TokenId:       0,
			CreatorUserID: 10,
		}}},
		nil,
		nil,
		nil,
		&mockListGroup2FileService{},
		&mockListUserMountPointTokenService{err: errors.New("token query failed")},
	).SelectList()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/select_list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected error response, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
