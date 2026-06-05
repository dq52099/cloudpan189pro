export const LOGIN_PATH = '/@login'
export const DEFAULT_AUTH_REDIRECT_PATH = '/@dashboard'

const isLoginPath = (path: string) => {
  return (
    path === LOGIN_PATH || path.startsWith(`${LOGIN_PATH}?`) || path.startsWith(`${LOGIN_PATH}#`)
  )
}

export const isSafeInternalRedirectPath = (value: unknown): value is string => {
  if (typeof value !== 'string') {
    return false
  }

  const path = value.trim()
  if (!path || /[\r\n]/.test(path)) {
    return false
  }

  return path.startsWith('/') && !path.startsWith('//') && !path.startsWith('/\\')
}

export const getSafePostLoginRedirectPath = (
  value: unknown,
  fallback = DEFAULT_AUTH_REDIRECT_PATH
) => {
  if (!isSafeInternalRedirectPath(value)) {
    return fallback
  }

  const path = value.trim()
  if (isLoginPath(path)) {
    return fallback
  }

  return path
}

export const buildLoginRedirectPath = (redirectPath: unknown) => {
  const targetPath = getSafePostLoginRedirectPath(redirectPath, '')
  if (!targetPath) {
    return LOGIN_PATH
  }

  return `${LOGIN_PATH}?redirect=${encodeURIComponent(targetPath)}`
}
