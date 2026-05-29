package mediafile

import (
	stdctx "context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
)

func TestWriteStrmKeepsExistingFileWhenCreateFailsOnPathConflict(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	relPath := "/movies/existing.strm"
	fullPath := filepath.Join(root, relPath)
	originalContent := []byte("http://example.test/original")

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create strm dir: %v", err)
	}

	if err := os.WriteFile(fullPath, originalContent, 0o644); err != nil {
		t.Fatalf("write existing strm: %v", err)
	}

	existing := createMediaFile(t, tDB.db, 3001, relPath)
	car := media.NewWriterCar(root, media.FileConflictPolicyReplace, "http://example.test").NewSubCar("movies/existing.strm")

	id, err := svc.WriteStrm(ctx, car, 3002, "http://example.test/new")
	if err == nil {
		t.Fatalf("expected path conflict error, got nil")
	}

	if id != 0 {
		t.Fatalf("expected zero id, got %d", id)
	}

	content, readErr := os.ReadFile(fullPath)
	if readErr != nil {
		t.Fatalf("expected existing strm file to remain, got %v", readErr)
	}

	if string(content) != string(originalContent) {
		t.Fatalf("expected existing strm content %q, got %q", string(originalContent), string(content))
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Where("id = ?", existing.ID).Count(&count).Error; err != nil {
		t.Fatalf("count existing media file: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected existing media file DB record to remain, got count %d", count)
	}
}
