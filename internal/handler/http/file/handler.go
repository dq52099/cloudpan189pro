package file

import (
	"net/http"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"

	cloudBridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

type Handler interface {
	Search() httpcontext.HandlerFunc
	CreateDownloadURL() httpcontext.HandlerFunc
	Download() httpcontext.HandlerFunc
	Open() httpcontext.HandlerFunc
	BatchDelete() httpcontext.HandlerFunc // [新增]
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeFileStartCode)

var (
	busCodeList                         = bi.Next("查询文件列表失败")
	busCodeCount                        = bi.Next("查询文件数量失败")
	busCodeCalFullPath                  = bi.Next("计算文件完整路径失败")
	busCodeFileQueryError               = bi.Next("查询文件失败")
	busCodeFileVerifyError              = bi.Next("鉴权失败")
	busCodeFileIsDirNotSupport          = bi.Next("目录文件不支持操作")
	busCodeTokenNotBind                 = bi.Next("文件令牌未绑定")
	busCodeTokenQueryError              = bi.Next("查询文件令牌失败")
	busCodeGetDownloadLinkError         = bi.Next("获取下载链接失败")
	busCodeMissFamilyId                 = bi.Next("缺少family_id参数")
	busCodeMissShareId                  = bi.Next("缺少share_id参数")
	busCodeUnsupportedOsType            = bi.Next("不支持的文件类型")
	busCodeCreateLocalProxyRequestError = bi.Next("创建本地代理请求失败")
	busCodeCreateMultiStreamProxyError  = bi.Next("创建多线程请求失败")
	busCodeFileSignError                = bi.Next("文件签名失败")
	busCodeFilePathSplitError           = bi.Next("路径切割失败")
	busCodeFileInvalidPath              = bi.Next("路径不合法，需要 / 开头的路径")
	busCodeFileNotFound                 = bi.Next("文件不存在").WithHTTPCode(http.StatusNotFound)
	busCodeQueryTopIdError              = bi.Next("查询文件顶级id失败")
	busCodeBatchDeleteError             = bi.Next("发送批量删除任务失败")
)

type handler struct {
	virtualFileService         virtualfileSvi.Service
	verifyService              verifySvi.Service
	cloudTokenService          cloudTokenSvi.Service
	cloudBridgeService         cloudBridgeSvi.Service
	mountPointService          mountPointSvi.Service
	group2FileService          group2fileSvi.Service
	userMountPointTokenService userMountPointTokenSvi.Service
	taskEngine                 taskengine.TaskEngine
}

func NewHandler(
	virtualFileService virtualfileSvi.Service,
	verifyService verifySvi.Service,
	cloudTokenService cloudTokenSvi.Service,
	cloudBridgeService cloudBridgeSvi.Service,
	mountPointService mountPointSvi.Service,
	group2FileService group2fileSvi.Service,
	userMountPointTokenService userMountPointTokenSvi.Service,
	taskEngine taskengine.TaskEngine,
) Handler {
	return &handler{
		virtualFileService:         virtualFileService,
		verifyService:              verifyService,
		cloudTokenService:          cloudTokenService,
		cloudBridgeService:         cloudBridgeService,
		mountPointService:          mountPointService,
		group2FileService:          group2FileService,
		userMountPointTokenService: userMountPointTokenService,
		taskEngine:                 taskEngine,
	}
}
