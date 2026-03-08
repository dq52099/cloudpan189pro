import axios from 'axios'
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
  (config) => {
    const authStore = useAuthStore()
    // 从存储获取 token
    const token = authStore.getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
      if (authStore.requireRefreshToken && !config.url?.includes('/user/refresh_token')) {
        authStore.doRefreshToken()
      }
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    const { data } = response

    // 检查 HTTP 状态码
    if (response.status < 200 || response.status >= 300) {
      console.error('请求失败:', data.msg || '请求失败')
      return Promise.reject(new Error(data.msg || '请求失败'))
    }

    return response
  },
  async (error) => {
    if (error.response) {
      const { status, data } = error.response
      const authStore = useAuthStore()
      switch (status) {
        case 400:
          return Promise.reject(new Error(data.msg || '请求失败'))
        case 401:
          window.location.href = '/@login'
          authStore.logout()
          console.error('未授权')
          break
        case 403:
          console.error('权限不足')
          break
        case 404:
          console.error('请求的资源不存在')
          break
        case 500:
          console.error('服务器内部错误')
          break
        default:
          console.error('请求失败:', data?.msg || '请求失败')
      }
    } else if (error.request) {
      console.error('网络错误，请检查网络连接')
    } else {
      console.error('请求配置错误')
    }

    return Promise.reject(error)
  }
)

export default api
