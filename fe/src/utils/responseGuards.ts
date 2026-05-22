import type {
  FamilyInfo,
  FileNode,
  GetSubscribeUserAllResponse,
  GetSubscribeUserResponse,
  ShareInfo,
  ShareResourceInfo,
} from '@/api/storage/advance'
import type { CreateDownloadUrlResponse, FileSearchItem } from '@/api/file'
import type { ConfigInfoResponse } from '@/api/media'
import type { CreateSubscribePlanResponse, PlanLogResult } from '@/api/autoingest'
import type { StorageInfo } from '@/api/storage'
import type {
  AISearchResponse,
  CategoriesResponse,
  CategoryOption,
  HotMovieItem,
  HotMoviesResponse,
  SearchResult,
  SubscriptionConfig,
} from '@/api/subscription'
import type { TelegramSetting, TelegramUser } from '@/api/telegram'
import type { UserGroupInfo } from '@/api/usergroup'

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const isSafePositiveInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

const isSafeNonNegativeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

const isString = (value: unknown): value is string => {
  return typeof value === 'string'
}

const isOptionalString = (value: unknown): value is string | undefined => {
  return value === undefined || isString(value)
}

const isNullableString = (value: unknown): value is string | null => {
  return value === null || isString(value)
}

const isOptionalNullableString = (value: unknown): value is string | null | undefined => {
  return value === undefined || isNullableString(value)
}

const isBoolean = (value: unknown): value is boolean => {
  return typeof value === 'boolean'
}

const isFiniteNumber = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isFinite(value)
}

const normalizeItems = <T>(value: unknown, isValidItem: (item: unknown) => boolean): T[] | null => {
  if (!Array.isArray(value) || !value.every(isValidItem)) {
    return null
  }

  return value as T[]
}

export const normalizeDashboardUsers = (value: unknown): Models.UserInfo[] | null => {
  return normalizeItems<Models.UserInfo>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isString(item.username) &&
      isSafeNonNegativeInteger(item.status) &&
      isBoolean(item.isAdmin) &&
      isSafeNonNegativeInteger(item.groupId) &&
      isOptionalString(item.groupName) &&
      isString(item.createdAt)
    )
  })
}

export const normalizeDashboardUserGroups = (value: unknown): Models.UserGroup[] | null => {
  return normalizeItems<Models.UserGroup>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isString(item.name) &&
      isSafeNonNegativeInteger(item.userCount) &&
      isString(item.createdAt) &&
      isString(item.updatedAt)
    )
  })
}

export const normalizeDashboardCloudTokens = (value: unknown): Models.CloudToken[] | null => {
  return normalizeItems<Models.CloudToken>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isString(item.name) &&
      isString(item.username) &&
      isSafeNonNegativeInteger(item.loginType) &&
      isSafeNonNegativeInteger(item.status) &&
      isSafeNonNegativeInteger(item.expiresIn) &&
      isString(item.updatedAt)
    )
  })
}

export const normalizeLoginLogs = (value: unknown): Models.LoginLog[] | null => {
  return normalizeItems<Models.LoginLog>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isSafeNonNegativeInteger(item.userId) &&
      isString(item.username) &&
      isString(item.addr) &&
      isString(item.location) &&
      isString(item.userAgent) &&
      isString(item.traceId) &&
      isString(item.reason) &&
      isString(item.method) &&
      isString(item.event) &&
      isString(item.status) &&
      isString(item.createdAt) &&
      isString(item.updatedAt)
    )
  })
}

export const normalizeFileTaskLogs = (value: unknown): Models.FileTaskLog[] | null => {
  return normalizeItems<Models.FileTaskLog>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isString(item.title) &&
      isString(item.type) &&
      isString(item.desc) &&
      isString(item.beginAt) &&
      isNullableString(item.endAt) &&
      isString(item.status) &&
      isString(item.result) &&
      isString(item.errorMsg) &&
      isSafeNonNegativeInteger(item.duration) &&
      isSafeNonNegativeInteger(item.fileId) &&
      isSafeNonNegativeInteger(item.userId) &&
      isSafeNonNegativeInteger(item.completed) &&
      isSafeNonNegativeInteger(item.total) &&
      isSafeNonNegativeInteger(item.failed) &&
      isString(item.createdAt) &&
      isString(item.updatedAt)
    )
  })
}

const isOptionalFileTaskLogs = (value: unknown): boolean => {
  return value === undefined || value === null || normalizeFileTaskLogs(value) !== null
}

export const normalizeStorageInfos = (value: unknown): StorageInfo[] | null => {
  return normalizeItems<StorageInfo>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isSafePositiveInteger(item.mountPointId) &&
      isSafeNonNegativeInteger(item.fileId) &&
      isSafeNonNegativeInteger(item.tokenId) &&
      isString(item.name) &&
      isString(item.fullPath) &&
      isString(item.osType) &&
      isBoolean(item.enableAutoRefresh) &&
      isSafeNonNegativeInteger(item.refreshInterval) &&
      isBoolean(item.enableDeepRefresh) &&
      isOptionalNullableString(item.autoRefreshBeginAt) &&
      isSafeNonNegativeInteger(item.autoRefreshDays) &&
      isString(item.lastState) &&
      isString(item.createdAt) &&
      isString(item.updatedAt) &&
      isOptionalString(item.tokenName) &&
      isBoolean(item.isInAutoRefreshPeriod) &&
      isOptionalNullableString(item.nextRefreshTime) &&
      isSafeNonNegativeInteger(item.fileCount) &&
      isOptionalFileTaskLogs(item.taskLogs)
    )
  })
}

const isAutoIngestRefreshStrategy = (value: unknown): value is Models.RefreshStrategy => {
  if (!isRecord(value)) {
    return false
  }

  return (
    isBoolean(value.enableAutoRefresh) &&
    isBoolean(value.enableDeepRefresh) &&
    isSafeNonNegativeInteger(value.autoRefreshDays) &&
    isSafeNonNegativeInteger(value.refreshInterval)
  )
}

export const normalizeAutoIngestPlans = (value: unknown): Models.AutoIngestPlan[] | null => {
  return normalizeItems<Models.AutoIngestPlan>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isString(item.name) &&
      isSafeNonNegativeInteger(item.tokenId) &&
      isString(item.parentPath) &&
      isString(item.sourceType) &&
      isSafeNonNegativeInteger(item.autoIngestInterval) &&
      isString(item.onConflict) &&
      isAutoIngestRefreshStrategy(item.refreshStrategy) &&
      isSafeNonNegativeInteger(item.offset) &&
      isBoolean(item.enabled) &&
      isSafeNonNegativeInteger(item.addCount) &&
      isSafeNonNegativeInteger(item.failedCount) &&
      isString(item.createdAt) &&
      isString(item.updatedAt)
    )
  })
}

const isAutoIngestLogLevel = (value: unknown): value is Models.AutoIngestLog['level'] => {
  return value === 'info' || value === 'warn' || value === 'error'
}

export const normalizePlanLogResults = (value: unknown): PlanLogResult[] | null => {
  return normalizeItems<PlanLogResult>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isSafeNonNegativeInteger(item.planId) &&
      isString(item.planName) &&
      isString(item.content) &&
      isAutoIngestLogLevel(item.level) &&
      isString(item.createdAt) &&
      isString(item.updatedAt)
    )
  })
}

const isOptionalRecord = (value: unknown): boolean => {
  return value === undefined || value === null || (isRecord(value) && !Array.isArray(value))
}

export const normalizeFileSearchItems = (value: unknown): FileSearchItem[] | null => {
  return normalizeItems<FileSearchItem>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.id) &&
      isString(item.cloudId) &&
      isSafeNonNegativeInteger(item.parentId) &&
      isSafeNonNegativeInteger(item.topId) &&
      isBoolean(item.isTop) &&
      isBoolean(item.isDir) &&
      isString(item.name) &&
      isSafeNonNegativeInteger(item.size) &&
      isString(item.hash) &&
      isString(item.osType) &&
      isOptionalRecord(item.addition) &&
      isString(item.rev) &&
      isString(item.createDate) &&
      isString(item.modifyDate) &&
      isString(item.createdAt) &&
      isString(item.updatedAt) &&
      isString(item.fullPath)
    )
  })
}

export const normalizeCreateDownloadUrlResponse = (
  value: unknown
): CreateDownloadUrlResponse | null => {
  if (!isRecord(value) || !isString(value.downloadUrl)) {
    return null
  }

  const downloadUrl = value.downloadUrl.trim()
  if (!downloadUrl) {
    return null
  }

  return { downloadUrl }
}

export const normalizeCloudTokens = (value: unknown): Models.CloudToken[] | null => {
  return normalizeItems<Models.CloudToken>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return isSafePositiveInteger(item.id) && isString(item.name) && isString(item.username)
  })
}

export const normalizeUserGroups = (value: unknown): UserGroupInfo[] | null => {
  return normalizeItems<UserGroupInfo>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return isSafeNonNegativeInteger(item.id) && isString(item.name)
  })
}

export const normalizeFamilies = (value: unknown): FamilyInfo[] | null => {
  return normalizeItems<FamilyInfo>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isString(item.familyId) &&
      isString(item.remarkName) &&
      isString(item.createTime) &&
      isString(item.expireTime) &&
      isSafeNonNegativeInteger(item.count) &&
      isSafeNonNegativeInteger(item.userRole)
    )
  })
}

export const normalizeFileNodes = (value: unknown): FileNode[] | null => {
  return normalizeItems<FileNode>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isString(item.id) &&
      isString(item.name) &&
      isString(item.parentId) &&
      (item.isFolder === 0 || item.isFolder === 1)
    )
  })
}

export const normalizeShareInfo = (value: unknown): ShareInfo | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isString(value.id) ||
    !isString(value.name) ||
    !isFiniteNumber(value.shareId) ||
    !isString(value.shareTime) ||
    !isBoolean(value.isFolder) ||
    !isString(value.accessCode)
  ) {
    return null
  }

  return {
    id: value.id,
    name: value.name,
    shareId: value.shareId,
    shareTime: value.shareTime,
    isFolder: value.isFolder,
    accessCode: value.accessCode,
  }
}

const normalizeShareResourceInfo = (value: unknown): ShareResourceInfo | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isString(value.id) ||
    !isString(value.name) ||
    !isFiniteNumber(value.shareId) ||
    !isString(value.userId) ||
    !isBoolean(value.isFolder) ||
    !isString(value.accessCode) ||
    !isString(value.shareTime)
  ) {
    return null
  }

  return {
    id: value.id,
    name: value.name,
    shareId: value.shareId,
    userId: value.userId,
    isFolder: value.isFolder,
    accessCode: value.accessCode,
    shareTime: value.shareTime,
  }
}

const normalizeShareResourceInfos = (value: unknown): ShareResourceInfo[] | null => {
  if (!Array.isArray(value)) {
    return null
  }

  const items: ShareResourceInfo[] = []
  for (const item of value) {
    const normalizedItem = normalizeShareResourceInfo(item)
    if (!normalizedItem) {
      return null
    }

    items.push(normalizedItem)
  }

  return items
}

export const normalizeGetSubscribeUserResponse = (
  value: unknown
): GetSubscribeUserResponse | null => {
  if (
    !isRecord(value) ||
    !isString(value.name) ||
    !isSafePositiveInteger(value.currentPage) ||
    !isSafePositiveInteger(value.pageSize) ||
    !isSafeNonNegativeInteger(value.total)
  ) {
    return null
  }

  const data = normalizeShareResourceInfos(value.data)
  if (!data) {
    return null
  }

  return {
    name: value.name,
    currentPage: value.currentPage,
    pageSize: value.pageSize,
    total: value.total,
    data,
  }
}

export const normalizeGetSubscribeUserAllResponse = (
  value: unknown
): GetSubscribeUserAllResponse | null => {
  if (!isRecord(value) || !isString(value.name) || !isSafeNonNegativeInteger(value.total)) {
    return null
  }

  const data = normalizeShareResourceInfos(value.data)
  if (!data) {
    return null
  }

  return {
    name: value.name,
    total: value.total,
    data,
  }
}

const normalizeCategoryOptions = (value: unknown): CategoryOption[] | null => {
  return normalizeItems<CategoryOption>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isString(item.value) &&
      isString(item.label) &&
      (item.children === undefined || normalizeCategoryOptions(item.children) !== null)
    )
  })
}

export const normalizeCategoriesResponse = (value: unknown): CategoriesResponse | null => {
  if (!isRecord(value)) {
    return null
  }

  const tmdb = normalizeCategoryOptions(value.tmdb)
  const douban = normalizeCategoryOptions(value.douban)
  if (!tmdb || !douban) {
    return null
  }

  return { tmdb, douban }
}

const normalizeHotMovieItems = (value: unknown): HotMovieItem[] | null => {
  return normalizeItems<HotMovieItem>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafeNonNegativeInteger(item.id) &&
      isString(item.title) &&
      isString(item.originalTitle) &&
      isString(item.year) &&
      isFiniteNumber(item.rating) &&
      isString(item.cover) &&
      isString(item.posterPath) &&
      isString(item.type) &&
      isString(item.description) &&
      isOptionalString(item.category)
    )
  })
}

export const normalizeHotMoviesResponse = (value: unknown): HotMoviesResponse | null => {
  if (!isRecord(value) || !isString(value.source) || !isString(value.category)) {
    return null
  }

  const movies = normalizeHotMovieItems(value.movies)
  if (!movies) {
    return null
  }

  return {
    movies,
    source: value.source,
    category: value.category,
  }
}

export const normalizeSearchResults = (value: unknown): SearchResult[] | null => {
  return normalizeItems<SearchResult>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isString(item.shareUrl) &&
      isString(item.shareCode) &&
      isString(item.name) &&
      isString(item.uploadTime) &&
      isString(item.source) &&
      isOptionalString(item.size) &&
      isOptionalString(item.cover) &&
      isOptionalString(item.note)
    )
  })
}

export const normalizeAISearchResponse = (value: unknown): AISearchResponse | null => {
  if (
    !isRecord(value) ||
    !isString(value.message) ||
    !isString(value.keyword) ||
    !isString(value.aiDescription)
  ) {
    return null
  }

  const resultItems = value.result === null ? [] : normalizeSearchResults([value.result])
  if (!resultItems) {
    return null
  }

  const allResults = normalizeSearchResults(value.allResults)
  if (!allResults) {
    return null
  }

  return {
    message: value.message,
    keyword: value.keyword,
    aiDescription: value.aiDescription,
    result: resultItems[0] ?? null,
    allResults,
  }
}

export const normalizeSubscriptionConfigResponse = (value: unknown): SubscriptionConfig | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isBoolean(value.enableTMDB) ||
    !isBoolean(value.enableDouban) ||
    !isString(value.panSearchURL) ||
    !isString(value.defaultMountPath) ||
    !isBoolean(value.autoMount) ||
    !isString(value.cronExpression) ||
    !isString(value.tmdbAPIKey) ||
    !isString(value.openaiAPIKey) ||
    !isString(value.openaiBaseURL) ||
    !isString(value.openaiModel)
  ) {
    return null
  }

  return {
    enableTMDB: value.enableTMDB,
    enableDouban: value.enableDouban,
    panSearchURL: value.panSearchURL,
    defaultMountPath: value.defaultMountPath,
    autoMount: value.autoMount,
    cronExpression: value.cronExpression,
    tmdbAPIKey: value.tmdbAPIKey,
    openaiAPIKey: value.openaiAPIKey,
    openaiBaseURL: value.openaiBaseURL,
    openaiModel: value.openaiModel,
  }
}

export const normalizeCreateSubscribePlanResponse = (
  value: unknown
): CreateSubscribePlanResponse | null => {
  if (!isRecord(value) || !isSafePositiveInteger(value.id)) {
    return null
  }

  if (
    (value.historyQueued !== undefined && !isBoolean(value.historyQueued)) ||
    !isOptionalString(value.historyError)
  ) {
    return null
  }

  return {
    id: value.id,
    historyQueued: value.historyQueued,
    historyError: value.historyError,
  }
}

const isOptionalSafeNonNegativeInteger = (value: unknown): value is number | undefined => {
  return value === undefined || isSafeNonNegativeInteger(value)
}

export const normalizeTelegramSetting = (value: unknown): TelegramSetting | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isOptionalSafeNonNegativeInteger(value.id) ||
    !isBoolean(value.enable) ||
    !isString(value.botToken) ||
    !isOptionalString(value.botTokenEncrypted) ||
    !isString(value.proxyURL) ||
    !isString(value.proxyType) ||
    !isString(value.apiURL) ||
    !isString(value.chatID) ||
    !isString(value.defaultMountPath) ||
    !isBoolean(value.enableNotify)
  ) {
    return null
  }

  return {
    id: value.id,
    enable: value.enable,
    botToken: value.botToken,
    botTokenEncrypted: value.botTokenEncrypted,
    proxyURL: value.proxyURL,
    proxyType: value.proxyType,
    apiURL: value.apiURL,
    chatID: value.chatID,
    defaultMountPath: value.defaultMountPath,
    enableNotify: value.enableNotify,
  }
}

export const normalizeTelegramUsers = (value: unknown): TelegramUser[] | null => {
  return normalizeItems<TelegramUser>(value, (item) => {
    if (!isRecord(item)) {
      return false
    }

    return (
      isSafePositiveInteger(item.userID) &&
      isString(item.username) &&
      isString(item.firstName) &&
      isString(item.lastName) &&
      isString(item.mountPath) &&
      typeof item.isAdmin === 'boolean' &&
      isString(item.lastSeenAt) &&
      isString(item.createdAt)
    )
  })
}

const isStringArray = (value: unknown): value is string[] => {
  return Array.isArray(value) && value.every(isString)
}

const isMediaFileConflictPolicy = (value: unknown): value is Enums.MediaFileConflictPolicy => {
  return value === 'skip' || value === 'replace'
}

const normalizeMediaConfig = (value: unknown): Models.MediaConfig | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isSafeNonNegativeInteger(value.id) ||
    !isBoolean(value.enable) ||
    !isString(value.storagePath) ||
    !isBoolean(value.autoClean) ||
    !isMediaFileConflictPolicy(value.conflictPolicy) ||
    !isString(value.baseURL) ||
    !isStringArray(value.includedSuffixes) ||
    !isBoolean(value.autoRebuildEnable) ||
    !isString(value.autoRebuildCron)
  ) {
    return null
  }

  return value as unknown as Models.MediaConfig
}

export const normalizeMediaConfigInfoResponse = (value: unknown): ConfigInfoResponse | null => {
  if (!isRecord(value) || !isBoolean(value.initialized)) {
    return null
  }

  if (!value.initialized) {
    return { initialized: false }
  }

  const config = normalizeMediaConfig(value.config)
  if (!config) {
    return null
  }

  return {
    initialized: true,
    config,
  }
}
