import axios from 'axios'
import { getCSRFToken } from '../utils/csrf'

export const http = axios.create({ baseURL: '' })

http.interceptors.request.use((config) => {
  const method = (config.method || 'get').toLowerCase()
  if (method !== 'get' && method !== 'head' && method !== 'options') {
    const token = getCSRFToken()
    if (token) config.headers['X-CSRF-Token'] = token
  }
  return config
})

http.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      // 登录失效 -> 跳登录(通过事件或 window.location)
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

// 统一错误消息提取
export function errorMessage(err: unknown): string {
  const data = (err as any)?.response?.data
  return data?.error || data?.message || '请求失败'
}
