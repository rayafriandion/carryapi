<template>
  <div class="model-edit">
    <n-card>
      <div class="page-head">
        <n-button quaternary size="small" @click="goBack">← 返回</n-button>
        <div class="title">{{ isEdit ? '编辑模型' : '新建模型' }}</div>
        <div class="head-actions">
          <n-button @click="goBack">取消</n-button>
          <n-button type="primary" :loading="saving" @click="onSave">保存</n-button>
        </div>
      </div>
    </n-card>

    <n-spin :show="loading">
      <n-card class="section" title="基本信息">
        <n-form label-placement="left" :label-width="100">
          <n-form-item label="名称" required>
            <n-input v-model:value="form.name" placeholder="对外模型名,例如 gpt-4o" />
          </n-form-item>
          <n-form-item label="供应商" required>
            <n-select v-model:value="form.provider_id" :options="providerOptions" placeholder="选择供应商" />
          </n-form-item>
          <n-form-item label="上游模型" required>
            <n-input v-model:value="form.upstream_model" placeholder="上游实际模型名" />
          </n-form-item>
          <n-form-item label="启用">
            <n-switch v-model:value="form.enabled" />
          </n-form-item>
          <n-form-item label="路由策略">
            <n-select v-model:value="form.routing_strategy" :options="routingStrategyOptions" />
          </n-form-item>
          <n-form-item v-if="form.routing_strategy === 'auto'" label="自动模式">
            <n-select v-model:value="form.auto_mode" :options="autoModeOptions" />
          </n-form-item>
        </n-form>
      </n-card>

      <n-card class="section" title="定价">
        <p class="hint">按每百万 token 计费。币种、输入/输出价格必填,缓存读/写可选。</p>
        <n-form label-placement="left" :label-width="140">
          <n-form-item label="币种" required>
            <n-select v-model:value="form.currency" :options="currencyOptions" style="max-width: 240px" />
          </n-form-item>
          <n-form-item label="输入价格" required>
            <n-input-number v-model:value="form.input_price" :min="0" :step="0.1" placeholder="每百万 token" style="width: 100%" />
          </n-form-item>
          <n-form-item label="输出价格" required>
            <n-input-number v-model:value="form.output_price" :min="0" :step="0.1" placeholder="每百万 token" style="width: 100%" />
          </n-form-item>
          <n-form-item label="缓存读价格">
            <n-input-number v-model:value="form.cache_read_price" :min="0" :step="0.1" placeholder="可选,每百万 token" clearable style="width: 100%" />
          </n-form-item>
          <n-form-item label="缓存写价格">
            <n-input-number v-model:value="form.cache_write_price" :min="0" :step="0.1" placeholder="可选,每百万 token" clearable style="width: 100%" />
          </n-form-item>
        </n-form>
      </n-card>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NCard, NButton, NForm, NFormItem, NInput, NInputNumber, NSelect,
  NSwitch, NSpin, useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const isEdit = computed(() => route.name === 'model-edit')
const modelId = computed(() => Number(route.params.id))

const providers = ref<any[]>([])
const providerOptions = computed(() => providers.value.map((p) => ({ label: p.name, value: p.id })))

const currencyOptions = [
  { label: '美元 (USD $)', value: 'USD' },
  { label: '人民币 (CNY ￥)', value: 'CNY' },
]
const routingStrategyOptions = [
  { label: '自动路由', value: 'auto' },
  { label: '随机使用', value: 'random' },
]
const autoModeOptions = [
  { label: '优先级', value: 'priority' },
  { label: '故障转移', value: 'failover' },
  { label: '健康感知', value: 'health' },
]

const form = reactive({
  name: '',
  provider_id: null as number | null,
  upstream_model: '',
  enabled: true,
  routing_strategy: 'auto',
  auto_mode: 'priority',
  currency: 'USD',
  input_price: null as number | null,
  output_price: null as number | null,
  cache_read_price: null as number | null,
  cache_write_price: null as number | null,
})

const loading = ref(false)
const saving = ref(false)

async function loadProviders() {
  const res = await http.get('/api/providers')
  providers.value = res.data || []
}

async function loadModel() {
  if (!isEdit.value) return
  loading.value = true
  try {
    const res = await http.get('/api/models')
    const m = (res.data || []).find((x: any) => x.id === modelId.value)
    if (!m) {
      message.error('模型不存在')
      goBack()
      return
    }
    form.name = m.name
    form.provider_id = m.provider_id
    form.upstream_model = m.upstream_model
    form.enabled = !!m.enabled
    form.routing_strategy = m.routing_strategy || 'auto'
    form.auto_mode = m.auto_mode || 'priority'
    const p = m.price
    if (p) {
      form.currency = p.currency || 'USD'
      form.input_price = p.input_price ?? null
      form.output_price = p.output_price ?? null
      form.cache_read_price = p.cache_read_price ?? null
      form.cache_write_price = p.cache_write_price ?? null
    }
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push({ name: 'models' })
}

function validate(): string | null {
  if (!form.name.trim()) return '请填写名称'
  if (!form.provider_id) return '请选择供应商'
  if (!form.upstream_model.trim()) return '请填写上游模型'
  if (!form.currency) return '请选择币种'
  if (form.input_price == null) return '请填写输入价格'
  if (form.output_price == null) return '请填写输出价格'
  if (form.input_price < 0 || form.output_price < 0) return '价格不能为负'
  return null
}

async function onSave() {
  const err = validate()
  if (err) {
    message.warning(err)
    return
  }
  const body: any = {
    name: form.name,
    provider_id: form.provider_id,
    upstream_model: form.upstream_model,
    enabled: form.enabled,
    routing_strategy: form.routing_strategy,
    auto_mode: form.routing_strategy === 'auto' ? form.auto_mode : '',
    currency: form.currency,
    input_price: form.input_price,
    output_price: form.output_price,
  }
  if (form.cache_read_price != null) body.cache_read_price = form.cache_read_price
  if (form.cache_write_price != null) body.cache_write_price = form.cache_write_price

  saving.value = true
  try {
    if (isEdit.value) {
      await http.put(`/api/models/${modelId.value}`, body)
    } else {
      await http.post('/api/models', body)
    }
    message.success('已保存')
    router.push({ name: 'models' })
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await loadProviders()
  await loadModel()
})
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.title {
  font-size: 16px;
  font-weight: 600;
  flex: 1;
}
.head-actions {
  display: flex;
  gap: 8px;
}
.section {
  margin-top: 16px;
}
.hint {
  color: #999;
  font-size: 12px;
  margin: 0 0 16px;
}
</style>
