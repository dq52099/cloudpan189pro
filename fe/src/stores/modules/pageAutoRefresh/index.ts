import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { localStg } from '@/utils/storage'
import type { StorageType } from '@/types/global'

const DEFAULT_SETTINGS: StorageType.PageAutoRefreshSetting = {
  autoRefreshEnabled: true, // 默认开启
  refreshInterval: 30, // 默认30秒
}
const REFRESH_INTERVAL_VALUES = [30, 60, 180, 300, 600] as const

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const isRefreshInterval = (value: unknown): value is number => {
  return (
    typeof value === 'number' &&
    Number.isSafeInteger(value) &&
    (REFRESH_INTERVAL_VALUES as readonly number[]).includes(value)
  )
}

const normalizeSettings = (
  value: unknown,
  fallback: StorageType.PageAutoRefreshSetting = DEFAULT_SETTINGS
): StorageType.PageAutoRefreshSetting => {
  if (!isRecord(value)) {
    return { ...fallback }
  }

  return {
    autoRefreshEnabled:
      typeof value.autoRefreshEnabled === 'boolean'
        ? value.autoRefreshEnabled
        : fallback.autoRefreshEnabled,
    refreshInterval: isRefreshInterval(value.refreshInterval)
      ? value.refreshInterval
      : fallback.refreshInterval,
  }
}

export const usePageAutoRefreshStore = defineStore('pageAutoRefresh', () => {
  // 从localStg读取设置
  const loadSettings = (): StorageType.PageAutoRefreshSetting => {
    return normalizeSettings(localStg.get('pageAutoRefreshSetting'))
  }

  // 保存设置到localStg
  const saveSettings = (settings: StorageType.PageAutoRefreshSetting) => {
    localStg.set('pageAutoRefreshSetting', settings)
  }

  // 响应式状态
  const autoRefreshEnabled = ref(DEFAULT_SETTINGS.autoRefreshEnabled)
  const refreshInterval = ref(DEFAULT_SETTINGS.refreshInterval)

  // 加载设置
  const load = () => {
    const settings = loadSettings()
    autoRefreshEnabled.value = settings.autoRefreshEnabled
    refreshInterval.value = settings.refreshInterval
    saveSettings(settings)
  }

  // 更新设置
  const updateSettings = (settings: Partial<StorageType.PageAutoRefreshSetting>) => {
    const currentSettings = getCurrentSettings()
    const nextSettings = normalizeSettings({ ...currentSettings, ...settings }, currentSettings)

    if (settings.autoRefreshEnabled !== undefined) {
      autoRefreshEnabled.value = nextSettings.autoRefreshEnabled
    }
    if (settings.refreshInterval !== undefined) {
      refreshInterval.value = nextSettings.refreshInterval
    }
  }

  // 重置为默认设置
  const resetSettings = () => {
    autoRefreshEnabled.value = DEFAULT_SETTINGS.autoRefreshEnabled
    refreshInterval.value = DEFAULT_SETTINGS.refreshInterval
    saveSettings(DEFAULT_SETTINGS)
  }

  // 获取当前设置
  const getCurrentSettings = (): StorageType.PageAutoRefreshSetting => ({
    autoRefreshEnabled: autoRefreshEnabled.value,
    refreshInterval: refreshInterval.value,
  })

  // 监听变化并自动保存
  watch(
    [autoRefreshEnabled, refreshInterval],
    () => {
      saveSettings(getCurrentSettings())
    },
    { deep: true }
  )

  // 初始化时加载设置
  load()

  return {
    // 状态
    autoRefreshEnabled,
    refreshInterval,

    // 方法
    load,
    updateSettings,
    resetSettings,
    getCurrentSettings,

    // 常量
    DEFAULT_SETTINGS,
  }
})
