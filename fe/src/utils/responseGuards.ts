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
import type { NormalizedTaskEngineListResponse } from '@/api/taskstate'
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

const isBase64ByteString = (value: string): boolean => {
  if (value === '') {
    return true
  }

  if (value.length % 4 !== 0) {
    return false
  }

  return /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value)
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

const isOptionalRecord = (value: unknown): boolean => {
  return value === undefined || value === null || (isRecord(value) && !Array.isArray(value))
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
  if (!Array.isArray(value)) {
    return null
  }

  const items: Models.UserGroup[] = []
  for (const item of value) {
    if (!isRecord(item)) {
      return null
    }

    const userCount = item.userCount ?? 0
    if (
      !isSafePositiveInteger(item.id) ||
      !isString(item.name) ||
      !isSafeNonNegativeInteger(userCount) ||
      !isString(item.createdAt) ||
      !isString(item.updatedAt)
    ) {
      return null
    }

    items.push({
      id: item.id,
      name: item.name,
      userCount,
      createdAt: item.createdAt,
      updatedAt: item.updatedAt,
    })
  }

  return items
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
  if (!Array.isArray(value)) {
    return null
  }

  const logs: Models.FileTaskLog[] = []
  for (const item of value) {
    if (!isRecord(item)) {
      return null
    }

    const desc = item.desc ?? ''
    const endAt = item.endAt ?? null
    const result = item.result ?? ''
    const errorMsg = item.errorMsg ?? ''
    const addition = item.addition ?? {}
    const duration = item.duration ?? 0
    const fileId = item.fileId ?? 0
    const userId = item.userId ?? 0
    const completed = item.completed ?? 0
    const total = item.total ?? 0
    const failed = item.failed ?? 0

    if (
      !isSafePositiveInteger(item.id) ||
      !isString(item.title) ||
      !isString(item.type) ||
      !isString(desc) ||
      !isString(item.beginAt) ||
      !isNullableString(endAt) ||
      !isString(item.status) ||
      !isString(result) ||
      !isString(errorMsg) ||
      !isOptionalRecord(addition) ||
      !isSafeNonNegativeInteger(duration) ||
      !isSafeNonNegativeInteger(fileId) ||
      !isSafeNonNegativeInteger(userId) ||
      !isSafeNonNegativeInteger(completed) ||
      !isSafeNonNegativeInteger(total) ||
      !isSafeNonNegativeInteger(failed) ||
      !isString(item.createdAt) ||
      !isString(item.updatedAt)
    ) {
      return null
    }

    logs.push({
      id: item.id,
      title: item.title,
      type: item.type,
      desc,
      beginAt: item.beginAt,
      endAt,
      status: item.status,
      result,
      errorMsg,
      addition: isRecord(addition) && !Array.isArray(addition) ? addition : {},
      duration,
      fileId,
      userId,
      completed,
      total,
      failed,
      createdAt: item.createdAt,
      updatedAt: item.updatedAt,
    })
  }

  return logs
}

const defaultTaskStats = (): Models.TaskStats => ({
  totalTasks: 0,
  pendingTasks: 0,
  runningTasks: 0,
  completedTasks: 0,
  failedTasks: 0,
  cancelledTasks: 0,
})

export const isCompleteTaskStats = (value: unknown): value is Models.TaskStats => {
  if (!isRecord(value)) {
    return false
  }

  return (
    isSafeNonNegativeInteger(value.totalTasks) &&
    isSafeNonNegativeInteger(value.pendingTasks) &&
    isSafeNonNegativeInteger(value.runningTasks) &&
    isSafeNonNegativeInteger(value.completedTasks) &&
    isSafeNonNegativeInteger(value.failedTasks) &&
    (value.cancelledTasks === undefined || isSafeNonNegativeInteger(value.cancelledTasks))
  )
}

const normalizeTaskStats = (value: unknown): Models.TaskStats => {
  if (!isCompleteTaskStats(value)) {
    return defaultTaskStats()
  }

  return {
    totalTasks: value.totalTasks,
    pendingTasks: value.pendingTasks,
    runningTasks: value.runningTasks,
    completedTasks: value.completedTasks,
    failedTasks: value.failedTasks,
    cancelledTasks: value.cancelledTasks ?? 0,
  }
}

const normalizeTaskPayload = (value: unknown): Models.TaskInfo['payload'] | null => {
  if (value === undefined || value === null) {
    return ''
  }

  if (isString(value)) {
    return isBase64ByteString(value) ? value : null
  }

  if (
    Array.isArray(value) &&
    value.every((item) => Number.isInteger(item) && item >= 0 && item <= 255)
  ) {
    return value
  }

  return null
}

const normalizeProcessorResult = (value: unknown): Models.ProcessorResult | null => {
  if (!isRecord(value)) {
    return null
  }

  const duration = value.duration === undefined || value.duration === null ? 0 : value.duration
  if (
    !isString(value.processorId) ||
    !isString(value.status) ||
    !isOptionalNullableString(value.error) ||
    !isOptionalNullableString(value.startTime) ||
    !isOptionalNullableString(value.endTime) ||
    !isFiniteNumber(duration) ||
    duration < 0
  ) {
    return null
  }

  return {
    processorId: value.processorId,
    status: value.status,
    error: isString(value.error) ? value.error : undefined,
    startTime: isString(value.startTime) ? value.startTime : '',
    endTime: isString(value.endTime) ? value.endTime : '',
    duration,
  }
}

const normalizeProcessorResults = (value: unknown): Models.ProcessorResult[] | null => {
  if (value === undefined || value === null) {
    return []
  }

  if (!Array.isArray(value)) {
    return null
  }

  const results: Models.ProcessorResult[] = []
  for (const item of value) {
    const result = normalizeProcessorResult(item)
    if (!result) {
      return null
    }

    results.push(result)
  }

  return results
}

const normalizeTaskInfo = (value: unknown): Models.TaskInfo | null => {
  if (!isRecord(value)) {
    return null
  }

  const payload = normalizeTaskPayload(value.payload)
  const results = normalizeProcessorResults(value.results)
  if (
    !isString(value.id) ||
    !isString(value.topic) ||
    payload === null ||
    !isString(value.status) ||
    !isOptionalNullableString(value.workerId) ||
    !isOptionalNullableString(value.receiveAt) ||
    !isOptionalNullableString(value.startAt) ||
    !isOptionalNullableString(value.endAt) ||
    !results
  ) {
    return null
  }

  return {
    id: value.id,
    topic: value.topic,
    payload,
    status: value.status,
    workerId: isString(value.workerId) ? value.workerId : '',
    receiveAt: isString(value.receiveAt) ? value.receiveAt : '',
    startAt: isString(value.startAt) ? value.startAt : null,
    endAt: isString(value.endAt) ? value.endAt : null,
    results,
  }
}

const normalizeTaskInfos = (value: unknown): Models.TaskInfo[] | null => {
  if (value === undefined || value === null) {
    return []
  }

  if (!Array.isArray(value)) {
    return null
  }

  const tasks: Models.TaskInfo[] = []
  for (const item of value) {
    const task = normalizeTaskInfo(item)
    if (!task) {
      return null
    }

    tasks.push(task)
  }

  return tasks
}

export const normalizeTaskEngineListResponse = (
  value: unknown
): NormalizedTaskEngineListResponse | null => {
  if (!isRecord(value) || !isBoolean(value.isRunning)) {
    return null
  }

  const pendingTasks = normalizeTaskInfos(value.pendingTasks)
  const runningTasks = normalizeTaskInfos(value.runningTasks)
  if (!pendingTasks || !runningTasks) {
    return null
  }

  return {
    isRunning: value.isRunning,
    stats: normalizeTaskStats(value.stats),
    pendingTasks,
    runningTasks,
  }
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
  if (!Array.isArray(value)) {
    return null
  }

  const families: FamilyInfo[] = []
  for (const item of value) {
    if (!isRecord(item)) {
      return null
    }

    const familyType = item.type === undefined || item.type === null ? 0 : item.type
    const useFlag = item.useFlag === undefined || item.useFlag === null ? 0 : item.useFlag
    if (
      !isString(item.familyId) ||
      !isString(item.remarkName) ||
      !isString(item.createTime) ||
      !isString(item.expireTime) ||
      !isSafeNonNegativeInteger(item.count) ||
      !isSafeNonNegativeInteger(familyType) ||
      !isSafeNonNegativeInteger(useFlag) ||
      !isSafeNonNegativeInteger(item.userRole)
    ) {
      return null
    }

    families.push({
      familyId: item.familyId,
      remarkName: item.remarkName,
      createTime: item.createTime,
      expireTime: item.expireTime,
      count: item.count,
      type: familyType,
      useFlag,
      userRole: item.userRole,
    })
  }

  return families
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

export interface FileNodePage {
  files: FileNode[]
  total: number
  currentPage: number
  pageSize: number
}

export const normalizeFileNodePage = (value: unknown): FileNodePage | null => {
  if (!isRecord(value)) {
    return null
  }

  const files = normalizeFileNodes(value.data)
  if (
    !files ||
    !isSafeNonNegativeInteger(value.total) ||
    !isSafePositiveInteger(value.currentPage) ||
    !isSafePositiveInteger(value.pageSize)
  ) {
    return null
  }

  return {
    files,
    total: value.total,
    currentPage: value.currentPage,
    pageSize: value.pageSize,
  }
}

export const normalizeShareInfo = (value: unknown): ShareInfo | null => {
  if (!isRecord(value)) {
    return null
  }

  const shareMode =
    value.shareMode === undefined || value.shareMode === null || value.shareMode === 0
      ? 1
      : value.shareMode
  if (
    !isString(value.id) ||
    !isString(value.name) ||
    !isFiniteNumber(value.shareId) ||
    !isSafePositiveInteger(shareMode) ||
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
    shareMode,
    shareTime: value.shareTime,
    isFolder: value.isFolder,
    accessCode: value.accessCode,
  }
}

const normalizeShareResourceInfo = (value: unknown): ShareResourceInfo | null => {
  if (!isRecord(value)) {
    return null
  }

  const isTop = value.isTop === undefined || value.isTop === null ? 0 : value.isTop
  let hasShareSource = false
  let accessCode = ''
  if (isString(value.accessCode)) {
    accessCode = value.accessCode
    hasShareSource = true
  }

  let shareUrl = accessCode
  if (isString(value.shareUrl)) {
    shareUrl = value.shareUrl
    hasShareSource = true
  }

  if (
    !isString(value.id) ||
    !isString(value.name) ||
    !isFiniteNumber(value.shareId) ||
    !isString(value.userId) ||
    !isBoolean(value.isFolder) ||
    !hasShareSource ||
    !isString(value.shareTime) ||
    !isSafeNonNegativeInteger(isTop)
  ) {
    return null
  }

  return {
    id: value.id,
    name: value.name,
    shareId: value.shareId,
    userId: value.userId,
    isFolder: value.isFolder,
    accessCode,
    shareUrl,
    shareTime: value.shareTime,
    isTop,
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
      isOptionalString(item.shareAccessCode) &&
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
    !isBoolean(value.enabled) ||
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
    enabled: value.enabled,
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
  if (!Array.isArray(value)) {
    return null
  }

  const items: TelegramUser[] = []
  for (const item of value) {
    if (!isRecord(item)) {
      return null
    }

    const lastName = item.lastName ?? ''
    if (
      !isSafePositiveInteger(item.userID) ||
      !isString(item.username) ||
      !isString(item.firstName) ||
      !isString(lastName) ||
      !isString(item.mountPath) ||
      typeof item.isAdmin !== 'boolean' ||
      !isString(item.lastSeenAt) ||
      !isString(item.createdAt)
    ) {
      return null
    }

    items.push({
      userID: item.userID,
      username: item.username,
      firstName: item.firstName,
      lastName,
      mountPath: item.mountPath,
      isAdmin: item.isAdmin,
      lastSeenAt: item.lastSeenAt,
      createdAt: item.createdAt,
    })
  }

  return items
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
    (value.autoRebuildEnable !== undefined &&
      value.autoRebuildEnable !== null &&
      !isBoolean(value.autoRebuildEnable)) ||
    (value.autoRebuildCron !== undefined &&
      value.autoRebuildCron !== null &&
      !isString(value.autoRebuildCron)) ||
    (value.lastRebuildTime !== undefined &&
      value.lastRebuildTime !== null &&
      !isString(value.lastRebuildTime))
  ) {
    return null
  }

  return {
    ...value,
    autoRebuildEnable: value.autoRebuildEnable ?? false,
    autoRebuildCron: value.autoRebuildCron ?? '0 2 * * *',
    lastRebuildTime: value.lastRebuildTime ?? '',
  } as unknown as Models.MediaConfig
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
