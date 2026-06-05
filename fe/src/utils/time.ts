/**
 * 时间相关工具函数
 */

import dayjs from 'dayjs'

export interface DateRangeQuery {
  beginAt: string
  endAt: string
}

const parseDateTimeValue = (value: number | string | null | undefined) => {
  if (value === null || value === undefined) {
    return null
  }

  if (typeof value === 'number' && !Number.isFinite(value)) {
    return null
  }

  if (typeof value === 'string' && !value.trim()) {
    return null
  }

  const date = dayjs(value)

  return date.isValid() ? date : null
}

/**
 * 格式化剩余时间
 * @param expiresIn 剩余有效期（秒），兼容历史毫秒时间戳
 * @returns 格式化后的剩余时间字符串
 */
export const formatRemainingTime = (expiresIn: number | null | undefined): string => {
  if (expiresIn === null || expiresIn === undefined) {
    return '永久有效'
  }

  if (!Number.isFinite(expiresIn) || expiresIn <= 0) {
    return '已过期'
  }

  const remainingSeconds =
    expiresIn > 10_000_000_000 ? Math.floor((expiresIn - Date.now()) / 1000) : Math.floor(expiresIn)
  if (remainingSeconds <= 0) {
    return '已过期'
  }

  const days = Math.floor(remainingSeconds / (60 * 60 * 24))
  const hours = Math.floor((remainingSeconds % (60 * 60 * 24)) / (60 * 60))
  const minutes = Math.floor((remainingSeconds % (60 * 60)) / 60)
  const seconds = remainingSeconds % 60

  let result = ''
  if (days > 0) result += `${days}天`
  if (hours > 0) result += `${hours}时`
  if (minutes > 0) result += `${minutes}分`
  if (seconds > 0) result += `${seconds}秒`

  return result || '即将过期'
}

/**
 * 格式化时间戳为本地时间字符串
 * @param timestamp 时间戳（毫秒）
 * @returns 格式化后的时间字符串
 */
export const formatDateTime = (timestamp: number | string | null | undefined): string => {
  const date = parseDateTimeValue(timestamp)

  return date ? date.format('YYYY-MM-DD HH:mm:ss') : '-'
}

export const formatDateRangeQuery = (
  range: readonly [number, number] | null | undefined
): DateRangeQuery | null => {
  if (!range || range.length !== 2) {
    return null
  }

  const beginAt = parseDateTimeValue(range[0])
  const endAt = parseDateTimeValue(range[1])
  if (!beginAt || !endAt) {
    return null
  }

  return {
    beginAt: beginAt.toISOString(),
    endAt: endAt.toISOString(),
  }
}
