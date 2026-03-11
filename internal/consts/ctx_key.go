package consts

const (
	CtxKeyUserId      = "__ctx_user_id__"
	CtxKeyUsername    = "__ctx_username__"
	CtxKeyIsAdmin     = "__ctx_is_admin__"
	CtxKeyUserGroupId = "__ctx_user_group_id__"

	CtxKeyInvokeHandlerName = "__ctx_invoke_handler_name__"
	CtxKeyFullPath          = "__ctx_full_path__"

	// CtxKeyFileFullPath 表示当前文件的完整路径 与 CtxKeyFullPath 区别：
	// CtxKeyFullPath 表示当前处理的文件的入口文件
	// CtxKeyFileFullPath  表示当前处理的文件的完整路径 递归调用时，会更新为当前文件的完整路径
	CtxKeyFileFullPath = "__ctx_file_full_path__"

	// CtxKeyTaskTracker 任务跟踪器，用于批量任务汇总进度
	CtxKeyTaskTracker = "__ctx_task_tracker__"
)
