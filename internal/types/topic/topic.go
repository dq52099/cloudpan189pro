package topic

import (
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
)

type Request interface {
	Topic() taskengine.Topic
}

type FileScanFileRequest struct {
	FileId int64 `json:"fileId"` // 入口地址
	Deep   bool  `json:"deep"`   // 是否深度扫描
}

func (r FileScanFileRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyFileScanFile)
}

type FileClearFileRequest struct {
	FileId int64 `json:"fileId"` // 入口地址
}

func (r FileClearFileRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyFileClearFile)
}

type AutoIngestRefreshSubscribeRequest struct {
	PlanId           int64 `json:"planId"`
	IsRetry          bool  `json:"isRetry"`                    // 是否是重试操作
	ExpectedUserID   int64 `json:"expectedUserId,omitempty"`   // HTTP 触发时的计划归属快照，0 表示系统任务或旧任务
	TriggeredByAdmin bool  `json:"triggeredByAdmin,omitempty"` // 是否由管理员手动触发
}

func (r AutoIngestRefreshSubscribeRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyAutoIngestRefreshSubscribe)
}

type MediaClearRequest struct{}

func (r MediaClearRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyMediaClear)
}

type MediaRebuildStrmFileRequest struct{}

func (r MediaRebuildStrmFileRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyMediaRebuildStrmFile)
}

// 单个挂载点STRM重建请求
type MediaRebuildStrmFileByMountPointRequest struct {
	MountPointFileId int64  `json:"mountPointFileId"`
	MountPointPath   string `json:"mountPointPath"`
}

func (r MediaRebuildStrmFileByMountPointRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyMediaRebuildStrmFileByMountPoint)
}

// 2. 定义请求结构体
type FileBatchDeleteRequest struct {
	IDs              []int64 `json:"ids"`
	ExpectedUserID   int64   `json:"expectedUserId,omitempty"`
	TriggeredByAdmin bool    `json:"triggeredByAdmin,omitempty"`
}

// 3. 实现接口
func (r FileBatchDeleteRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyFileBatchDelete)
}

// 单个文件删除请求
type FileDeleteRequest struct {
	FileId           int64 `json:"fileId"`
	ExpectedUserID   int64 `json:"expectedUserId,omitempty"`
	TriggeredByAdmin bool  `json:"triggeredByAdmin,omitempty"`
}

func (r FileDeleteRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyFileDelete)
}

type FileBatchModifyTokenRequest struct {
	IDs           []int64 `json:"ids"`
	MountPointIDs []int64 `json:"mountPointIds,omitempty"`
	TokenID       int64   `json:"tokenId"`
	UserID        int64   `json:"userId"`
	IsAdmin       bool    `json:"isAdmin"`
	UserGroupID   int64   `json:"userGroupId"`
}

func (r FileBatchModifyTokenRequest) Topic() taskengine.Topic {
	return taskengine.Topic(KeyFileBatchModifyToken)
}

// 批量解析文本请求 (仅用于 API，不用于 Task)
type BatchParseTextRequest struct {
	Content    string `json:"content" binding:"required"`    // 文本内容
	CloudToken int64  `json:"cloudToken" binding:"required"` // 需要用到token去查询信息
	UserID     int64  `json:"-"`                             // 当前用户ID
	IsAdmin    bool   `json:"-"`                             // 是否管理员
}

// 批量解析响应项 (仅用于 API，不用于 Task)
type BatchParseItem struct {
	Name            string `json:"name"`            // 识别出的名称
	OsType          string `json:"osType"`          // 识别出的类型: share_folder 或 person_folder
	ShareCode       string `json:"shareCode"`       // 分享码
	ShareAccessCode string `json:"shareAccessCode"` // 提取码
	FileId          string `json:"fileId"`          // 文件夹ID
	SubscribeUser   string `json:"subscribeUser"`   // 订阅号用户ID
}
