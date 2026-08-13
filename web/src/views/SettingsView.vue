<template>
  <div class="settings">
    <n-card title="广播开关" class="section">
      <div class="setting-row">
        <div class="setting-info">
          <span class="label">允许其他设备访问</span>
          <span class="desc">{{ listenDescription }}</span>
        </div>
        <n-switch v-model:value="broadcastOn" :disabled="listenLocked || saving" />
      </div>
      <div class="listen-value">当前监听模式: <n-tag>{{ listenModeLabel }}</n-tag></div>
      <div v-if="listenLocked" class="listen-value">监听地址由{{ listenSourceLabel }}控制，后台无法修改。</div>
      <div v-else class="listen-value">修改后需要重启 carryAPI 才会生效。</div>
    </n-card>

    <n-card title="一般设置" class="section">
      <div class="setting-row">
        <div class="setting-info">
          <span class="label">开放注册</span>
          <span class="desc">是否允许新用户注册</span>
        </div>
        <n-switch v-model:value="form.registration_open" />
      </div>

      <div class="setting-row">
        <div class="setting-info">
          <span class="label">强制两步验证</span>
          <span class="desc">要求所有账户开启 TOTP</span>
        </div>
        <n-switch v-model:value="form.force_2fa" />
      </div>

      <div class="setting-row">
        <div class="setting-info">
          <span class="label">日志保留天数</span>
          <span class="desc">请求日志保留的天数</span>
        </div>
        <n-input-number v-model:value="form.log_retention_days" :min="1" style="width: 160px" />
      </div>

      <div class="actions">
        <n-button type="primary" :loading="saving" @click="onSave">保存设置</n-button>
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { NCard, NSwitch, NInputNumber, NButton, NTag, useMessage } from 'naive-ui'
import { http, errorMessage } from '../api/http'

const message = useMessage()
const saving = ref(false)

const listenHost = ref('all')
const listenLocked = ref(false)
const listenSource = ref('default')
const form = reactive({ registration_open: false, force_2fa: false, log_retention_days: 30 })

const broadcastOn = computed({
  get: () => listenHost.value !== '127.0.0.1' && listenHost.value !== '::1',
  set: (value: boolean) => {
    listenHost.value = value ? 'all' : '127.0.0.1'
  },
})

const listenModeLabel = computed(() => {
  switch (listenHost.value) {
    case 'all':
      return '双栈全部接口 ([::])'
    case '0.0.0.0':
      return '仅 IPv4 全部接口'
    case '::':
      return 'IPv6 双栈全部接口 ([::])'
    case '127.0.0.1':
      return '仅本机 IPv4'
    case '::1':
      return '仅本机 IPv6'
    default:
      return listenHost.value || '-'
  }
})

const listenSourceLabel = computed(() => {
  switch (listenSource.value) {
    case 'flag':
      return '启动参数 --host'
    case 'env':
      return '环境变量 CARRYAPI_HOST'
    case 'database':
      return '系统设置'
    default:
      return '默认配置'
  }
})

const listenDescription = computed(() => {
  if (broadcastOn.value) {
    return '广播开：本机、局域网和公网均可访问，需放行防火墙和路由器端口。'
  }
  return '广播关：仅运行 carryAPI 的这台机器可以访问。'
})

async function load() {
  try {
    const res = await http.get('/api/settings')
    const s = res.data || {}
    listenHost.value = s.listen_host || 'all'
    listenLocked.value = s.listen_host_locked === 'true'
    listenSource.value = s.listen_host_source || 'default'
    form.registration_open = s.registration_open === 'true'
    form.force_2fa = s.force_2fa === 'true'
    const d = parseInt(s.log_retention_days, 10)
    form.log_retention_days = Number.isNaN(d) ? 30 : d
  } catch (e) {
    message.error(errorMessage(e))
  }
}

async function onSave() {
  saving.value = true
  try {
    const res = await http.put('/api/settings', {
      listen_host: listenHost.value,
      registration_open: String(form.registration_open),
      force_2fa: String(form.force_2fa),
      log_retention_days: String(form.log_retention_days),
    })
    if (res.data?.restart_required) {
      message.success('已保存，监听地址需重启 carryAPI 后生效')
    } else {
      message.success('已保存')
    }
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.section {
  margin-bottom: 16px;
}
.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  gap: 16px;
}
.setting-info {
  display: flex;
  flex-direction: column;
}
.label {
  font-weight: 600;
}
.desc {
  color: #999;
  font-size: 12px;
}
.listen-value {
  margin-top: 12px;
  color: #666;
}
.actions {
  margin-top: 16px;
}
</style>
