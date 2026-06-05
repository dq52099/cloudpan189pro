/**
 * 格式化工具函数
 */

import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

dayjs.extend(relativeTime)

const parseDateValue = (dateString: string | null | undefined) => {
  const value = dateString?.trim()
  if (!value) {
    return null
  }

  const date = dayjs(value)

  return date.isValid() ? date : null
}

/**
 * 格式化文件大小
 * @param bytes 字节数
 * @returns 格式化后的文件大小字符串
 */
export const formatFileSize = (bytes: number): string => {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

/**
 * 格式化日期时间
 * @param dateString 日期字符串
 * @returns 格式化后的日期时间字符串
 */
export const formatDate = (dateString: string | null | undefined): string => {
  const date = parseDateValue(dateString)

  return date ? date.format('YYYY-MM-DD HH:mm:ss') : '-'
}

/**
 * 格式化相对时间
 * @param dateString 日期字符串
 * @returns 相对时间字符串
 */
export const formatRelativeTime = (dateString: string | null | undefined): string => {
  const date = parseDateValue(dateString)

  return date ? date.fromNow() : '-'
}
