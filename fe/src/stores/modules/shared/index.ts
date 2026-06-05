import { defineStore } from 'pinia'
import { reactive, watch } from 'vue'
import { localStg } from '@/utils/storage'
import { type StorageType } from '@/types/global.d'

const storageSettingVersion = 2

const defaultStorageSetting = (): StorageType.StorageSetting => ({
  pathPrefix: '/',
  selectedToken: 0,
  version: storageSettingVersion,
})

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const isSafeNonNegativeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

const isIntegerInRange = (value: unknown, min: number, max: number): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= min && value <= max
}

const normalizePathPrefix = (value: unknown) => {
  if (typeof value !== 'string') {
    return '/'
  }

  const pathPrefix = value.trim()
  if (!pathPrefix || /^\/+$/.test(pathPrefix)) {
    return '/'
  }

  const normalized = pathPrefix.startsWith('/') ? pathPrefix : `/${pathPrefix}`

  return normalized.endsWith('/') ? normalized : `${normalized}/`
}

const normalizeRefreshInterval = (value: unknown, version: number) => {
  if (isIntegerInRange(value, 30, 1440)) {
    return value
  }

  if (version < 2 && isIntegerInRange(value, 1, 29)) {
    return 30
  }

  return undefined
}

const migrateStorageSetting = (setting: unknown): StorageType.StorageSetting => {
  if (!isRecord(setting)) {
    return defaultStorageSetting()
  }

  const version = isSafeNonNegativeInteger(setting.version) ? setting.version : 1
  const enableAutoRefresh =
    typeof setting.enableAutoRefresh === 'boolean' ? setting.enableAutoRefresh : false
  const autoRefreshDays = isIntegerInRange(setting.autoRefreshDays, 1, 365)
    ? setting.autoRefreshDays
    : enableAutoRefresh
      ? 7
      : undefined
  const refreshInterval =
    normalizeRefreshInterval(setting.refreshInterval, version) ??
    (enableAutoRefresh ? 60 : undefined)

  const next: StorageType.StorageSetting = {
    pathPrefix: normalizePathPrefix(setting.pathPrefix),
    selectedToken: isSafeNonNegativeInteger(setting.selectedToken) ? setting.selectedToken : 0,
    version: storageSettingVersion,
    enableAutoRefresh,
    autoRefreshDays,
    refreshInterval,
    enableDeepRefresh:
      typeof setting.enableDeepRefresh === 'boolean' ? setting.enableDeepRefresh : false,
  }

  return next
}

export const useSharedStore = defineStore('shared', () => {
  let temporaryPathPrefixRestoreValue: string | null = null

  const load = (): StorageType.StorageSetting => {
    const _storageSetting = localStg.get('storageSetting')
    const normalizedStorageSetting = migrateStorageSetting(_storageSetting)
    localStg.set('storageSetting', normalizedStorageSetting)

    return normalizedStorageSetting
  }

  const persist = (data: StorageType.StorageSetting) => {
    const normalizedStorageSetting = migrateStorageSetting({
      ...data,
      version: storageSettingVersion,
      pathPrefix: temporaryPathPrefixRestoreValue ?? data.pathPrefix,
    })

    localStg.set('storageSetting', normalizedStorageSetting)
  }

  const storageSetting = reactive<StorageType.StorageSetting>(load())

  // 重置路径前缀（不保存到localStorage，用于订阅号挂载）
  const resetPathPrefix = (prefix: string) => {
    temporaryPathPrefixRestoreValue ??= migrateStorageSetting(
      localStg.get('storageSetting')
    ).pathPrefix

    if (storageSetting.pathPrefix === prefix) {
      return
    }

    storageSetting.pathPrefix = prefix
  }

  const restorePathPrefix = () => {
    if (temporaryPathPrefixRestoreValue == null) {
      return
    }

    const pathPrefix = temporaryPathPrefixRestoreValue
    temporaryPathPrefixRestoreValue = null
    storageSetting.pathPrefix = pathPrefix
  }

  // 监听变化保存到localStorage
  watch(
    () => storageSetting,
    (state) => {
      persist(state)
    },
    { deep: true }
  )

  return { storageSetting, resetPathPrefix, restorePathPrefix }
})
