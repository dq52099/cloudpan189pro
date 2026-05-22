import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { getSystemInfo } from '@/api/setting'
import { localStg } from '@/utils/storage'
import type { ApiResponse } from '@/utils/api'

type SystemInfo = Models.SystemInfo
type SystemRefreshResult = ApiResponse<SystemInfo> | void

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
  let refreshPromise: Promise<SystemRefreshResult> | null = null

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

    Object.assign(systemInfo, normalizedSystemInfo)
  }

  const store = (_systemInfo: unknown) => {
    const normalizedSystemInfo = normalizeSystemInfo(_systemInfo)
    if (!normalizedSystemInfo) {
      return false
    }

    localStg.set('systemInfo', normalizedSystemInfo)
    Object.assign(systemInfo, normalizedSystemInfo)

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

          loaded.value = true
        } else {
          error.value = response.msg || '获取系统信息失败'
        }

        return response
      })
      .catch((err) => {
        error.value = err instanceof Error ? err.message : '网络错误'
        console.error('获取系统信息失败:', err)
      })
      .finally(() => {
        loading.value = false
        refreshPromise = null
      })

    const promise = refreshPromise

    return promise
  }

  const ensureLoaded = () => {
    if (loaded.value) {
      return Promise.resolve()
    }

    return refresh().then(() => undefined)
  }

  const get = () => systemInfo

  return {
    loading,
    error,

    load,
    refresh,
    ensureLoaded,
    get,
  }
})
