<template>
  <div class="settings">
    <n-card title="广播开关" class="section">
      <div class="setting-row">
        <div class="setting-info">
          <span class="label">监听地址</span>
          <span class="desc">监听地址修改需在服务器配置中设置,重启后生效。</span>
        </div>
        <n-switch :value="broadcastOn" disabled />
      </div>
      <div class="listen-value">当前监听地址: <n-tag>{{ listenHost || '-' }}</n-tag></div>
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
import { onMounted, reactive, ref } from 'vue'
import { NCard, NSwitch, NInputNumber, NButton, NTag, useMessage } from 'naive-ui'
import { http, errorMessage } from '../api/http'

const message = useMessage()
const saving = ref(false)

const listenHost = ref('')
const broadcastOn = ref(false)
const form = reactive({ registration_open: false, force_2fa: false, log_retention_days: 30 })

async function load() {
  try {
    const res = await http.get('/api/settings')
    const s = res.data || {}
    listenHost.value = s.listen_host || ''
    broadcastOn.value = listenHost.value === '0.0.0.0'
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
    await http.put('/api/settings', {
      registration_open: String(form.registration_open),
      force_2fa: String(form.force_2fa),
      log_retention_days: String(form.log_retention_days),
    })
    message.success('已保存')
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
