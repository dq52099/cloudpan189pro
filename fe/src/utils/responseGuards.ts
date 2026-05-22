import type { FamilyInfo, FileNode } from '@/api/storage/advance'
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

const isOptionalString = (value: unknown): value is string | undefined => {
  return value === undefined || isString(value)
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
