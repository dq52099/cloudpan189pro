import { api, type ApiResponse } from '@/utils/api'

// ===== 任务状态管理相关接口 =====

// 文件任务日志列表查询参数
export interface FileLogListQuery {
  currentPage?: number // 当前页码，默认为1
  pageSize?: number // 每页大小，默认为10
  noPaginate?: boolean // 是否不分页，默认false
  type?: string // 任务类型筛选
  status?: string // 任务状态筛选(pending/running/completed/failed)
  fileId?: number // 文件ID筛选
  userId?: number // 用户ID筛选
  title?: string // 任务标题模糊搜索
  beginAt?: string // 开始时间筛选(格式: 2006-01-02T15:04:05Z07:00)
  endAt?: string // 结束时间筛选(格式: 2006-01-02T15:04:05Z07:00)
}

// 文件任务日志列表响应接口
export interface FileLogListResponse {
  total: number // 总记录数
  currentPage: number // 当前页码
  pageSize: number // 每页大小
  data: Models.FileTaskLog[] // 任务日志列表数据
}

// 任务引擎状态响应接口
export interface TaskEngineListResponse {
  isRunning: boolean // 引擎是否正在运行
  stats: Models.TaskStats // 任务引擎统计信息
  pendingTasks: Models.TaskInfo[] // 待处理的任务列表
  runningTasks: Models.TaskInfo[] // 正在运行的任务列表
}

// ===== 任务状态管理接口 =====

// 获取文件任务日志列表
export const getFileLogList = (
  params?: FileLogListQuery
): Promise<ApiResponse<FileLogListResponse>> => {
  return api.get('/task_state/file_log/list', { params }).then((res) => res.data)
}

// 获取任务引擎状态
export const getTaskEngineList = (): Promise<ApiResponse<TaskEngineListResponse>> => {
  return api.get('/task_state/task_engine/list').then((res) => res.data)
}

// 清空任务日志请求参数
export interface ClearTaskLogsRequest {
  duration?: number // 保留最近多少小时的日志，不传则清空所有
}

// 清空任务日志
export const clearTaskLogs = (
  params?: ClearTaskLogsRequest
): Promise<ApiResponse> => {
  return api.post('/task_state/file_log/clear', params).then((res) => res.data)
}
