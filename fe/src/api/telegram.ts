import { api, type ApiResponse } from '@/utils/api'

export interface TelegramSetting {
  id?: number
  enable: boolean
  botToken: string
  botTokenEncrypted?: string
  proxyURL: string
  proxyType: string
  apiURL: string
  chatID: string
  defaultMountPath: string
  enableNotify: boolean
}

export interface TelegramUser {
  userID: number
  username: string
  firstName: string
  lastName: string
  mountPath: string
  isAdmin: boolean
  lastSeenAt: string
  createdAt: string
}

export interface UpdateSettingRequest {
  botToken?: string
  proxyURL?: string
  proxyType?: string
  apiURL?: string
  chatID?: string
  defaultMountPath?: string
  enableNotify?: boolean
  enable?: boolean
}

export interface UpdateUserRequest {
  userID: number
  mountPath: string
  isAdmin: boolean
}

export interface SendMessageRequest {
  message: string
}

export const getTelegramSetting = (): Promise<ApiResponse<TelegramSetting>> => {
  return api.get('/telegram/setting').then((res) => res.data)
}

export const updateTelegramSetting = (
  data: UpdateSettingRequest
): Promise<ApiResponse<TelegramSetting>> => {
  return api.post('/telegram/setting', data).then((res) => res.data)
}

export const testTelegramConnection = (): Promise<ApiResponse<{ message: string }>> => {
  return api.post('/telegram/test').then((res) => res.data)
}

export const getTelegramUsers = (): Promise<ApiResponse<TelegramUser[]>> => {
  return api.get('/telegram/users').then((res) => res.data)
}

export const updateTelegramUser = (data: UpdateUserRequest): Promise<ApiResponse<TelegramUser>> => {
  return api.post('/telegram/user', data).then((res) => res.data)
}

export const sendTelegramMessage = (
  data: SendMessageRequest
): Promise<ApiResponse<{ message: string }>> => {
  return api.post('/telegram/send', data).then((res) => res.data)
}
