<template>
  <div class="account">
    <n-card title="基本信息" class="section">
      <n-descriptions label-placement="left" bordered :column="1">
        <n-descriptions-item label="用户 ID">{{ auth.user?.id }}</n-descriptions-item>
        <n-descriptions-item label="邮箱">{{ auth.user?.email }}</n-descriptions-item>
        <n-descriptions-item label="角色">{{ roleLabel }}</n-descriptions-item>
        <n-descriptions-item label="状态">{{ statusLabel }}</n-descriptions-item>
        <n-descriptions-item label="认证方式">{{ authMethodsText || '-' }}</n-descriptions-item>
      </n-descriptions>
    </n-card>

    <n-card title="两步验证 (TOTP)" class="section">
      <template v-if="hasTotp">
        <p>当前已开启两步验证。关闭需要输入当前密码。</p>
        <n-input v-model:value="disablePassword" type="password" show-password-on="click" placeholder="当前密码" class="inline-input" />
        <n-button type="error" :loading="disabling" :disabled="!disablePassword" @click="onDisableTOTP">
          关闭 TOTP
        </n-button>
      </template>
      <template v-else>
        <p>当前未开启两步验证。开启后将展示密钥与备份码,请妥善保存。</p>
        <n-button type="primary" :loading="settingUp" @click="onSetupTOTP">开启 TOTP</n-button>
      </template>
    </n-card>

    <!-- TOTP 启用展示 -->
    <n-modal v-model:show="setupShow" preset="card" title="开启两步验证" style="width: 520px">
      <n-alert type="warning" :show-icon="true">请立即保存以下密钥与备份码,关闭后无法再次查看。</n-alert>
      <div class="totp-box">
        <p>1. 在 Authenticator 应用中添加:</p>
        <n-input :value="setupResult?.otpauth_url" readonly />
        <p class="mt">2. 或手动输入密钥:</p>
        <n-input :value="setupResult?.secret" readonly />
        <p class="mt">3. 备份码(每个只能使用一次):</p>
        <div class="backup-codes">
          <n-tag v-for="c in setupResult?.backup_codes" :key="c" type="info" class="code-tag">{{ c }}</n-tag>
        </div>
      </div>
      <template #footer>
        <div class="modal-footer">
          <n-button type="primary" @click="copyTotp">复制</n-button>
          <n-button @click="finishSetup">我已保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NCard, NDescriptions, NDescriptionsItem, NButton, NInput, NModal, NAlert, NTag, useMessage } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { errorMessage } from '../api/http'

const message = useMessage()
const auth = useAuthStore()

const roleLabel = computed(() => (auth.user?.role === 'admin' ? '管理员' : '用户'))
const statusLabel = computed(() => (auth.user?.status === 'active' ? '正常' : '禁用'))
const authMethodsText = computed(() => auth.authMethods?.join(', ') || '')
const hasTotp = computed(() => auth.authMethods?.includes('totp'))

// disable
const disablePassword = ref('')
const disabling = ref(false)

async function onDisableTOTP() {
  if (!disablePassword.value) return
  disabling.value = true
  try {
    await auth.disableTOTP(disablePassword.value)
    disablePassword.value = ''
    message.success('已关闭两步验证')
    await auth.fetchMe()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    disabling.value = false
  }
}

// setup
const settingUp = ref(false)
const setupShow = ref(false)
const setupResult = ref<any>(null)

async function onSetupTOTP() {
  settingUp.value = true
  try {
    setupResult.value = await auth.setupTOTP()
    setupShow.value = true
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    settingUp.value = false
  }
}

function copyTotp() {
  if (!setupResult.value) return
  const text = [
    `OTPAuth: ${setupResult.value.otpauth_url || ''}`,
    `Secret: ${setupResult.value.secret || ''}`,
    `Backup codes: ${(setupResult.value.backup_codes || []).join(', ')}`,
  ].join('\n')
  navigator.clipboard.writeText(text)
  message.success('已复制')
}

async function finishSetup() {
  setupShow.value = false
  setupResult.value = null
  await auth.fetchMe()
}

onMounted(() => {
  // 确保 auth_methods 已初始化
  if (!auth.initialized) auth.fetchMe()
})
</script>

<style scoped>
.section {
  margin-bottom: 16px;
}
.inline-input {
  width: 280px;
  margin-bottom: 12px;
  display: block;
}
.totp-box {
  margin-top: 12px;
}
.mt {
  margin-top: 12px;
}
.backup-codes {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}
.code-tag {
  font-family: monospace;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
