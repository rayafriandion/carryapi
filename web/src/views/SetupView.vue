<template>
  <div class="setup-page">
    <n-card class="setup-card" :bordered="false">
      <div class="setup-title">carryAPI · 首次设置</div>
      <p class="setup-sub">请设置管理员账户，用于登录管理后台。</p>
      <n-form ref="formRef" :model="form" :rules="rules">
        <n-form-item label="邮箱" path="email">
          <n-input v-model:value="form.email" placeholder="admin@example.com" />
        </n-form-item>
        <n-form-item label="密码" path="password">
          <n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="至少 8 位" />
        </n-form-item>
        <n-form-item label="确认密码" path="confirm">
          <n-input v-model:value="form.confirm" type="password" show-password-on="click" placeholder="再次输入密码" @keydown.enter="onSubmit" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="onSubmit">创建管理员</n-button>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NInput, NButton, NForm, NFormItem, NCard } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { http, errorMessage } from '../api/http'

const router = useRouter()
const message = useMessage()

const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const form = reactive({ email: '', password: '', confirm: '' })

const rules: FormRules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少 8 位', trigger: 'blur' },
  ],
  confirm: [
    {
      validator: (_rule, value: string) => value === form.password,
      message: '两次输入的密码不一致',
      trigger: 'blur',
    },
  ],
}

async function onSubmit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    await http.post('/api/setup/admin', { email: form.email, password: form.password })
    message.success('管理员已创建，请登录')
    router.push('/login')
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}
.setup-card {
  width: 380px;
}
.setup-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 4px;
}
.setup-sub {
  color: #999;
  margin-bottom: 16px;
}
</style>
