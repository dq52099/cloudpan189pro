import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { buildLoginRedirectPath, LOGIN_PATH } from '@/utils/redirect'

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

type AuthStoreAccessor = {
  getToken: () => string
  requireRefreshToken: boolean
  hasRefreshToken: boolean
  isLogin: boolean
  doRefreshToken: (force?: boolean) => Promise<string | null>
  logout: () => void
}

let authStoreGetter: (() => AuthStoreAccessor) | null = null
let systemAuthEnabledGetter: (() => boolean) | null = null
let systemAuthRequiredHandler: (() => void) | null = null

const isLoginRequest = (url?: string) => url?.includes('/user/login') ?? false
const isRefreshTokenRequest = (url?: string) => url?.includes('/user/refresh_token') ?? false
const isSystemInfoRequest = (url?: string) => url?.includes('/setting/info') ?? false
const isInitSystemRequest = (url?: string) => url?.includes('/setting/init_system') ?? false
const isAuthBootstrapRequest = (url?: string) =>
  isLoginRequest(url) ||
  isRefreshTokenRequest(url) ||
  isSystemInfoRequest(url) ||
  isInitSystemRequest(url)
const getApiErrorMessage = (data: ApiResponse | undefined, fallback: string) =>
  data?.msg || fallback
const redirectToLogin = () => {
  if (window.location.pathname !== LOGIN_PATH) {
    const currentPath = `${window.location.pathname}${window.location.search}${window.location.hash}`

    window.location.href = buildLoginRedirectPath(currentPath)
  }
}
export const setAuthStoreGetter = (getter: () => AuthStoreAccessor) => {
  authStoreGetter = getter
}

export const setSystemAuthEnabledGetter = (getter: () => boolean) => {
  systemAuthEnabledGetter = getter
}

export const setSystemAuthRequiredHandler = (handler: () => void) => {
  systemAuthRequiredHandler = handler
}

const getAuthStore = () => authStoreGetter?.() ?? null

const isSystemAuthEnabled = () => systemAuthEnabledGetter?.() ?? true

const markSystemAuthRequired = () => {
  systemAuthRequiredHandler?.()
}

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
    if (isAuthBootstrapRequest(config.url)) {
      return config
    }

    if (!isSystemAuthEnabled()) {
      return config
    }

    const authStore = getAuthStore()
    if (!authStore) {
      redirectToLogin()

      return Promise.reject(new Error('登录状态已过期，请重新登录'))
    }

    let token = authStore.getToken()

    if (authStore.requireRefreshToken) {
      const refreshedToken = await authStore.doRefreshToken()
      if (!refreshedToken) {
        redirectToLogin()

        return Promise.reject(new Error('登录状态已过期，请重新登录'))
      }

      token = refreshedToken
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

      switch (status) {
        case 400:
          return Promise.reject(new Error(getApiErrorMessage(data, '请求失败')))
        case 401:
          if (!isAuthBootstrapRequest(config.url)) {
            markSystemAuthRequired()
            const authStore = getAuthStore()
            if (authStore?.hasRefreshToken && !(config as RetriableRequestConfig)._retry) {
              const retryConfig = config as RetriableRequestConfig
              retryConfig._retry = true

              const token = await authStore.doRefreshToken(true)
              if (token) {
                retryConfig.headers.Authorization = `Bearer ${token}`

                return api.request(retryConfig)
              }
            }

            authStore?.logout()
          }

          redirectToLogin()
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
