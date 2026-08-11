<template>
  <n-layout position="absolute" class="app-layout">
    <n-layout-header bordered class="app-header">
      <div class="header-inner">
        <div class="app-name">carryAPI 控制台</div>
        <div class="header-right">
          <span class="user-email">{{ auth.user?.email }}</span>
          <n-button size="small" @click="onLogout">退出登录</n-button>
        </div>
      </div>
    </n-layout-header>
    <n-layout has-sider position="absolute" class="app-body">
      <n-layout-sider bordered collapse-mode="width" :collapsed-width="64" :width="200" :collapsed="collapsed" show-trigger @collapse="collapsed = true" @expand="collapsed = false">
        <n-menu :value="activeKey" :options="menuOptions" @update:value="onMenuSelect" />
      </n-layout-sider>
      <n-layout content-style="padding: 16px; overflow: auto;">
        <router-view />
      </n-layout>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NLayout, NLayoutHeader, NLayoutSider, NMenu, NButton } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const collapsed = ref(false)

const baseMenus: MenuOption[] = [
  { label: 'Dashboard', key: 'dashboard' },
  { label: '统计', key: 'stats' },
  { label: '日志', key: 'logs' },
  { label: 'API Key', key: 'keys' },
  { label: '账号', key: 'account' },
]

const adminMenus: MenuOption[] = [
  { label: '模型管理', key: 'models' },
  { label: '配额管理', key: 'quotas' },
  { label: '用户管理', key: 'users' },
  { label: '系统设置', key: 'settings' },
]

const menuOptions = computed<MenuOption[]>(() => auth.isAdmin ? [...baseMenus, ...adminMenus] : baseMenus)

const activeKey = computed(() => (route.name as string) || 'dashboard')

function onMenuSelect(key: string) {
  router.push({ name: key })
}

async function onLogout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.app-layout {
  height: 100vh;
}
.app-header {
  padding: 0 16px;
  height: 56px;
  display: flex;
  align-items: center;
}
.header-inner {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
}
.app-name {
  font-size: 18px;
  font-weight: 600;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.user-email {
  color: #666;
}
.app-body {
  top: 56px;
}
</style>
