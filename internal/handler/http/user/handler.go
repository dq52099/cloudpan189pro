package user

import (
	"net/http"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	loginlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/loginlog"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	usergroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	"github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
)

type Handler interface {
	Add() httpcontext.HandlerFunc
	Login() httpcontext.HandlerFunc
	RefreshToken() httpcontext.HandlerFunc
	Del() httpcontext.HandlerFunc
	Update() httpcontext.HandlerFunc
	ToggleStatus() httpcontext.HandlerFunc
	List() httpcontext.HandlerFunc
	ModifyPass() httpcontext.HandlerFunc
	BindGroup() httpcontext.HandlerFunc
	Info() httpcontext.HandlerFunc
	ModifyOwnPass() httpcontext.HandlerFunc

	RecordLog(eventType loginlog.Event) httpcontext.HandlerFunc
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeUserStartCode)

var (
	codeAddUserFailed       = bi.Next("用户添加失败")
	codeLoginFailed         = bi.Next("登录失败")
	codeUserDisabled        = bi.Next("用户被禁用")
	codeTokenGenerateFailed = bi.Next("Token生成失败")
	codeRefreshTokenInvalid = bi.Next("刷新令牌无效")
	codeUserInfoUpdated     = bi.Next("用户信息已更新")
	codeDelUserFailed       = bi.Next("用户删除失败")
	codeUpdateUserFailed    = bi.Next("用户更新失败")
	codeListUserFailed      = bi.Next("用户列表获取失败")
	codeModifyPassFailed    = bi.Next("密码修改失败")
	codeBindGroupFailed     = bi.Next("绑定用户组失败")
	codeUserInfoFailed      = bi.Next("用户信息获取失败")
	codeNoUpdateFields      = bi.Next("请填写需要更新的字段")
	codeUserPasswordFailed  = bi.Next("用户密码错误")
	codeUserNotFound        = bi.Next("用户不存在")
	codeUserResourceMissing = bi.Next("用户不存在").WithHTTPCode(http.StatusNotFound)
	codeUserGroupMissing    = bi.Next("用户组不存在").WithHTTPCode(http.StatusNotFound)
)

type handler struct {
	userService      userSvi.Service
	userGroupService usergroupSvi.Service
	loginLogService  loginlogSvi.Service
}

func NewHandler(userService userSvi.Service, userGroupService usergroupSvi.Service, loginLogService loginlogSvi.Service) Handler {
	return &handler{
		userService:      userService,
		userGroupService: userGroupService,
		loginLogService:  loginLogService,
	}
}
