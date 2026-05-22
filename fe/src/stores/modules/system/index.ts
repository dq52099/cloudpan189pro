import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { getSystemInfo } from '@/api/setting'
import { localStg } from '@/utils/storage'
import type { ApiResponse } from '@/utils/api'

type SystemInfo = Models.SystemInfo
type SystemRefreshResult = ApiResponse<SystemInfo> | void

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
    if (_systemInfo) {
      Object.assign(systemInfo, _systemInfo)
    }
  }

  const store = (_systemInfo: SystemInfo) => {
    localStg.set('systemInfo', _systemInfo)
    Object.assign(systemInfo, _systemInfo)
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
          store(response.data)
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
