const CLOUD189_SHARE_LINK_RE =
  /(?:^|[^a-z0-9_+./:@-])(?:(?:https?:\/\/|\/\/)?)(?:www\.)?cloud\.189\.cn(?::\d+)?\/t\/([^\s/?#()（）,，;；。:：]+)/i
const CLOUD189_SHARE_CODE_RE =
  /(?:分享码|\bshare[\s_-]?code\b|\bshare\b)[:：]\s*([^\s,，;；。()（）]+)/i
const CLOUD189_ACCESS_CODE_RE =
  /(访问码|提取码|\baccess[\s_-]?code\b|\baccess\b|\bcode\b)[:：]\s*([^\s,，;；。()（）]+)/gi
const CLOUD189_LABEL_PREFIX_RE =
  /^(?:访问码|提取码|分享码|\baccess[\s_-]?code\b|\baccess\b|\bcode\b|\bshare[\s_-]?code\b|\bshare\b)[:：]?/i
const CLOUD189_BOUNDARY_TRIM_RE =
  /^[\s.,，。;；:：!?！？、()[\]【】<>《》「」『』“”‘’"']+|[\s.,，。;；:：!?！？、()[\]【】<>《》「」『』“”‘’"']+$/g
const CLOUD189_EMBEDDED_LABEL_SEPARATOR_RE = /[、!！?？\]】》〉」』”’]/gu
const CLOUD189_RAW_CODE_RE = /^[a-z0-9]+$/i
const CLOUD189_URL_LIKE_RE =
  /(?:[a-z][a-z0-9+.-]*:\/\/|\/\/)[^\s"'<>]+|(?:^|[^a-z0-9_+./:@-])(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?::\d+)?(?:\/[^\s"'<>]*|\?[^\s"'<>]*)/i

export interface Cloud189ShareParams {
  shareCode: string
  accessCode: string
}

export const isCloud189ShareCode = (value: string) => {
  return CLOUD189_RAW_CODE_RE.test(value.trim())
}

export const isCloud189AccessCode = (value: string) => {
  return CLOUD189_RAW_CODE_RE.test(value.trim())
}

export const containsCloud189URLLike = (value: string) => {
  return CLOUD189_URL_LIKE_RE.test(value.trim())
}

export const parseCloud189ShareCode = (
  input: string,
  explicitAccessCode = ''
): Cloud189ShareParams => {
  const cleanCode = normalizeCloud189ShareText(input)
  const normalizedExplicitAccessCode = normalizeExplicitCloud189AccessCode(explicitAccessCode)

  return {
    shareCode: extractCloud189ShareCode(cleanCode),
    accessCode: normalizedExplicitAccessCode || extractCloud189AccessCode(cleanCode),
  }
}

const normalizeExplicitCloud189AccessCode = (input: string) => {
  const cleanCode = normalizeCloud189ShareText(input)
  if (!cleanCode) {
    return ''
  }

  const accessCode = extractCloud189AccessCode(cleanCode)
  if (accessCode) {
    return accessCode
  }

  return cleanCloud189ExtractedCodeCandidate(cleanCode)
}

const normalizeCloud189ShareText = (input: string) => {
  return input.replace(/（/g, '(').replace(/）/g, ')').replace(/：/g, ':').trim()
}

const extractCloud189AccessCode = (cleanCode: string) => {
  CLOUD189_ACCESS_CODE_RE.lastIndex = 0

  for (const match of cleanCode.matchAll(CLOUD189_ACCESS_CODE_RE)) {
    const label = match[1]?.toLowerCase() || ''
    const index = match.index ?? 0
    if (label === 'code' && hasShareLabelBeforeCode(cleanCode.slice(0, index))) {
      continue
    }

    return cleanCloud189ExtractedCodeCandidate(match[2] || '')
  }

  const parts = cleanCode.split(/\s+/).filter(Boolean)
  if (parts.length <= 1) {
    return ''
  }

  const lastPart = cleanCloud189ExtractedCodeCandidate(parts[parts.length - 1])
  if (lastPart.length === 4) {
    return lastPart
  }

  return ''
}

const hasShareLabelBeforeCode = (prefix: string) => {
  return prefix
    .trim()
    .toLowerCase()
    .replace(/[-_\s]+$/g, '')
    .endsWith('share')
}

const cleanCloud189ExtractedCodeCandidate = (value: string) => {
  const trimmedValue = value.trim()

  return trimCloud189CodeBoundary(cutCloud189EmbeddedLabel(trimmedValue))
}

const cutCloud189EmbeddedLabel = (value: string) => {
  for (const match of value.matchAll(CLOUD189_EMBEDDED_LABEL_SEPARATOR_RE)) {
    const index = match.index ?? -1
    if (index < 1) {
      continue
    }

    const prefix = value.slice(0, index).trim()
    const suffix = trimCloud189CodeBoundary(value.slice(index + match[0].length))
    if (prefix && CLOUD189_LABEL_PREFIX_RE.test(suffix)) {
      return prefix
    }
  }

  return value
}

const trimCloud189CodeBoundary = (value: string) => {
  return value.replace(CLOUD189_BOUNDARY_TRIM_RE, '')
}

const extractCloud189ShareCode = (cleanCode: string) => {
  const linkMatch = cleanCode.match(CLOUD189_SHARE_LINK_RE)
  if (linkMatch?.[1]) {
    return cleanCloud189ExtractedCodeCandidate(linkMatch[1])
  }

  const labelMatch = cleanCode.match(CLOUD189_SHARE_CODE_RE)
  if (labelMatch?.[1]) {
    return cleanCloud189ExtractedCodeCandidate(labelMatch[1])
  }

  const bracketIndex = cleanCode.indexOf('(')
  if (bracketIndex > -1) {
    return cleanCode.slice(0, bracketIndex).trim()
  }

  if (CLOUD189_URL_LIKE_RE.test(cleanCode)) {
    return ''
  }

  const parts = cleanCode.split(/\s+/).filter(Boolean)
  if (parts.length > 0) {
    return cleanCloud189ExtractedCodeCandidate(parts[0])
  }

  return cleanCode
}
