const COOKIE_NAME = 'carryapi_csrf'

export function getCSRFToken(): string {
  if (typeof document === 'undefined') return ''
  const match = document.cookie.match(new RegExp('(?:^|; )' + COOKIE_NAME + '=([^;]*)'))
  return match ? decodeURIComponent(match[1]) : ''
}
