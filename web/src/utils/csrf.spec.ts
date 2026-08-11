import { describe, it, expect, beforeEach } from 'vitest'
import { getCSRFToken } from './csrf'

describe('getCSRFToken', () => {
  beforeEach(() => {
    // jsdom 里 document.cookie 可读写
    document.cookie = 'carryapi_csrf=abc123; path=/'
  })
  it('reads the csrf token from cookie', () => {
    expect(getCSRFToken()).toBe('abc123')
  })
  it('returns empty string when cookie absent', () => {
    document.cookie = 'carryapi_csrf=; expires=Thu, 01 Jan 1970 00:00:00 GMT'
    expect(getCSRFToken()).toBe('')
  })
})
