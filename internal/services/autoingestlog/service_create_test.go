package autoingestlog

import (
	stdctx "context"
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
)

func TestCreateSanitizesContentBeforePersisting(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	content := "新增入库失败: GET https://proxy-user:proxy-pass@cloud.example.test/file?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"

	id, err := svc.Create(ctx, 10, autoingest.LogLevelError, content)
	if err != nil {
		t.Fatalf("create auto ingest log: %v", err)
	}

	var got models.AutoIngestLog
	if err := tDB.db.First(&got, id).Error; err != nil {
		t.Fatalf("query auto ingest log: %v", err)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret"} {
		if strings.Contains(got.Content, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got.Content)
		}
	}

	if !strings.Contains(got.Content, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", got.Content)
	}
}
