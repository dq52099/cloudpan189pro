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

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const isNonEmptyString = (value: unknown): value is string => {
  return typeof value === 'string' && value.trim().length > 0
}

const isPositiveFiniteNumber = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

const isValidExpireTime = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

const normalizeLoginResponse = (value: unknown): LoginResponse | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isNonEmptyString(value.accessToken) ||
    !isNonEmptyString(value.refreshToken) ||
    !isNonEmptyString(value.tokenType) ||
    !isPositiveFiniteNumber(value.expiresIn) ||
    !isRecord(value.user)
  ) {
    return null
  }

  return value as unknown as LoginResponse
}

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

  const storedAccessToken = localStg.get('token')
  const storedRefreshToken = localStg.get('refreshToken')
  const storedExpireTime = localStg.get('expireTime')

  accessToken.value = typeof storedAccessToken === 'string' ? storedAccessToken : ''
  refreshToken.value = typeof storedRefreshToken === 'string' ? storedRefreshToken : ''
  expireTime.value = isValidExpireTime(storedExpireTime) ? storedExpireTime : 0

  if (storedAccessToken !== null && typeof storedAccessToken !== 'string') {
    localStg.remove('token')
  }
  if (storedRefreshToken !== null && typeof storedRefreshToken !== 'string') {
    localStg.remove('refreshToken')
  }
  if (storedExpireTime !== null && !isValidExpireTime(storedExpireTime)) {
    localStg.remove('expireTime')
  }

  const login = (loginData: LoginRequest): Promise<ApiResponse<LoginResponse>> => {
    loading.value = true

    return loginApi(loginData)
      .then((res) => {
        if (res.code !== 200) {
          throw new Error(res.msg || '登录失败，请检查用户名和密码')
        }

        const loginResponse = normalizeLoginResponse(res.data)
        if (!loginResponse || !storeWithUser(loginResponse)) {
          clearSession()

          throw new Error('登录失败：响应数据格式异常')
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
          const loginResponse = normalizeLoginResponse(res.data)
          if (!loginResponse || !storeWithUser(loginResponse)) {
            clearSession()

            return null
          }

          return loginResponse.accessToken
        }

        clearSession()

        return null
      })
      .catch((error) => {
        console.error('刷新登录状态失败:', error)
        clearSession()

        return null
      })
      .finally(() => {
        refreshPromise = null
      })

    return refreshPromise
  }

  const storeWithUser = (loginResponse: LoginResponse) => {
    if (!userStore.store(loginResponse.user)) {
      return false
    }

    store(loginResponse)

    return true
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
