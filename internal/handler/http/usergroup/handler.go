package usergroup

import (
	"net/http"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	usergroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
)

type Handler interface {
	Add() httpcontext.HandlerFunc
	Delete() httpcontext.HandlerFunc
	ModifyName() httpcontext.HandlerFunc
	BatchBindFiles() httpcontext.HandlerFunc
	GetBindFiles() httpcontext.HandlerFunc
	List() httpcontext.HandlerFunc
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeUserGroupStartCode)

var (
	codeAddUserGroupFailed    = bi.Next("用户组添加失败")
	codeDeleteUserGroupFailed = bi.Next("用户组删除失败")
	codeModifyNameFailed      = bi.Next("用户组名称修改失败")
	codeBatchBindFilesFailed  = bi.Next("批量绑定文件失败")
	codeGetBindFilesFailed    = bi.Next("获取绑定文件失败")
	codeListUserGroupFailed   = bi.Next("用户组列表获取失败")
	codeUserGroupNotFound     = bi.Next("用户组不存在").WithHTTPCode(http.StatusNotFound)
	codeBindFileNotFound      = bi.Next("绑定文件不存在").WithHTTPCode(http.StatusNotFound)
)

type handler struct {
	userGroupService  usergroupSvi.Service
	group2FileService group2fileSvi.Service
	userService       userSvi.Service
}

func NewHandler(userGroupService usergroupSvi.Service, group2FileService group2fileSvi.Service, userService userSvi.Service) Handler {
	return &handler{
		userGroupService:  userGroupService,
		group2FileService: group2FileService,
		userService:       userService,
	}
}
