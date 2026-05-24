package cloudtoken

import (
	"net/http"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
)

type Handler interface {
	InitQrcode() httpcontext.HandlerFunc
	CheckQrcode() httpcontext.HandlerFunc
	ModifyName() httpcontext.HandlerFunc
	Delete() httpcontext.HandlerFunc
	List() httpcontext.HandlerFunc
	UsernameLogin() httpcontext.HandlerFunc
	Query() httpcontext.HandlerFunc
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeCloudTokenStartCode)

var (
	codeInitQrcodeFailed    = bi.Next("初始化二维码失败")
	codeCheckQrcodeFailed   = bi.Next("检查二维码失败")
	codeModifyNameFailed    = bi.Next("修改名称失败")
	codeDeleteFailed        = bi.Next("删除云盘令牌失败")
	codeListFailed          = bi.Next("获取云盘令牌列表失败")
	codeUsernameLoginFailed = bi.Next("用户名登录失败")
	codeQueryFailed         = bi.Next("查询云盘令牌失败")
	codeMountPointUsed      = bi.Next("令牌正在被挂载点使用，请先解绑")
	codeMissUsername        = bi.Next("缺少用户名")
	codeMissPassword        = bi.Next("缺少密码")
	codeNotMatchLoginType   = bi.Next("不匹配的登录类型")
	codeTokenNotFound       = bi.Next("云盘令牌不存在").WithHTTPCode(http.StatusNotFound)
)

type handler struct {
	cloudTokenService          cloudtokenSvi.Service
	mountPointService          mountPointSvi.Service
	userMountPointTokenService userMountPointTokenSvi.Service
}

func NewHandler(
	cloudTokenService cloudtokenSvi.Service,
	mountPointService mountPointSvi.Service,
	userMountPointTokenService userMountPointTokenSvi.Service,
) Handler {
	return &handler{
		cloudTokenService:          cloudTokenService,
		mountPointService:          mountPointService,
		userMountPointTokenService: userMountPointTokenService,
	}
}
