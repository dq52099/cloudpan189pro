import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores'

// 响应数据类型
export interface ApiResponse<T = unknown> {
  msg: string
  code: number
  data?: T
}

// 批量操作响应
export interface BatchOperationResponse {
  success: number
  failed: number
}

type RetriableRequestConfig = InternalAxiosRequestConfig & {
  _retry?: boolean
}

const isRefreshTokenRequest = (url?: string) => url?.includes('/user/refresh_token') ?? false
const getApiErrorMessage = (data: ApiResponse | undefined, fallback: string) =>
  data?.msg || fallback

export const getErrorMessage = (error: unknown, fallback: string) => {
  if (error instanceof Error && error.message) {
    return error.message
  }

  if (typeof error === 'string' && error.trim()) {
    return error
  }

  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) {
      return message
    }
  }

  return fallback
}

// 创建 axios 实例
export const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器
api.interceptors.request.use(
  async (config: InternalAxiosRequestConfig) => {
    const authStore = useAuthStore()
    let token = authStore.getToken()

    if (authStore.requireRefreshToken && !isRefreshTokenRequest(config.url)) {
      token = (await authStore.doRefreshToken()) || token
    }

    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }

    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    const { data } = response

    if (response.status < 200 || response.status >= 300) {
      const message = getApiErrorMessage(data, '请求失败')
      console.error('请求失败:', message)
      return Promise.reject(new Error(message))
    }

    return response
  },
  async (error: AxiosError<ApiResponse>) => {
    if (error.code === 'ECONNABORTED') {
      console.error('请求超时，请稍后重试')
      return Promise.reject(new Error('请求超时，请稍后重试'))
    }

    if (error.response) {
      const { status, data, config } = error.response
      const authStore = useAuthStore()

      switch (status) {
        case 400:
          return Promise.reject(new Error(getApiErrorMessage(data, '请求失败')))
        case 401:
          if (
            authStore.hasRefreshToken &&
            !isRefreshTokenRequest(config.url) &&
            !(config as RetriableRequestConfig)._retry
          ) {
            const retryConfig = config as RetriableRequestConfig
            retryConfig._retry = true

            const token = await authStore.doRefreshToken(true)
            if (token) {
              retryConfig.headers.Authorization = `Bearer ${token}`

              return api.request(retryConfig)
            }
          }

          if (authStore.isLogin) {
            authStore.logout()
          }
          window.location.href = '/@login'
          console.error('未登录或会话已过期')
          return Promise.reject(new Error(getApiErrorMessage(data, '未登录或会话已过期')))
        case 403:
          console.error('权限不足')
          return Promise.reject(new Error(getApiErrorMessage(data, '权限不足')))
        case 404:
          console.error('请求的资源不存在')
          return Promise.reject(new Error(getApiErrorMessage(data, '请求的资源不存在')))
        case 500:
          console.error('服务器内部错误')
          return Promise.reject(new Error(getApiErrorMessage(data, '服务器内部错误')))
        default:
          console.error('请求失败:', getApiErrorMessage(data, '请求失败'))
          return Promise.reject(new Error(getApiErrorMessage(data, '请求失败')))
      }
    } else if (error.request) {
      console.error('网络错误，请检查网络连接')
      return Promise.reject(new Error('网络错误，请检查网络连接'))
    } else {
      console.error('请求配置错误')
    }

    return Promise.reject(error)
  }
)

export default api
