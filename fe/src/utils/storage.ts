import localforage from 'localforage'
import type { StorageType } from '@/types/global'

/** The storage driver (值域) */
export type StorageDriver = 'local' | 'session'

function createStorage<T extends object>(type: StorageDriver, storagePrefix: string) {
  const stg = type === 'session' ? window.sessionStorage : window.localStorage

  const storage = {
    /**
     * Set session
     *
     * @param key Session key
     * @param value Session value
     */
    set<K extends keyof T>(key: K, value: T[K]) {
      const json = JSON.stringify(value)

      stg.setItem(`${storagePrefix}${key as string}`, json)
    },
    /**
     * Get session
     *
     * @param key Session key
     */
    get<K extends keyof T>(key: K): T[K] | null {
      const json = stg.getItem(`${storagePrefix}${key as string}`)
      if (json === null) {
        return null
      }

      try {
        return JSON.parse(json) as T[K]
      } catch (error) {
        console.error('解析session失败:', error)
        stg.removeItem(`${storagePrefix}${key as string}`)

        return null
      }
    },
    remove(key: keyof T) {
      stg.removeItem(`${storagePrefix}${key as string}`)
    },
    clear() {
      stg.clear()
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

export const localStg = createStorage<StorageType.Local>('local', storagePrefix)

export const sessionStg = createStorage<StorageType.Session>('session', storagePrefix)

export const localforages = createLocalforage<StorageType.Local>('local')
