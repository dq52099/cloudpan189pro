export const isValidListTotal = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

export const getListItems = <T>(value: unknown): T[] | null => {
  if (!value || typeof value !== 'object') {
    return null
  }

  const items = (value as { data?: unknown }).data

  return Array.isArray(items) ? (items as T[]) : null
}

export const getListTotal = (value: unknown): number | null => {
  if (!value || typeof value !== 'object') {
    return null
  }

  const total = (value as { total?: unknown }).total

  return isValidListTotal(total) ? total : null
}
