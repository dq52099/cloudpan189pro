const CLOUD189_SUBSCRIBE_USER_LINK_RE =
  /(?:^|[^a-z0-9_+./:@-])(?:(?:https?:\/\/|\/\/)?)content\.21cn\.com(?::\d+)?(?:\/[^\s"'<>]*)?[?&]uuid=([^&#\s"'<>]+)/i
const CLOUD189_SUBSCRIBE_USER_LABEL_RE =
  /(?:订阅号(?:ID)?|订阅用户(?:ID)?|\bsubscribe[\s_-]?user(?:id)?\b|\bup[\s_-]?user(?:id)?\b|\buuid\b)[:：=]\s*([^\s,，;；。()（）]+)/i
const CLOUD189_SUBSCRIBE_USER_BOUNDARY_TRIM_RE =
  /^[\s.,，。;；:：!?！？、()[\]【】<>《》「」『』“”‘’"']+|[\s.,，。;；:：!?！？、()[\]【】<>《》「」『』“”‘’"']+$/g
const CLOUD189_URL_LIKE_RE =
  /(?:[a-z][a-z0-9+.-]*:\/\/|\/\/)[^\s"'<>]+|(?:^|[^a-z0-9_+./:@-])(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?::\d+)?(?:\/[^\s"'<>]*|\?[^\s"'<>]*)/i

export const parseCloud189SubscribeUserId = (input: string) => {
  let match = input.match(CLOUD189_SUBSCRIBE_USER_LINK_RE)
  if (match?.[1]) {
    return normalizeSubscribeUserCandidate(match[1])
  }

  if (CLOUD189_URL_LIKE_RE.test(input)) {
    return ''
  }

  match = input.match(CLOUD189_SUBSCRIBE_USER_LABEL_RE)
  if (match?.[1]) {
    return normalizeSubscribeUserCandidate(match[1])
  }

  return ''
}

export const normalizeCloud189SubscribeUserInput = (input: string) => {
  const parsed = parseCloud189SubscribeUserId(input)
  if (parsed) {
    return parsed
  }

  input = trimSubscribeUserBoundary(input)
  if (CLOUD189_URL_LIKE_RE.test(input)) {
    return ''
  }

  return input
}

const normalizeSubscribeUserCandidate = (input: string) => {
  let subscribeUserId = trimSubscribeUserBoundary(input)
  if (!subscribeUserId) {
    return ''
  }

  try {
    subscribeUserId = trimSubscribeUserBoundary(
      decodeURIComponent(subscribeUserId.replace(/\+/g, ' '))
    )
  } catch {
    return subscribeUserId.trim()
  }

  return subscribeUserId.trim()
}

const trimSubscribeUserBoundary = (value: string) => {
  return value.trim().replace(CLOUD189_SUBSCRIBE_USER_BOUNDARY_TRIM_RE, '')
}
