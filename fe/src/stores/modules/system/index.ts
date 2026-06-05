import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { getSystemInfo } from '@/api/setting'
import { localStg, sessionStg } from '@/utils/storage'
import { getErrorMessage, type ApiResponse } from '@/utils/api'

type SystemInfo = Models.SystemInfo
type SystemRefreshResult = ApiResponse<SystemInfo> | void

const authRequiredOverrideKey = 'systemAuthRequiredUntil'
const authRequiredOverrideTTL = 60 * 1000

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const isString = (value: unknown): value is string => typeof value === 'string'

const isBoolean = (value: unknown): value is boolean => typeof value === 'boolean'

const isSafeNonNegativeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

const normalizeSystemInfo = (value: unknown): SystemInfo | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isString(value.baseURL) ||
    !isBoolean(value.enableAuth) ||
    !isBoolean(value.initialized) ||
    !isSafeNonNegativeInteger(value.runTime) ||
    !isString(value.runTimeHuman) ||
    !isString(value.title)
  ) {
    return null
  }

  return {
    baseURL: value.baseURL,
    enableAuth: value.enableAuth,
    initialized: value.initialized,
    runTime: value.runTime,
    runTimeHuman: value.runTimeHuman,
    title: value.title,
  }
}

const getAuthRequiredOverrideUntil = () => {
  const value = sessionStg.get(authRequiredOverrideKey)
  if (value === null) {
    return 0
  }

  if (typeof value !== 'number' || !Number.isFinite(value)) {
    sessionStg.remove(authRequiredOverrideKey)

    return 0
  }

  if (value <= Date.now()) {
    sessionStg.remove(authRequiredOverrideKey)

    return 0
  }

  return value
}

const hasAuthRequiredOverride = () => getAuthRequiredOverrideUntil() > Date.now()

const clearAuthRequiredOverride = () => {
  sessionStg.remove(authRequiredOverrideKey)
}

const applyAuthRequiredOverride = (systemInfo: SystemInfo): SystemInfo => {
  if (!systemInfo.enableAuth && hasAuthRequiredOverride()) {
    return {
      ...systemInfo,
      enableAuth: true,
    }
  }

  return systemInfo
}

// 默认系统信息
const defaultSystemInfo: SystemInfo = {
  initialized: true,
  enableAuth: true,
  title: '云盘分享系统',
  baseURL: 'http://localhost:5173',
  runTime: 62,
  runTimeHuman: '1分2秒',
}

export const useSystemStore = defineStore('system', () => {
  // 系统信息状态
  const systemInfo = reactive<SystemInfo>(defaultSystemInfo)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const loaded = ref(false)
  const remoteLoaded = ref(false)
  let refreshPromise: Promise<SystemRefreshResult> | null = null
  let authRequiredOverrideTimer: ReturnType<typeof setTimeout> | null = null

  const clearAuthRequiredOverrideTimer = () => {
    if (!authRequiredOverrideTimer) {
      return
    }

    clearTimeout(authRequiredOverrideTimer)
    authRequiredOverrideTimer = null
  }

  const restoreCachedSystemInfo = () => {
    const cachedSystemInfo = localStg.get('systemInfo')
    const normalizedSystemInfo = normalizeSystemInfo(cachedSystemInfo)
    if (!normalizedSystemInfo) {
      return false
    }

    Object.assign(systemInfo, normalizedSystemInfo)
    loaded.value = true
    remoteLoaded.value = false
    error.value = null

    return true
  }

  const scheduleAuthRequiredOverrideExpiry = () => {
    clearAuthRequiredOverrideTimer()

    const overrideUntil = getAuthRequiredOverrideUntil()
    if (overrideUntil <= Date.now()) {
      return
    }

    authRequiredOverrideTimer = setTimeout(() => {
      authRequiredOverrideTimer = null

      if (!hasAuthRequiredOverride()) {
        restoreCachedSystemInfo()
        return
      }

      scheduleAuthRequiredOverrideExpiry()
    }, overrideUntil - Date.now())
  }

  const load = () => {
    const _systemInfo = localStg.get('systemInfo')
    if (!_systemInfo) {
      return
    }

    const normalizedSystemInfo = normalizeSystemInfo(_systemInfo)
    if (!normalizedSystemInfo) {
      localStg.remove('systemInfo')

      return
    }

    Object.assign(systemInfo, applyAuthRequiredOverride(normalizedSystemInfo))
    loaded.value = true
    remoteLoaded.value = false
    error.value = null
    scheduleAuthRequiredOverrideExpiry()
  }

  const store = (_systemInfo: unknown) => {
    const normalizedSystemInfo = normalizeSystemInfo(_systemInfo)
    if (!normalizedSystemInfo) {
      return false
    }

    const effectiveSystemInfo = applyAuthRequiredOverride(normalizedSystemInfo)
    localStg.set('systemInfo', normalizedSystemInfo)
    Object.assign(systemInfo, effectiveSystemInfo)
    loaded.value = true
    remoteLoaded.value = true
    error.value = null
    scheduleAuthRequiredOverrideExpiry()

    return true
  }

  const patchCached = (patch: Partial<SystemInfo>) => {
    if (patch.enableAuth === false) {
      clearAuthRequiredOverride()
    }

    const patchedSystemInfo = normalizeSystemInfo({
      ...systemInfo,
      ...patch,
    })

    if (!patchedSystemInfo) {
      return false
    }

    localStg.set('systemInfo', patchedSystemInfo)
    Object.assign(systemInfo, patchedSystemInfo)
    loaded.value = true
    scheduleAuthRequiredOverrideExpiry()

    return true
  }

  const markAuthRequired = () => {
    sessionStg.set(authRequiredOverrideKey, Date.now() + authRequiredOverrideTTL)

    const patchedSystemInfo = normalizeSystemInfo({
      ...systemInfo,
      enableAuth: true,
    })

    if (!patchedSystemInfo) {
      return false
    }

    Object.assign(systemInfo, patchedSystemInfo)
    loaded.value = true
    scheduleAuthRequiredOverrideExpiry()

    return true
  }

  const refresh = (): Promise<SystemRefreshResult> => {
    if (refreshPromise) {
      return refreshPromise
    }

    loading.value = true
    error.value = null

    refreshPromise = getSystemInfo()
      .then((response) => {
        if (response.code === 200 && response.data) {
          if (!store(response.data)) {
            error.value = '获取系统信息失败：响应数据格式异常'

            return response
          }
        } else {
          error.value = response.msg || '获取系统信息失败'
        }

        return response
      })
      .catch((err) => {
        const errorMessage = getErrorMessage(err, '网络错误')

        error.value = errorMessage
        console.error('获取系统信息失败:', errorMessage)
      })
      .finally(() => {
        loading.value = false
        refreshPromise = null
      })

    const promise = refreshPromise

    return promise
  }

  const ensureLoaded = () => {
    if (remoteLoaded.value) {
      return Promise.resolve()
    }

    return refresh().then(() => {
      if (!remoteLoaded.value) {
        throw new Error(error.value || '获取系统信息失败')
      }
    })
  }

  const get = () => systemInfo

  return {
    loading,
    error,
    loaded,
    remoteLoaded,

    load,
    markAuthRequired,
    patchCached,
    refresh,
    ensureLoaded,
    get,
  }
})
