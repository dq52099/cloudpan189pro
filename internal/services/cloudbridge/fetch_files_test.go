package cloudbridge

import (
	stdContext "context"
	"os"
	"strconv"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

// tip 工作目录设置为项目根目录

var (
	accessToken     = os.Getenv(consts.EnvKeyTestAccessToken)
	accessExpire, _ = strconv.Atoi(os.Getenv(consts.EnvKeyTestAccessExpire))

	personFileId       = os.Getenv(consts.EnvKeyTestPersonFileId)
	personFileCount, _ = strconv.Atoi(os.Getenv(consts.EnvKeyTestPersonFileCount))

	familyFamilyId     = os.Getenv(consts.EnvKeyTestFamilyFamilyId)
	familyFileId       = os.Getenv(consts.EnvKeyTestFamilyFileId)
	familyFileCount, _ = strconv.Atoi(os.Getenv(consts.EnvKeyTestFamilyFileCount))

	// Subscribe User 相关测试数据
	subUserId               = os.Getenv(consts.EnvKeyTestSubUserId)
	subUserResourceCount, _ = strconv.Atoi(os.Getenv(consts.EnvKeyTestSubUserResourceCount))

	// Subscribe Share 相关测试数据
	subShareId, _            = strconv.ParseInt(os.Getenv(consts.EnvKeyTestSubShareId), 10, 64)
	subShareFileId           = os.Getenv(consts.EnvKeyTestSubShareFileId)
	subShareIsFolder, _      = strconv.ParseBool(os.Getenv(consts.EnvKeyTestSubShareIsFolder))
	subShareResourceCount, _ = strconv.Atoi(os.Getenv(consts.EnvKeyTestSubShareResourceCount))

	// Share 相关测试数据
	shareId, _            = strconv.ParseInt(os.Getenv(consts.EnvKeyTestShareId), 10, 64)
	shareFileId           = os.Getenv(consts.EnvKeyTestShareFileId)
	shareMode, _          = strconv.Atoi(os.Getenv(consts.EnvKeyTestShareMode))
	shareIsFolder, _      = strconv.ParseBool(os.Getenv(consts.EnvKeyTestShareIsFolder))
	shareResourceCount, _ = strconv.Atoi(os.Getenv(consts.EnvKeyTestShareResourceCount))

	// Share With Code 相关测试数据
	shareIdWithCode, _            = strconv.ParseInt(os.Getenv(consts.EnvKeyTestShareIdWithCode), 10, 64)
	shareFileIdWithCode           = os.Getenv(consts.EnvKeyTestShareFileIdWithCode)
	shareModeWithCode, _          = strconv.Atoi(os.Getenv(consts.EnvKeyTestShareModeWithCode))
	shareIsFolderWithCode, _      = strconv.ParseBool(os.Getenv(consts.EnvKeyTestShareIsFolderWithCode))
	shareWithCodeResourceCount, _ = strconv.Atoi(os.Getenv(consts.EnvKeyTestShareWithCodeResourceCount))
	shareCode                     = os.Getenv(consts.EnvKeyTestShareCode)
)

func requireTestEnv(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if os.Getenv(key) == "" {
			t.Skipf("skipping cloudbridge integration test; missing %s", key)
		}
	}
}

func TestGetSubscribeUserFiles(t *testing.T) {
	requireTestEnv(t,
		consts.EnvKeyTestSubUserId,
		consts.EnvKeyTestSubUserResourceCount,
	)

	mockSvc := bootstrap.NewMockServiceContext()

	ctx := context.NewContext(stdContext.Background())

	list, err := NewService(mockSvc).GetSubscribeUserFiles(ctx, subUserId)
	if err != nil {
		t.Error(err)

		return
	}

	if len(list) != subUserResourceCount {
		t.Errorf("期望获取 %d 个文件，实际获取 %d 个文件", subUserResourceCount, len(list))

		return
	}

	t.Logf("成功获取到 %d 个文件", len(list))
}

func TestGetSubscribeShareFiles(t *testing.T) {
	requireTestEnv(t,
		consts.EnvKeyTestSubUserId,
		consts.EnvKeyTestSubShareId,
		consts.EnvKeyTestSubShareFileId,
		consts.EnvKeyTestSubShareIsFolder,
		consts.EnvKeyTestSubShareResourceCount,
	)

	mockSvc := bootstrap.NewMockServiceContext()

	ctx := context.NewContext(stdContext.Background())

	list, err := NewService(mockSvc).GetSubscribeShareFiles(ctx, subUserId, subShareId, subShareFileId, subShareIsFolder)
	if err != nil {
		t.Error(err)

		return
	}

	if len(list) != subShareResourceCount {
		t.Errorf("期望获取 %d 个文件，实际获取 %d 个文件", subShareResourceCount, len(list))

		return
	}

	t.Logf("成功获取到 %d 个文件", len(list))
}

func TestGetShareFiles(t *testing.T) {
	requireTestEnv(t,
		consts.EnvKeyTestShareId,
		consts.EnvKeyTestShareFileId,
		consts.EnvKeyTestShareMode,
		consts.EnvKeyTestShareIsFolder,
		consts.EnvKeyTestShareResourceCount,
	)

	mockSvc := bootstrap.NewMockServiceContext()

	ctx := context.NewContext(stdContext.Background())

	list, err := NewService(mockSvc).GetShareFiles(ctx, shareId, shareFileId, shareMode, "", shareIsFolder)
	if err != nil {
		t.Error(err)

		return
	}

	if len(list) != shareResourceCount {
		t.Errorf("期望获取 %d 个文件，实际获取 %d 个文件", shareResourceCount, len(list))

		return
	}

	t.Logf("成功获取到 %d 个文件", len(list))
}

func TestGetShareFilesWithCode(t *testing.T) {
	requireTestEnv(t,
		consts.EnvKeyTestShareIdWithCode,
		consts.EnvKeyTestShareFileIdWithCode,
		consts.EnvKeyTestShareModeWithCode,
		consts.EnvKeyTestShareIsFolderWithCode,
		consts.EnvKeyTestShareWithCodeResourceCount,
		consts.EnvKeyTestShareCode,
	)

	mockSvc := bootstrap.NewMockServiceContext()

	ctx := context.NewContext(stdContext.Background())

	list, err := NewService(mockSvc).GetShareFiles(ctx, shareIdWithCode, shareFileIdWithCode, shareModeWithCode, shareCode, shareIsFolderWithCode)
	if err != nil {
		t.Error(err)

		return
	}

	if len(list) != shareWithCodeResourceCount {
		t.Errorf("期望获取 %d 个文件，实际获取 %d 个文件", shareWithCodeResourceCount, len(list))

		return
	}

	t.Logf("成功获取到 %d 个文件", len(list))
}

func TestGetCloudFiles(t *testing.T) {
	requireTestEnv(t,
		consts.EnvKeyTestAccessToken,
		consts.EnvKeyTestAccessExpire,
		consts.EnvKeyTestPersonFileId,
		consts.EnvKeyTestPersonFileCount,
	)

	mockSvc := bootstrap.NewMockServiceContext()

	ctx := context.NewContext(stdContext.Background())

	list, err := NewService(mockSvc).GetCloudFiles(ctx, NewAuthToken(accessToken, int64(accessExpire)), personFileId)
	if err != nil {
		t.Error(err)

		return
	}

	if len(list) != personFileCount {
		t.Errorf("期望获取 %d 个文件，实际获取 %d 个文件", personFileCount, len(list))

		return
	}

	t.Logf("成功获取到 %d 个文件", len(list))
}

func TestGetCloudFamilyFiles(t *testing.T) {
	requireTestEnv(t,
		consts.EnvKeyTestAccessToken,
		consts.EnvKeyTestAccessExpire,
		consts.EnvKeyTestFamilyFamilyId,
		consts.EnvKeyTestFamilyFileId,
		consts.EnvKeyTestFamilyFileCount,
	)

	mockSvc := bootstrap.NewMockServiceContext()

	ctx := context.NewContext(stdContext.Background())

	list, err := NewService(mockSvc).GetCloudFamilyFiles(ctx, NewAuthToken(accessToken, int64(accessExpire)), familyFamilyId, familyFileId)
	if err != nil {
		t.Error(err)

		return
	}

	if len(list) != familyFileCount {
		t.Errorf("期望获取 %d 个文件，实际获取 %d 个文件", familyFileCount, len(list))

		return
	}

	t.Logf("成功获取到 %d 个文件", len(list))
}
