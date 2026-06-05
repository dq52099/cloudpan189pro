const HTTP_BASE_URL_RE = /^https?:\/\/[^/?#\s]+(?:[/?#][^\s]*)?$/i

const hasControlChar = (value: string) => {
  return Array.from(value).some((char) => {
    const code = char.charCodeAt(0)

    return code <= 0x1f || code === 0x7f
  })
}

export const normalizeHttpBaseURL = (value: string | null | undefined) => {
  const trimmed = (value || '').trim()
  if (!trimmed || hasControlChar(trimmed) || !HTTP_BASE_URL_RE.test(trimmed)) {
    return ''
  }

  try {
    const parsedURL = new URL(trimmed)
    if ((parsedURL.protocol !== 'http:' && parsedURL.protocol !== 'https:') || !parsedURL.host) {
      return ''
    }
  } catch {
    return ''
  }

  return trimmed
}
