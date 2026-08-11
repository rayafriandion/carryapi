<template>
  <div class="login-page">
    <n-card class="login-card" :bordered="false">
      <div class="login-title">carryAPI 控制台</div>
      <n-tabs v-model:value="mode" type="line" animated>
        <!-- 登录 -->
        <n-tab-pane name="login" tab="登录">
          <n-form ref="loginFormRef" :model="loginForm" :rules="loginRules">
            <n-form-item label="邮箱" path="email">
              <n-input v-model:value="loginForm.email" placeholder="you@example.com" />
            </n-form-item>
            <template v-if="!requires2fa">
              <n-form-item label="密码" path="password">
                <n-input v-model:value="loginForm.password" type="password" show-password-on="click" placeholder="密码" @keydown.enter="onLogin" />
              </n-form-item>
            </template>
            <template v-else>
              <n-form-item label="TOTP 验证码" path="code">
                <n-input v-model:value="loginForm.code" placeholder="6 位验证码" @keydown.enter="onComplete2FA" />
              </n-form-item>
            </template>
            <n-button type="primary" block :loading="loading" @click="requires2fa ? onComplete2FA() : onLogin()">
              {{ requires2fa ? '验证' : '登录' }}
            </n-button>
          </n-form>

          <!-- OAuth -->
          <div class="oauth-row">
            <a href="/api/auth/oauth/discord">
              <n-button type="info" tertiary>Discord 登录</n-button>
            </a>
            <a href="/api/auth/oauth/x">
              <n-button type="default" tertiary>X 登录</n-button>
            </a>
          </div>
          <n-divider title-placement="left">或</n-divider>

          <!-- Passkey -->
          <n-form class="passkey-form">
            <n-form-item label="Passkey 邮箱">
              <n-input v-model:value="passkeyEmail" placeholder="you@example.com" @keydown.enter="onPasskey" />
            </n-form-item>
            <n-button block :loading="passkeyLoading" @click="onPasskey">使用 Passkey 登录</n-button>
          </n-form>
        </n-tab-pane>

        <!-- 注册 -->
        <n-tab-pane name="register" tab="注册">
          <n-form ref="registerFormRef" :model="registerForm" :rules="registerRules">
            <n-form-item label="邮箱" path="email">
              <n-input v-model:value="registerForm.email" placeholder="you@example.com" />
            </n-form-item>
            <n-form-item label="密码" path="password">
              <n-input v-model:value="registerForm.password" type="password" show-password-on="click" placeholder="至少 8 位" />
            </n-form-item>
            <n-button type="primary" block :loading="registering" @click="onRegister">创建账户</n-button>
          </n-form>
        </n-tab-pane>
      </n-tabs>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NInput, NButton, NForm, NFormItem, NTabPane, NTabs, NCard, NDivider } from 'naive-ui'
import { http, errorMessage } from '../api/http'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const mode = ref<'login' | 'register'>('login')
const loading = ref(false)
const registering = ref(false)
const passkeyLoading = ref(false)
const requires2fa = ref(false)

const loginForm = ref({ email: '', password: '', code: '' })
const registerForm = ref({ email: '', password: '' })
const passkeyEmail = ref('')
const loginFormRef = ref()
const registerFormRef = ref()

const loginRules = {
  email: { required: true, message: '请输入邮箱', trigger: 'blur' },
  password: { required: true, message: '请输入密码', trigger: 'blur' },
}
const registerRules = {
  email: { required: true, message: '请输入邮箱', trigger: 'blur' },
  password: { required: true, message: '请输入密码', trigger: 'blur' },
}

async function onLogin() {
  if (requires2fa.value) return onComplete2FA()
  loading.value = true
  try {
    const res = await auth.login(loginForm.value.email, loginForm.value.password)
    if (res.requires2fa) {
      requires2fa.value = true
      message.info('该账户已开启两步验证，请输入 TOTP 验证码')
    } else {
      message.success('登录成功')
      router.push('/')
    }
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}

async function onComplete2FA() {
  loading.value = true
  try {
    await auth.complete2FA(loginForm.value.email, loginForm.value.code)
    message.success('登录成功')
    router.push('/')
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}

async function onRegister() {
  registering.value = true
  try {
    await auth.register(registerForm.value.email, registerForm.value.password)
    message.success('账户创建成功，请登录')
    // 回填登录表单并切换到登录
    loginForm.value.email = registerForm.value.email
    loginForm.value.password = registerForm.value.password
    requires2fa.value = false
    mode.value = 'login'
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    registering.value = false
  }
}

async function onPasskey() {
  if (!passkeyEmail.value) {
    message.warning('请输入邮箱')
    return
  }
  passkeyLoading.value = true
  try {
    const begin = await http.post('/api/auth/passkey/login/begin', {}, {
      params: { email: passkeyEmail.value },
    })
    const { publicKey, session_key } = begin.data
    const assertion = await navigator.credentials.get({ publicKey })
    const finish = await http.post('/api/auth/passkey/login/finish', assertion as any, {
      params: { email: passkeyEmail.value, session_key },
    })
    auth.setUser(finish.data.user)
    message.success('登录成功')
    router.push('/')
  } catch (e: any) {
    message.error(errorMessage(e))
  } finally {
    passkeyLoading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}
.login-card {
  width: 400px;
  padding: 8px;
}
.login-title {
  font-size: 20px;
  font-weight: 600;
  text-align: center;
  margin-bottom: 16px;
}
.oauth-row {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}
.oauth-row a {
  flex: 1;
  text-decoration: none;
}
.passkey-form {
  margin-top: 8px;
}
</style>
