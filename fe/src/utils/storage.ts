import localforage from 'localforage'
import type { StorageType } from '@/types/global'

/** The storage driver (值域) */
export type StorageDriver = 'local' | 'session'

function createStorage<T extends object>(
  type: StorageDriver,
  storagePrefix: string,
  clearableKeys: readonly (keyof T)[] = []
) {
  const stg = type === 'session' ? window.sessionStorage : window.localStorage
  const getStorageKey = (key: keyof T) => `${storagePrefix}${key as string}`
  const getPrefixedStorageKeys = () => {
    const keys: string[] = []
    if (!storagePrefix) {
      return keys
    }

    for (let i = 0; i < stg.length; i += 1) {
      const key = stg.key(i)
      if (key?.startsWith(storagePrefix)) {
        keys.push(key)
      }
    }

    return keys
  }

  const storage = {
    /**
     * Set session
     *
     * @param key Session key
     * @param value Session value
     */
    set<K extends keyof T>(key: K, value: T[K]) {
      try {
        const json = JSON.stringify(value)

        stg.setItem(getStorageKey(key), json)
      } catch (error) {
        console.error('写入本地存储失败:', error)
      }
    },
    /**
     * Get session
     *
     * @param key Session key
     */
    get<K extends keyof T>(key: K): T[K] | null {
      const storageKey = getStorageKey(key)

      try {
        const json = stg.getItem(storageKey)
        if (json === null) {
          return null
        }

        return JSON.parse(json) as T[K]
      } catch {
        console.error('读取本地存储失败')
        storage.remove(key)

        return null
      }
    },
    remove(key: keyof T) {
      try {
        stg.removeItem(getStorageKey(key))
      } catch (error) {
        console.error('移除本地存储失败:', error)
      }
    },
    clear() {
      try {
        if (clearableKeys.length > 0) {
          clearableKeys.forEach((key) => {
            stg.removeItem(getStorageKey(key))
          })

          return
        }

        getPrefixedStorageKeys().forEach((key) => {
          stg.removeItem(key)
        })
      } catch (error) {
        console.error('清空本地存储失败:', error)
      }
    },
  }
  return storage
}

type LocalForage<T extends object> = Omit<
  typeof localforage,
  'getItem' | 'setItem' | 'removeItem'
> & {
  getItem<K extends keyof T>(
    key: K,
    callback?: (err: unknown, value: T[K] | null) => void
  ): Promise<T[K] | null>

  setItem<K extends keyof T>(
    key: K,
    value: T[K],
    callback?: (err: unknown, value: T[K]) => void
  ): Promise<T[K]>

  removeItem(key: keyof T, callback?: (err: unknown) => void): Promise<void>
}

type LocalforageDriver = 'local' | 'indexedDB' | 'webSQL'

function createLocalforage<T extends object>(driver: LocalforageDriver) {
  const driverMap: Record<LocalforageDriver, string> = {
    local: localforage.LOCALSTORAGE,
    indexedDB: localforage.INDEXEDDB,
    webSQL: localforage.WEBSQL,
  }

  localforage.config({
    driver: driverMap[driver],
  })

  return localforage as LocalForage<T>
}

const storagePrefix = import.meta.env.VITE_STORAGE_PREFIX || ''

const localStorageKeys = [
  'token',
  'refreshToken',
  'expireTime',
  'user',
  'systemInfo',
  'theme',
  'storageSetting',
  'pageAutoRefreshSetting',
] as const satisfies readonly (keyof StorageType.Local)[]

export const localStg = createStorage<StorageType.Local>('local', storagePrefix, localStorageKeys)

export const sessionStg = createStorage<StorageType.Session>('session', storagePrefix)

export const localforages = createLocalforage<StorageType.Local>('local')
