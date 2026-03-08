import { api, type ApiResponse } from '@/utils/api'

// ===== 媒体配置（Media Config）相关接口 =====

// 获取媒体配置 - 响应体
export interface ConfigInfoResponse {
  initialized: boolean // 是否已初始化
  config?: Models.MediaConfig // 媒体配置（未初始化则为空）
}

// 初始化媒体配置 - 请求体
export interface ConfigInitRequest {
  enable: boolean // 是否启用媒体服务
  storagePath: string // 落盘根路径，例如：/opt/media
  autoClean: boolean // 自动清理空文件夹
  conflictPolicy?: Enums.MediaFileConflictPolicy // 冲突策略：skip/replace
  baseURL: string // 媒体基础URL，例如：http://localhost:12395
  includedSuffixes?: string[] // 包括的后缀格式，例如：['.mp4','.mkv','.avi']
  autoRebuildEnable?: boolean // 定时重建strm开关
  autoRebuildCron?: string // 定时重建cron表达式
}

// 更新媒体配置 - 请求体（任意字段可选）
export interface ConfigUpdateRequest {
  enable?: boolean
  storagePath?: string
  autoClean?: boolean
  conflictPolicy?: Enums.MediaFileConflictPolicy
  baseURL?: string
  includedSuffixes?: string[] // 包括的后缀格式，例如：['.mp4','.mkv','.avi']
  autoRebuildEnable?: boolean // 定时重建strm开关
  autoRebuildCron?: string // 定时重建cron表达式
}

// 切换启用状态 - 请求体
export interface ConfigToggleRequest {
  enable: boolean
}

// ===== 媒体配置管理接口（管理员权限） =====

// 获取媒体配置（用于判断是否需要初始化）
export const getMediaConfigInfo = (): Promise<ApiResponse<ConfigInfoResponse>> => {
  return api.get('/media/config/info').then((res) => res.data)
}

// 初始化媒体配置
export const initMediaConfig = (data: ConfigInitRequest): Promise<ApiResponse> => {
  return api.post('/media/config/init', data).then((res) => res.data)
}

// 更新媒体配置（部分字段）
export const updateMediaConfig = (data: ConfigUpdateRequest): Promise<ApiResponse> => {
  return api.post('/media/config/update', data).then((res) => res.data)
}

// 切换媒体配置启用状态
export const toggleMediaConfig = (data: ConfigToggleRequest): Promise<ApiResponse> => {
  return api.post('/media/config/toggle', data).then((res) => res.data)
}

// ===== 媒体操作（Media Operations）相关接口 =====

// 清理媒体文件 - 清理媒体存储路径下的所有媒体文件
export const clearMediaFiles = (): Promise<ApiResponse> => {
  return api.post('/media/clear').then((res) => res.data)
}

// 重建strm文件 - 扫描所有挂载点并重新生成strm文件
export const rebuildStrmFiles = (): Promise<ApiResponse> => {
  return api.post('/media/rebuild_strm_file').then((res) => res.data)
}
