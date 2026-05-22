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

const migrateStorageSetting = (
  setting: StorageType.StorageSetting | null
): StorageType.StorageSetting => {
  if (!setting) {
    return defaultStorageSetting()
  }

  const next: StorageType.StorageSetting = {
    ...defaultStorageSetting(),
    ...setting,
  }

  if ((setting.version ?? 1) < 2 && next.refreshInterval && next.refreshInterval < 30) {
    next.refreshInterval = 30
  }

  next.version = storageSettingVersion

  return next
}

export const useSharedStore = defineStore('shared', () => {
  let temporaryPathPrefixRestoreValue: string | null = null

  const load = (): StorageType.StorageSetting => {
    const _storageSetting = localStg.get('storageSetting')

    return migrateStorageSetting(_storageSetting)
  }

  const persist = (data: StorageType.StorageSetting) => {
    localStg.set('storageSetting', {
      ...data,
      version: storageSettingVersion,
      pathPrefix: temporaryPathPrefixRestoreValue ?? data.pathPrefix,
    })
  }

  const storageSetting = reactive<StorageType.StorageSetting>(load())

  // 重置路径前缀（不保存到localStorage，用于订阅号挂载）
  const resetPathPrefix = (prefix: string) => {
    temporaryPathPrefixRestoreValue ??=
      localStg.get('storageSetting')?.pathPrefix ?? storageSetting.pathPrefix

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
