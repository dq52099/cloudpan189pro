export namespace StorageType {
  export type Session = Record<string, unknown>
  export type StorageSetting = {
    pathPrefix: string
    selectedToken: number
    version?: number
    enableAutoRefresh?: boolean
    autoRefreshDays?: number
    refreshInterval?: number
    enableDeepRefresh?: boolean
  }

  export type PageAutoRefreshSetting = {
    autoRefreshEnabled: boolean
    refreshInterval: number
  }

  export interface Local {
    token: string
    refreshToken: string
    expireTime: number
    user: Models.User
    systemInfo: Models.SystemInfo
    storageSetting: StorageType.StorageSetting
    pageAutoRefreshSetting: StorageType.PageAutoRefreshSetting
  }
}
