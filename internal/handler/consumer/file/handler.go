package file

import (
	"path"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"

	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"

	"go.uber.org/zap"
)

type Handler interface {
	ScanFile() taskcontext.HandlerFunc
	ClearFile() taskcontext.HandlerFunc
	HandleBatchDelete() taskcontext.HandlerFunc
	HandleDelete() taskcontext.HandlerFunc
	HandleBatchModifyToken() taskcontext.HandlerFunc
}

type handler struct {
	logger                     *zap.Logger
	virtualFileService         virtualfileSvi.Service
	cloudBridgeService         cloudbridgeSvi.Service
	cloudTokenService          cloudtokenSvi.Service
	mountPointService          mountPointSvi.Service
	fileTaskLogService         filetasklogSvi.Service
	mediaFileService           mediafileSvi.Service
	verifyService              verifySvi.Service
	group2FileService          group2fileSvi.Service
	userMountPointTokenService userMountPointTokenSvi.Service
}

func NewHandler(
	logger *zap.Logger,
	virtualFileService virtualfileSvi.Service,
	cloudBridgeService cloudbridgeSvi.Service,
	cloudTokenService cloudtokenSvi.Service,
	mountPointService mountPointSvi.Service,
	fileTaskLogService filetasklogSvi.Service,
	mediaFileService mediafileSvi.Service,
	verifyService verifySvi.Service,
	group2FileService group2fileSvi.Service,
	userMountPointTokenService userMountPointTokenSvi.Service,
) Handler {
	return &handler{
		logger:                     logger,
		virtualFileService:         virtualFileService,
		cloudBridgeService:         cloudBridgeService,
		cloudTokenService:          cloudTokenService,
		mountPointService:          mountPointService,
		fileTaskLogService:         fileTaskLogService,
		mediaFileService:           mediaFileService,
		verifyService:              verifyService,
		group2FileService:          group2FileService,
		userMountPointTokenService: userMountPointTokenService,
	}
}

type walkFunc func(ctx context.Context, file *models.VirtualFile, childrenFiles []*models.VirtualFile) (nextWalkFiles []*models.VirtualFile, err error)

func (h *handler) walkFile(ctx context.Context, rootId int64, walkFunc walkFunc) (err error) {
	file := new(models.VirtualFile)

	if rootId == 0 {
		file = models.RootFile()
	} else {
		if file, err = h.virtualFileService.Query(ctx, rootId); err != nil {
			return err
		}
	}

	// 计算当前文件的路径
	{
		if prevPath, ok := ctx.GetString(consts.CtxKeyFileFullPath); ok {
			ctx = ctx.WithValue(consts.CtxKeyFileFullPath, path.Join(prevPath, file.Name))
		} else {
			beginPath, err := h.virtualFileService.CalFullPath(ctx, file.ID)
			if err != nil {
				logger.Error("获取文件路径失败", zap.Int64("file_id", file.ID), zap.Error(err))

				return err
			}

			ctx = ctx.WithValue(consts.CtxKeyFileFullPath, beginPath)
		}
	}

	children := make([]*models.VirtualFile, 0)

	// 如果是文件夹类型，递归处理
	if file.IsDir {
		if children, err = h.virtualFileService.List(ctx, &virtualfileSvi.ListRequest{
			ParentId: &file.ID,
		}); err != nil {
			return err
		}
	}
	time.Sleep(5 * time.Millisecond)
	ctx.Debug(
		"开始处理文件",
		zap.Int64("file_id", file.ID),
		zap.String("file_name", file.Name),
	)

	if nextFiles, walkErr := walkFunc(ctx, file, children); walkErr != nil {
		return walkErr
	} else if len(nextFiles) > 0 {
		// 获取线程数配置
		threadCount := shared.SettingAddition.TaskThreadCount
		if threadCount <= 0 {
			threadCount = 1
		}

		// 如果只有一个线程或者文件数量很少，使用串行处理
		if threadCount == 1 || len(nextFiles) <= 1 {
			for _, nextFile := range nextFiles {
				time.Sleep(2 * time.Millisecond)
				if err = h.walkFile(ctx, nextFile.ID, walkFunc); err != nil {
					return err
				}
			}
		} else {
			// 使用多线程并发处理
			var wg sync.WaitGroup

			errorChan := make(chan error, len(nextFiles))
			semaphore := make(chan struct{}, threadCount)

			for _, nextFile := range nextFiles {
				wg.Add(1)

				go func(file *models.VirtualFile) {
					defer wg.Done()

					// 获取信号量
					semaphore <- struct{}{}
					defer func() {
						<-semaphore
						time.Sleep(10 * time.Millisecond)
					}()

					if walkErr := h.walkFile(ctx, file.ID, walkFunc); walkErr != nil {
						errorChan <- walkErr
					}
				}(nextFile)
			}

			// 等待所有goroutine完成
			wg.Wait()
			close(errorChan)

			// 检查是否有错误
			for err = range errorChan {
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
