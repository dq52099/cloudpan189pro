import type {
  FamilyInfo,
  FileNode,
  GetSubscribeUserAllResponse,
  GetSubscribeUserResponse,
  ShareInfo,
  ShareResourceInfo,
} from '@/api/storage/advance'
import type { ConfigInfoResponse } from '@/api/media'
import type {
  CategoriesResponse,
  CategoryOption,
  HotMovieItem,
  HotMoviesResponse,
  SearchResult,
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
