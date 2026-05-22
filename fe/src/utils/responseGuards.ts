import type { FamilyInfo, FileNode } from '@/api/storage/advance'
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
