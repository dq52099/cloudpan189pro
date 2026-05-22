package file

import (
	stdctx "context"
	"errors"
	"slices"
	"testing"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"go.uber.org/zap"
)

type mockBatchDeleteVirtualFileService struct {
	virtualfileSvi.Service
	listErr          error
	childrenByParent map[int64][]*models.VirtualFile
	deleteErrByID    map[int64]error
	filesByID        map[int64]*models.VirtualFile
	deletedIDs       []int64
	batchDeletedIDs  []int64
}

func (m *mockBatchDeleteVirtualFileService) List(ctx appContext.Context, req *virtualfileSvi.ListRequest) ([]*models.VirtualFile, error) {
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
	for _, id := range ids {
		if err := m.deleteErrByID[id]; err != nil {
			return nil, err
		}
	}

	m.batchDeletedIDs = append(m.batchDeletedIDs, ids...)

	return append([]int64(nil), ids...), nil
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
