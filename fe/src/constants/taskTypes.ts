// 任务类型常量定义
// 对应后端 internal/types/topic/consts.go

export const TASK_TYPES = {
  FILE_SCAN: 'topic::file::scan::file',
  FILE_CLEAR: 'topic::file::clear::file',
  FILE_BATCH_DELETE: 'topic::file::batch_delete::file',
  FILE_DELETE: 'topic::file::delete::file',

  // 自动入库
  AUTO_INGEST_REFRESH_SUBSCRIBE: 'topic::autoingest::refresh::subscribe',

  // 媒体相关
  MEDIA_CLEAR: 'topic::media::clear',
  MEDIA_REBUILD_STRM: 'topic::media::rebuild::strm::file',
  MEDIA_REBUILD_STRM_BY_MOUNTPOINT: 'topic::media::rebuild::strm::file::by::mountpoint',
} as const

// 任务类型选项配置
export const TASK_TYPE_OPTIONS = [
  { label: '文件扫描', value: TASK_TYPES.FILE_SCAN },
  { label: '文件清空', value: TASK_TYPES.FILE_CLEAR },
  { label: '批量删除', value: TASK_TYPES.FILE_BATCH_DELETE },
  { label: '文件删除', value: TASK_TYPES.FILE_DELETE },
  { label: '自动入库刷新', value: TASK_TYPES.AUTO_INGEST_REFRESH_SUBSCRIBE },
  { label: '媒体清理', value: TASK_TYPES.MEDIA_CLEAR },
  { label: 'STRM重建', value: TASK_TYPES.MEDIA_REBUILD_STRM },
  { label: 'STRM重建(挂载点)', value: TASK_TYPES.MEDIA_REBUILD_STRM_BY_MOUNTPOINT },
]

// 任务类型显示文本映射
export const TASK_TYPE_TEXT_MAP = {
  [TASK_TYPES.FILE_SCAN]: '文件扫描',
  [TASK_TYPES.FILE_CLEAR]: '文件清空',
  [TASK_TYPES.FILE_BATCH_DELETE]: '批量删除',
  [TASK_TYPES.FILE_DELETE]: '文件删除',
  [TASK_TYPES.AUTO_INGEST_REFRESH_SUBSCRIBE]: '自动入库刷新',
  [TASK_TYPES.MEDIA_CLEAR]: '媒体清理',
  [TASK_TYPES.MEDIA_REBUILD_STRM]: 'STRM重建',
  [TASK_TYPES.MEDIA_REBUILD_STRM_BY_MOUNTPOINT]: 'STRM重建(挂载点)',

  // 兼容后端直接使用的类型字符串（无 topic:: 前缀）
  批量删除: '批量删除',
  'media.rebuild_strm': 'STRM重建',
  'media.rebuild_strm_file': 'STRM重建',
  STRM重建: 'STRM重建',
  文件扫描: '文件扫描',
  文件清空: '文件清空',
  文件删除: '文件删除',
  自动入库刷新: '自动入库刷新',
  媒体清理: '媒体清理',
} as const

// 任务类型标签类型映射
export const TASK_TYPE_TAG_MAP = {
  [TASK_TYPES.FILE_SCAN]: 'info',
  [TASK_TYPES.FILE_CLEAR]: 'warning',
  [TASK_TYPES.FILE_BATCH_DELETE]: 'error',
  [TASK_TYPES.FILE_DELETE]: 'error',
  [TASK_TYPES.AUTO_INGEST_REFRESH_SUBSCRIBE]: 'success',
  [TASK_TYPES.MEDIA_CLEAR]: 'warning',
  [TASK_TYPES.MEDIA_REBUILD_STRM]: 'info',
  [TASK_TYPES.MEDIA_REBUILD_STRM_BY_MOUNTPOINT]: 'info',

  // 兼容映射
  批量删除: 'error',
  'media.rebuild_strm': 'info',
  'media.rebuild_strm_file': 'info',
  STRM重建: 'info',
  文件扫描: 'info',
  文件清空: 'warning',
  文件删除: 'error',
  自动入库刷新: 'success',
  媒体清理: 'warning',
} as const

// 任务状态常量定义
export const TASK_STATUS = {
  PENDING: 'pending',
  RUNNING: 'running',
  COMPLETED: 'completed',
  FAILED: 'failed',
} as const

// 任务状态选项配置
export const TASK_STATUS_OPTIONS = [
  { label: '待处理', value: TASK_STATUS.PENDING },
  { label: '运行中', value: TASK_STATUS.RUNNING },
  { label: '已完成', value: TASK_STATUS.COMPLETED },
  { label: '失败', value: TASK_STATUS.FAILED },
]

// 任务状态显示文本映射
export const TASK_STATUS_TEXT_MAP = {
  [TASK_STATUS.PENDING]: '待处理',
  [TASK_STATUS.RUNNING]: '运行中',
  [TASK_STATUS.COMPLETED]: '已完成',
  [TASK_STATUS.FAILED]: '失败',
} as const

// 任务状态标签类型映射
export const TASK_STATUS_TAG_MAP = {
  [TASK_STATUS.PENDING]: 'warning',
  [TASK_STATUS.RUNNING]: 'info',
  [TASK_STATUS.COMPLETED]: 'success',
  [TASK_STATUS.FAILED]: 'error',
} as const

// 任务类型定义
export type TaskType = (typeof TASK_TYPES)[keyof typeof TASK_TYPES]

// 任务状态定义
export type TaskStatus = (typeof TASK_STATUS)[keyof typeof TASK_STATUS]
