import type { FamilyInfo, FileNode } from '@/api/storage/advance'
import type { SearchResult } from '@/api/subscription'
import type { TelegramUser } from '@/api/telegram'
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
