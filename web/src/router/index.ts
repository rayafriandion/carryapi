import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    { path: '/', component: () => import('../components/AppLayout.vue'), children: [
      { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
      { path: 'stats', name: 'stats', component: () => import('../views/StatsView.vue') },
      { path: 'logs', name: 'logs', component: () => import('../views/LogsView.vue') },
      { path: 'keys', name: 'keys', component: () => import('../views/KeysView.vue') },
      { path: 'account', name: 'account', component: () => import('../views/AccountView.vue') },
      // admin-only
      { path: 'models', name: 'models', component: () => import('../views/ModelsView.vue'), meta: { admin: true } },
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
  if (to.name === 'login') return true
  if (!auth.isLoggedIn) return { name: 'login' }
  if (to.meta.admin && !auth.isAdmin) return { name: 'dashboard' }
  return true
})

export default router
