package http

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/services/telegram"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

type mockTelegramStorageFacade struct {
	storagefacade.Service
	createID int64
	req      *storagefacade.CreateStorageRequest
}

func (m *mockTelegramStorageFacade) CreateStorage(
	ctx appContext.Context,
	req *storagefacade.CreateStorageRequest,
) (int64, error) {
	m.req = req

	return m.createID, nil
}

type mockTelegramMountPointService struct {
	mountpoint.Service
	mountPoint *models.MountPoint
	queryFile  int64
	err        error
}

func (m *mockTelegramMountPointService) Query(
	ctx appContext.Context,
	fileID int64,
) (*models.MountPoint, error) {
	m.queryFile = fileID
	if m.err != nil {
		return nil, m.err
	}

	return m.mountPoint, nil
}

type mockTelegramTaskEngine struct {
	taskengine.TaskEngine
	err      error
	payloads [][]byte
	paths    []any
	topics   []taskengine.Topic
}

func (m *mockTelegramTaskEngine) PushMessage(
	ctx stdctx.Context,
	taskTopic taskengine.Topic,
	payload []byte,
) error {
	if m.err != nil {
		return m.err
	}

	m.payloads = append(m.payloads, payload)
	m.paths = append(m.paths, ctx.Value(consts.CtxKeyFullPath))
	m.topics = append(m.topics, taskTopic)

	return nil
}

func TestTelegramMountAdapterQueuesScanTask(t *testing.T) {
	storageFacade := &mockTelegramStorageFacade{createID: 11}
	mountPointService := &mockTelegramMountPointService{
		mountPoint: &models.MountPoint{
			FileId:   11,
			FullPath: "/Telegram/share",
		},
	}
	taskEngine := &mockTelegramTaskEngine{}
	adapter := &telegramMountAdapter{
		inner:             storageFacade,
		taskEngine:        taskEngine,
		mountPointService: mountPointService,
	}

	id, err := adapter.CreateMountPoint(stdctx.Background(), &telegram.MountRequest{
		LocalPath:         "/Telegram/share",
		OsType:            models.OsTypeSubscribeShareFolder,
		ShareCode:         "share-code",
		FileID:            "cloud-file-id",
		EnableDeepRefresh: true,
	})
	if err != nil {
		t.Fatalf("expected mount and scan dispatch success, got %v", err)
	}

	if id != 11 {
		t.Fatalf("expected mount id 11, got %d", id)
	}

	if storageFacade.req == nil || !storageFacade.req.AllowExisting {
		t.Fatalf("expected allow existing storage request, got %#v", storageFacade.req)
	}

	if mountPointService.queryFile != 11 {
		t.Fatalf("expected query by created file id 11, got %d", mountPointService.queryFile)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one scan task payload, got %d", len(taskEngine.payloads))
	}

	if len(taskEngine.topics) != 1 || taskEngine.topics[0] != taskengine.Topic(topic.KeyFileScanFile) {
		t.Fatalf("expected file scan topic, got %#v", taskEngine.topics)
	}

	if len(taskEngine.paths) != 1 || taskEngine.paths[0] != "/Telegram/share" {
		t.Fatalf("expected full path context, got %#v", taskEngine.paths)
	}

	var taskReq topic.FileScanFileRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatalf("unmarshal scan payload: %v", err)
	}

	if taskReq.FileId != 11 || !taskReq.Deep {
		t.Fatalf("expected deep scan for file 11, got %+v", taskReq)
	}
}

func TestTelegramMountAdapterReturnsScanDispatchError(t *testing.T) {
	adapter := &telegramMountAdapter{
		inner: &mockTelegramStorageFacade{createID: 11},
		taskEngine: &mockTelegramTaskEngine{
			err: errors.New("queue unavailable"),
		},
		mountPointService: &mockTelegramMountPointService{
			mountPoint: &models.MountPoint{
				FileId:   11,
				FullPath: "/Telegram/share",
			},
		},
	}

	id, err := adapter.CreateMountPoint(stdctx.Background(), &telegram.MountRequest{
		LocalPath: "/Telegram/share",
		OsType:    models.OsTypeSubscribeShareFolder,
		FileID:    "cloud-file-id",
	})
	if id != 11 {
		t.Fatalf("expected created mount id to be returned with error, got %d", id)
	}

	if err == nil {
		t.Fatal("expected scan dispatch error")
	}

	if !strings.Contains(err.Error(), "下发 Telegram 挂载扫描任务失败") ||
		!strings.Contains(err.Error(), "queue unavailable") {
		t.Fatalf("expected wrapped queue error, got %v", err)
	}
}
