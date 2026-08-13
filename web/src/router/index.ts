import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { http } from '../api/http'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/setup', name: 'setup', component: () => import('../views/SetupView.vue') },
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    { path: '/', component: () => import('../components/AppLayout.vue'), children: [
      { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
      { path: 'stats', name: 'stats', component: () => import('../views/StatsView.vue') },
      { path: 'logs', name: 'logs', component: () => import('../views/LogsView.vue') },
      { path: 'keys', name: 'keys', component: () => import('../views/KeysView.vue') },
      { path: 'models-catalog', name: 'models-catalog', component: () => import('../views/ModelListView.vue') },
      { path: 'account', name: 'account', component: () => import('../views/AccountView.vue') },
      // admin-only
      { path: 'models', name: 'models', component: () => import('../views/ModelsView.vue'), meta: { admin: true } },
      { path: 'routing', name: 'routing', component: () => import('../views/RoutingView.vue'), meta: { requiresAuth: true, admin: true } },
      { path: 'models/new', name: 'model-new', component: () => import('../views/ModelEditView.vue'), meta: { admin: true } },
      { path: 'models/:id/edit', name: 'model-edit', component: () => import('../views/ModelEditView.vue'), meta: { admin: true } },
      { path: 'quotas', name: 'quotas', component: () => import('../views/QuotasView.vue'), meta: { admin: true } },
      { path: 'users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { admin: true } },
      { path: 'settings', name: 'settings', component: () => import('../views/SettingsView.vue'), meta: { admin: true } },
    ]},
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.initialized) {
    try { await auth.fetchMe() } catch { /* 未登录 */ }
  }
  // 未登录时先检查是否需要进行首次设置
  if (!auth.isLoggedIn) {
    let needsSetup = false
    try {
      const res = await http.get('/api/setup/status')
      needsSetup = !!res.data?.needs_setup
    } catch { /* 忽略 */ }
    if (needsSetup) {
      if (to.name !== 'setup') return { name: 'setup' }
      return true
    }
    if (to.name === 'setup') return { name: 'login' }
    if (to.name === 'login') return true
    return { name: 'login' }
  }
  // 已登录
  if (to.name === 'login' || to.name === 'setup') return { name: 'dashboard' }
  if (to.meta.admin && !auth.isAdmin) return { name: 'dashboard' }
  return true
})

export default router
