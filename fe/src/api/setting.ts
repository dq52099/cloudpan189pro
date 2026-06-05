import { api, type ApiResponse } from '@/utils/api'

// 系统初始化请求接口
export interface InitSystemRequest {
  baseURL: string // 系统基础URL
  enableAuth: boolean // 是否启用认证
  superUsername: string // 超级管理员用户名，长度3-20位
  superPassword: string // 超级管理员密码，长度6-20位
  title: string // 系统标题
}

// 修改系统标题请求接口
export interface ModifyTitleRequest {
  title: string // 新的系统标题
}

// 修改系统基础URL请求接口
export interface ModifyBaseURLRequest {
  baseURL: string // 新的系统基础URL
}

// ===== 系统设置接口 =====

// 获取系统信息
export const getSystemInfo = (): Promise<ApiResponse<Models.SystemInfo>> => {
  return api.get('/setting/info').then((res) => res.data)
}

// 初始化系统
export const initSystem = (data: InitSystemRequest): Promise<ApiResponse> => {
  return api.post('/setting/init_system', data).then((res) => res.data)
}

// 修改系统标题（管理员权限）
export const modifySystemTitle = (title: string): Promise<ApiResponse> => {
  const data: ModifyTitleRequest = { title }
  return api.post('/setting/modify_title', data).then((res) => res.data)
}

// 修改系统基础URL（管理员权限）
export const modifySystemBaseURL = (baseURL: string): Promise<ApiResponse> => {
  const data: ModifyBaseURLRequest = { baseURL }
  return api.post('/setting/modify_base_url', data).then((res) => res.data)
}

// 切换系统鉴权开关（管理员权限）
export interface ToggleEnableAuthRequest {
  enableAuth: boolean // 是否启用鉴权
}

export const toggleSystemEnableAuth = (enableAuth: boolean): Promise<ApiResponse> => {
  const data: ToggleEnableAuthRequest = { enableAuth }
  return api.post('/setting/toggle_enable_auth', data).then((res) => res.data)
}

// 查询系统附加设置（仅登录用户）
export const getSettingAddition = (): Promise<ApiResponse<Models.SettingAddition>> => {
  return api.get('/setting/addition').then((res) => res.data)
}

// 可选修改系统附加设置（管理员权限）
export interface ModifySettingAdditionRequest {
  localProxy?: boolean
  localProxyURL?: string
  multipleStream?: boolean
  multipleStreamThreadCount?: number
  multipleStreamChunkSize?: number
  taskThreadCount?: number
  workerCount?: number
  enableStorageAutoRefresh?: boolean
  webdavUserStrmOnly?: boolean
  webdavAllowedSuffixes?: string[]
}

export const modifySettingAddition = (data: ModifySettingAdditionRequest): Promise<ApiResponse> => {
  return api.post('/setting/modify_addition', data).then((res) => res.data)
}
