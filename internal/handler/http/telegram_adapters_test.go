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
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/services/telegram"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

type mockTelegramCloudBridgeService struct {
	cloudbridge.Service
	info *cloudbridge.ShareInfo
	err  error
}

func (m *mockTelegramCloudBridgeService) GetShareInfo(
	ctx appContext.Context,
	shareCode string,
	accessCode string,
) (*cloudbridge.ShareInfo, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.info, nil
}

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

func TestTelegramShareAdapterPreservesShareMetadata(t *testing.T) {
	adapter := &telegramShareAdapter{
		inner: &mockTelegramCloudBridgeService{
			info: &cloudbridge.ShareInfo{
				Name:       "分享目录",
				ShareId:    12345,
				ShareMode:  2,
				ID:         "cloud-file-id",
				IsFolder:   true,
				AccessCode: "abcd",
			},
		},
	}

	info, err := adapter.GetShareInfo(stdctx.Background(), "share-code", "abcd")
	if err != nil {
		t.Fatalf("get share info: %v", err)
	}

	if info.Name != "分享目录" ||
		info.ShareId != 12345 ||
		info.ShareMode != 2 ||
		info.FileId != "cloud-file-id" ||
		!info.IsFolder ||
		info.AccessCode != "abcd" {
		t.Fatalf("expected full share metadata preserved, got %+v", info)
	}
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
		ShareID:           12345,
		ShareMode:         2,
		AccessCode:        "abcd",
		FileID:            "cloud-file-id",
		IsFolder:          true,
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

	if storageFacade.req.CreatorUserID != 1 || !storageFacade.req.IsAdmin {
		t.Fatalf("expected system admin storage request, got %#v", storageFacade.req)
	}

	assertTelegramAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyShareId, int64(12345))
	assertTelegramAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyIsFolder, true)
	assertTelegramAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyShareMode, 2)
	assertTelegramAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyAccessCode, "abcd")

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

func TestTelegramMountAdapterRejectsMissingShareID(t *testing.T) {
	storageFacade := &mockTelegramStorageFacade{createID: 11}
	adapter := &telegramMountAdapter{
		inner:             storageFacade,
		taskEngine:        &mockTelegramTaskEngine{},
		mountPointService: &mockTelegramMountPointService{},
	}

	id, err := adapter.CreateMountPoint(stdctx.Background(), &telegram.MountRequest{
		LocalPath: "/Telegram/share",
		OsType:    models.OsTypeSubscribeShareFolder,
		ShareCode: "share-code",
		FileID:    "cloud-file-id",
	})
	if id != 0 {
		t.Fatalf("expected no mount id when share id is missing, got %d", id)
	}

	if err == nil || !strings.Contains(err.Error(), "分享ID缺失") {
		t.Fatalf("expected missing share id error, got %v", err)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage not to be called when share id is missing, got %#v", storageFacade.req)
	}
}

func TestTelegramMountAdapterRejectsTypedNilScanDependencies(t *testing.T) {
	var (
		typedNilMountPointService *mockTelegramMountPointService
		typedNilTaskEngine        *mockTelegramTaskEngine
	)

	tests := []struct {
		name              string
		mountPointService mountpoint.Service
		taskEngine        taskengine.TaskEngine
	}{
		{
			name:              "typed nil mount point service",
			mountPointService: typedNilMountPointService,
			taskEngine:        &mockTelegramTaskEngine{},
		},
		{
			name: "typed nil task engine",
			mountPointService: &mockTelegramMountPointService{
				mountPoint: &models.MountPoint{
					FileId:   11,
					FullPath: "/Telegram/share",
				},
			},
			taskEngine: typedNilTaskEngine,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageFacade := &mockTelegramStorageFacade{createID: 11}
			adapter := &telegramMountAdapter{
				inner:             storageFacade,
				taskEngine:        tt.taskEngine,
				mountPointService: tt.mountPointService,
			}

			id, err := adapter.CreateMountPoint(stdctx.Background(), &telegram.MountRequest{
				LocalPath: "/Telegram/share",
				OsType:    models.OsTypeSubscribeShareFolder,
				ShareID:   12345,
				FileID:    "cloud-file-id",
				IsFolder:  true,
			})
			if id != 11 {
				t.Fatalf("expected created mount id to be returned with error, got %d", id)
			}

			if err == nil || !strings.Contains(err.Error(), "telegram 挂载扫描依赖未初始化") {
				t.Fatalf("expected dependency error, got %v", err)
			}

			if storageFacade.req == nil {
				t.Fatal("expected storage creation before scan dependency validation")
			}

			if mountPointService, ok := tt.mountPointService.(*mockTelegramMountPointService); ok &&
				mountPointService != nil &&
				mountPointService.queryFile != 0 {
				t.Fatalf("expected missing scan dependency to stop before querying mount point, got %d", mountPointService.queryFile)
			}

			if taskEngine, ok := tt.taskEngine.(*mockTelegramTaskEngine); ok &&
				taskEngine != nil &&
				len(taskEngine.payloads) != 0 {
				t.Fatalf("expected missing scan dependency to stop before pushing scan task, got %d payloads", len(taskEngine.payloads))
			}
		})
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
		ShareID:   12345,
		FileID:    "cloud-file-id",
		IsFolder:  true,
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

func assertTelegramAdditionValue(t *testing.T, addition map[string]interface{}, key string, want interface{}) {
	t.Helper()

	got, ok := addition[key]
	if !ok {
		t.Fatalf("expected addition %q to exist, got %#v", key, addition)
	}

	if got != want {
		t.Fatalf("expected addition %q = %#v (%T), got %#v (%T)", key, want, want, got, got)
	}
}
