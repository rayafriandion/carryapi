import { defineStore } from 'pinia'
import { http } from '../api/http'

export interface UserInfo {
  id: number
  email: string
  role: string
  status: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as UserInfo | null,
    authMethods: [] as string[],
    initialized: false,
  }),
  getters: {
    isAdmin: (s) => s.user?.role === 'admin',
    isLoggedIn: (s) => !!s.user,
  },
  actions: {
    async login(email: string, password: string) {
      const res = await http.post('/api/auth/login', { email, password })
      if (res.data.requires_2fa) {
        return { requires2fa: true }
      }
      this.setUser(res.data.user)
      return { requires2fa: false }
    },
    async complete2FA(email: string, code: string) {
      const res = await http.post('/api/auth/2fa/complete', { email, code })
      this.setUser(res.data.user)
    },
    async register(email: string, password: string) {
      await http.post('/api/auth/register', { email, password })
    },
    async logout() {
      try { await http.post('/api/auth/logout') } catch { /* ignore */ }
      this.user = null
      this.authMethods = []
    },
    async fetchMe() {
      const res = await http.get('/api/auth/me')
      this.setUser(res.data.user)
      this.authMethods = res.data.auth_methods || []
      this.initialized = true
    },
    setUser(u: UserInfo) {
      this.user = u
      this.initialized = true
    },
    async setupTOTP() {
      const res = await http.post('/api/auth/2fa/setup')
      return res.data // {secret, otpauth_url, backup_codes}
    },
    async disableTOTP(password: string) {
      await http.post('/api/auth/2fa/disable', { password })
    },
  },
})
