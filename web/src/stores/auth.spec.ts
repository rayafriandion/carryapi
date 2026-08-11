import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from './auth'

vi.mock('../api/http', () => ({
  http: {
    post: vi.fn(),
    get: vi.fn(),
  },
}))

import { http } from '../api/http'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  it('login without 2fa sets user', async () => {
    (http.post as any).mockResolvedValue({ data: { user: { id: 1, email: 'a@x.com', role: 'admin' } } })
    const s = useAuthStore()
    const r = await s.login('a@x.com', 'pw')
    expect(r.requires2fa).toBe(false)
    expect(s.user?.email).toBe('a@x.com')
    expect(s.isAdmin).toBe(true)
  })
  it('login requiring 2fa does not set user', async () => {
    (http.post as any).mockResolvedValue({ data: { requires_2fa: true } })
    const s = useAuthStore()
    const r = await s.login('a@x.com', 'pw')
    expect(r.requires2fa).toBe(true)
    expect(s.user).toBeNull()
  })
  it('logout clears user', async () => {
    (http.post as any).mockResolvedValue({})
    const s = useAuthStore()
    s.setUser({ id: 1, email: 'a@x.com', role: 'user', status: 'active' })
    await s.logout()
    expect(s.user).toBeNull()
  })
})
