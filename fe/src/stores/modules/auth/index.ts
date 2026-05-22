import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  login as loginApi,
  refreshToken as refreshTokenApi,
  type LoginRequest,
  type LoginResponse,
} from '@/api/auth'
import type { ApiResponse } from '@/utils/api'
import { localStg } from '@/utils/storage'
import { useUserStore } from '../user'

// Token刷新提前时间（5分钟）
const TOKEN_REFRESH_BUFFER = 5 * 60 * 1000
// Token自动刷新阈值（60分钟）
const TOKEN_AUTO_REFRESH_THRESHOLD = 60 * 60 * 1000

export const useAuthStore = defineStore('auth', () => {
  const userStore = useUserStore()

  const accessToken = ref<string>('')
  const refreshToken = ref<string>('')
  const expireTime = ref<number>(0)
  const isAccessTokenValid = computed(() => !!accessToken.value && expireTime.value > Date.now())
  const hasRefreshToken = computed(() => !!refreshToken.value)
  const isLogin = computed(() => isAccessTokenValid.value || hasRefreshToken.value)
  // 需要刷新token（距离过期时间小于60分钟）
  const requireRefreshToken = computed(
    () =>
      hasRefreshToken.value &&
      (!isAccessTokenValid.value || expireTime.value - Date.now() < TOKEN_AUTO_REFRESH_THRESHOLD)
  )

  const loading = ref<boolean>(false)

  let refreshPromise: Promise<string | null> | null = null

  accessToken.value = localStg.get('token') || ''
  refreshToken.value = localStg.get('refreshToken') || ''
  expireTime.value = localStg.get('expireTime') || 0

  const login = (loginData: LoginRequest): Promise<ApiResponse<LoginResponse>> => {
    loading.value = true

    return loginApi(loginData)
      .then((res) => {
        if (res.code === 200 && res.data) {
          storeWithUser(res.data)
        }

        return res
      })
      .finally(() => {
        loading.value = false
      })
  }

  const logout = () => {
    clearSession()
  }

  const doRefreshToken = async (force = false): Promise<string | null> => {
    if (!refreshToken.value) {
      clearSession()

      return null
    }

    if (!force && isAccessTokenValid.value && !requireRefreshToken.value) {
      return accessToken.value
    }

    if (!force && !requireRefreshToken.value) {
      return accessToken.value || null
    }

    if (refreshPromise) {
      return refreshPromise
    }

    refreshPromise = refreshTokenApi({
      refreshToken: refreshToken.value,
    })
      .then((res) => {
        if (res.code === 200 && res.data) {
          storeWithUser(res.data)

          return res.data.accessToken
        }

        clearSession()

        return null
      })
      .catch((error) => {
        clearSession()

        throw error
      })
      .finally(() => {
        refreshPromise = null
      })

    return refreshPromise
  }

  const storeWithUser = (loginResponse: LoginResponse) => {
    store(loginResponse)
    userStore.store(loginResponse.user)
  }

  const store = (data: { accessToken: string; refreshToken: string; expiresIn: number }) => {
    expireTime.value = Date.now() + data.expiresIn * 1000 - TOKEN_REFRESH_BUFFER
    accessToken.value = data.accessToken
    refreshToken.value = data.refreshToken

    localStg.set('token', accessToken.value)
    localStg.set('refreshToken', refreshToken.value)
    localStg.set('expireTime', expireTime.value)
  }

  const clearAuth = () => {
    accessToken.value = ''
    refreshToken.value = ''
    expireTime.value = 0

    localStg.remove('token')
    localStg.remove('refreshToken')
    localStg.remove('expireTime')
  }

  const clearSession = () => {
    clearAuth()
    userStore.clear()
  }

  const getToken = () => {
    return accessToken.value
  }

  return {
    // 状态
    // user,
    // refreshTokenValue,
    loading,
    requireRefreshToken,
    hasRefreshToken,
    isAccessTokenValid,

    // 计算属性
    // isAdmin,
    // username,
    // userId,

    // 方法
    // fetchUserInfo,
    // tryRefreshToken,
    // updateUserInfo,

    doRefreshToken,
    login,
    logout,
    getToken,

    isLogin,
  }
})
