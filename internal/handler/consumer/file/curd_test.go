package file

import (
	stdctx "context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	mediaType "github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockBatchDeleteVirtualFileService struct {
	virtualfileSvi.Service
	listErr          error
	childrenByParent map[int64][]*models.VirtualFile
	deleteErrByID    map[int64]error
	filesByID        map[int64]*models.VirtualFile
	deletedIDs       []int64
	batchDeletedIDs  []int64
	callLog          *[]string
}

func appendTestCall(callLog *[]string, name string) {
	if callLog != nil {
		*callLog = append(*callLog, name)
	}
}

type mockStrmWrite struct {
	fid     int64
	baseURL string
	fileURL string
}

type mockStrmMediaFileService struct {
	mediafileSvi.Service

	writes     []mockStrmWrite
	deletedIDs []int64
}

func (m *mockStrmMediaFileService) WriteStrm(ctx appContext.Context, car mediaType.WriterCar, fid int64, fileURL string) (int64, error) {
	m.writes = append(m.writes, mockStrmWrite{
		fid:     fid,
		baseURL: car.GetBaseURL(),
		fileURL: fileURL,
	})

	return 1, nil
}

func (m *mockStrmMediaFileService) DeleteStrm(ctx appContext.Context, fid int64, rootPath string) error {
	m.deletedIDs = append(m.deletedIDs, fid)

	return nil
}

func (m *mockStrmMediaFileService) DeleteStrmByFullPath(ctx appContext.Context, fullPath string) error {
	return nil
}

type mockStrmVerifyService struct {
	verifySvi.Service
}

func (m *mockStrmVerifyService) SignV1(ctx appContext.Context, fileID int64, opts ...verifySvi.SignV1OptionFunc) (url.Values, error) {
	return url.Values{"sign": []string{"ok"}}, nil
}

func (m *mockBatchDeleteVirtualFileService) List(ctx appContext.Context, req *virtualfileSvi.ListRequest) ([]*models.VirtualFile, error) {
	appendTestCall(m.callLog, "virtual-list")

	if m.listErr != nil {
		return nil, m.listErr
	}

	if req == nil || req.ParentId == nil {
		return []*models.VirtualFile{}, nil
	}

	return m.childrenByParent[*req.ParentId], nil
}

func (m *mockBatchDeleteVirtualFileService) Query(ctx appContext.Context, fid int64) (*models.VirtualFile, error) {
	return m.filesByID[fid], nil
}

func (m *mockBatchDeleteVirtualFileService) CalFullPath(ctx appContext.Context, id int64) (string, error) {
	return "/", nil
}

func (m *mockBatchDeleteVirtualFileService) Delete(
	ctx appContext.Context,
	id int64,
	hooks ...virtualfileSvi.DeleteHook,
) error {
	appendTestCall(m.callLog, "virtual-delete")

	if err := m.deleteErrByID[id]; err != nil {
		return err
	}

	m.deletedIDs = append(m.deletedIDs, id)

	return nil
}

func (m *mockBatchDeleteVirtualFileService) BatchDelete(
	ctx appContext.Context,
	ids []int64,
	hooks ...virtualfileSvi.BatchDeleteHook,
) ([]int64, error) {
	appendTestCall(m.callLog, "virtual-batch-delete")

	for _, id := range ids {
		if err := m.deleteErrByID[id]; err != nil {
			return nil, err
		}
	}

	m.batchDeletedIDs = append(m.batchDeletedIDs, ids...)

	return append([]int64(nil), ids...), nil
}

func TestWalkFileReturnsNotFoundWhenRootQueryReturnsNil(t *testing.T) {
	h := &handler{
		logger: zap.NewNop(),
		virtualFileService: &mockBatchDeleteVirtualFileService{
			filesByID: map[int64]*models.VirtualFile{
				10: nil,
			},
		},
	}
	called := false

	err := h.walkFile(appContext.NewContext(stdctx.Background()), 10, func(ctx appContext.Context, file *models.VirtualFile, childrenFiles []*models.VirtualFile) ([]*models.VirtualFile, error) {
		called = true

		return nil, nil
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if called {
		t.Fatal("expected walk function not to be called for nil root file")
	}
}

func TestBatchDeleteFilesReturnsChildListError(t *testing.T) {
	wantErr := errors.New("list children failed")
	h := &handler{
		logger: zap.NewNop(),
		virtualFileService: &mockBatchDeleteVirtualFileService{
			listErr:          wantErr,
			childrenByParent: map[int64][]*models.VirtualFile{},
			deleteErrByID:    map[int64]error{},
		},
	}

	err := h.batchDeleteFiles(appContext.NewContext(stdctx.Background()), []*models.VirtualFile{{
		ID:    10,
		IsDir: true,
		Name:  "dir",
	}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected child list error, got %v", err)
	}
}

func TestBatchDeleteFilesReturnsRecursiveDeleteError(t *testing.T) {
	wantErr := errors.New("delete child failed")
	h := &handler{
		logger: zap.NewNop(),
		virtualFileService: &mockBatchDeleteVirtualFileService{
			childrenByParent: map[int64][]*models.VirtualFile{
				10: {{
					ID:   11,
					Name: "child",
				}},
			},
			deleteErrByID: map[int64]error{11: wantErr},
		},
	}

	err := h.batchDeleteFiles(appContext.NewContext(stdctx.Background()), []*models.VirtualFile{{
		ID:    10,
		IsDir: true,
		Name:  "dir",
	}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected recursive delete error, got %v", err)
	}
}

func TestBatchDeleteFilesDeletesChildrenBeforeParent(t *testing.T) {
	mockService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{
			10: {{
				ID:    11,
				IsDir: true,
				Name:  "child-dir",
			}},
			11: {{
				ID:   12,
				Name: "grandchild.mkv",
			}},
		},
		deleteErrByID: map[int64]error{},
	}
	h := &handler{
		logger:             zap.NewNop(),
		virtualFileService: mockService,
	}

	err := h.batchDeleteFiles(appContext.NewContext(stdctx.Background()), []*models.VirtualFile{{
		ID:    10,
		IsDir: true,
		Name:  "parent-dir",
	}})
	if err != nil {
		t.Fatalf("batch delete files: %v", err)
	}

	wantDeletedIDs := []int64{12, 11, 10}
	if !slices.Equal(mockService.batchDeletedIDs, wantDeletedIDs) {
		t.Fatalf("expected batch deleted ids %v, got %v", wantDeletedIDs, mockService.batchDeletedIDs)
	}
}

func TestCleanupEmptyAncestorFoldersStopsWhenChildListFails(t *testing.T) {
	wantErr := errors.New("list children failed")
	mockService := &mockBatchDeleteVirtualFileService{
		listErr: wantErr,
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, ParentId: 9, Name: "parent"},
		},
		deleteErrByID: map[int64]error{},
	}
	h := &handler{
		logger:             zap.NewNop(),
		virtualFileService: mockService,
	}

	h.cleanupEmptyAncestorFolders(appContext.NewContext(stdctx.Background()), 10)

	if len(mockService.deletedIDs) != 0 {
		t.Fatalf("expected no ancestor delete when child list fails, got %v", mockService.deletedIDs)
	}
}

func TestCleanupEmptyAncestorFoldersDeletesEmptyAncestors(t *testing.T) {
	mockService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		filesByID: map[int64]*models.VirtualFile{
			9:  {ID: 9, ParentId: 0, Name: "grandparent"},
			10: {ID: 10, ParentId: 9, Name: "parent"},
		},
		deleteErrByID: map[int64]error{},
	}
	h := &handler{
		logger:             zap.NewNop(),
		virtualFileService: mockService,
	}

	h.cleanupEmptyAncestorFolders(appContext.NewContext(stdctx.Background()), 10)

	wantDeletedIDs := []int64{10, 9}
	if !slices.Equal(mockService.deletedIDs, wantDeletedIDs) {
		t.Fatalf("expected deleted ids %v, got %v", wantDeletedIDs, mockService.deletedIDs)
	}
}

func TestCleanupEmptyAncestorFoldersKeepsNonEmptyParent(t *testing.T) {
	mockService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{
			10: {{ID: 11, ParentId: 10, Name: "child"}},
		},
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, ParentId: 9, Name: "parent"},
		},
		deleteErrByID: map[int64]error{},
	}
	h := &handler{
		logger:             zap.NewNop(),
		virtualFileService: mockService,
	}

	h.cleanupEmptyAncestorFolders(appContext.NewContext(stdctx.Background()), 10)

	if len(mockService.deletedIDs) != 0 {
		t.Fatalf("expected non-empty ancestor to be kept, got deletes %v", mockService.deletedIDs)
	}
}

func setupConsumerFileStrmMediaConfig(t *testing.T) {
	t.Helper()

	oldMediaConfig := shared.MediaConfig
	oldBaseURL := shared.BaseURL

	t.Cleanup(func() {
		shared.MediaConfig = oldMediaConfig
		shared.BaseURL = oldBaseURL
	})

	shared.MediaConfig = &models.MediaConfig{
		Enable:         true,
		StoragePath:    t.TempDir(),
		ConflictPolicy: mediaType.FileConflictPolicySkip,
		BaseURL:        "https://media.example.test",
	}
	shared.BaseURL = "https://system.example.test"
}

func TestCreateStrmIteratorUsesMediaConfigBaseURL(t *testing.T) {
	setupConsumerFileStrmMediaConfig(t)

	mediaFileService := &mockStrmMediaFileService{}
	h := &handler{
		logger:           zap.NewNop(),
		mediaFileService: mediaFileService,
		verifyService:    &mockStrmVerifyService{},
	}
	ctx := appContext.NewContext(stdctx.Background()).WithValue(consts.CtxKeyFileFullPath, "/movies")

	h.createStrmIteratorfunc(ctx, &gorm.DB{}, []*models.VirtualFile{
		{ID: 200, Name: "movie.mp4", IsDir: false},
	})

	if len(mediaFileService.writes) != 1 {
		t.Fatalf("expected one STRM write, got %#v", mediaFileService.writes)
	}

	got := mediaFileService.writes[0]
	if got.baseURL != shared.MediaConfig.BaseURL {
		t.Fatalf("expected car base URL %q, got %q", shared.MediaConfig.BaseURL, got.baseURL)
	}

	if !strings.HasPrefix(got.fileURL, shared.MediaConfig.BaseURL+"/api/file/download/200?") {
		t.Fatalf("expected STRM download URL to use media base URL, got %q", got.fileURL)
	}

	if strings.HasPrefix(got.fileURL, shared.BaseURL) {
		t.Fatalf("expected STRM download URL not to use system base URL %q", shared.BaseURL)
	}
}

func TestSyncStrmAfterUpdateUsesMediaConfigBaseURL(t *testing.T) {
	setupConsumerFileStrmMediaConfig(t)

	mediaFileService := &mockStrmMediaFileService{}
	h := &handler{
		logger:           zap.NewNop(),
		mediaFileService: mediaFileService,
		verifyService:    &mockStrmVerifyService{},
	}
	ctx := appContext.NewContext(stdctx.Background()).WithValue(consts.CtxKeyFileFullPath, "/movies")

	h.syncStrmAfterUpdate(ctx, []*updateStrmContext{{
		oldFile: &models.VirtualFile{ID: 300, Name: "movie.mp4", IsDir: false},
		newFile: &models.VirtualFile{ID: 300, Name: "movie.mp4", IsDir: false},
	}})

	if len(mediaFileService.writes) != 1 {
		t.Fatalf("expected one STRM write, got %#v", mediaFileService.writes)
	}

	got := mediaFileService.writes[0]
	if got.baseURL != shared.MediaConfig.BaseURL {
		t.Fatalf("expected car base URL %q, got %q", shared.MediaConfig.BaseURL, got.baseURL)
	}

	if !strings.HasPrefix(got.fileURL, shared.MediaConfig.BaseURL+"/api/file/download/300?") {
		t.Fatalf("expected STRM download URL to use media base URL, got %q", got.fileURL)
	}

	if strings.HasPrefix(got.fileURL, shared.BaseURL) {
		t.Fatalf("expected STRM download URL not to use system base URL %q", shared.BaseURL)
	}
}
