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
          <n-form-item label="启用">
            <n-switch v-model:value="form.enabled" />
          </n-form-item>
          <n-form-item label="路由策略">
            <n-select
              v-model:value="form.routing_strategy"
              :options="routingStrategyOptions"
              style="max-width: 220px"
            />
          </n-form-item>
          <n-form-item v-if="form.routing_strategy === 'auto'" label="自动模式">
            <n-select
              v-model:value="form.auto_mode"
              :options="autoModeOptions"
              style="max-width: 220px"
            />
          </n-form-item>
        </n-form>
      </n-card>

      <n-card class="section" title="上游绑定">
        <template #header-extra>
          <n-button size="small" type="primary" @click="addBinding">添加绑定</n-button>
        </template>
        <p class="hint">
          一个对外模型可绑定多个供应商+上游模型,按优先级/权重参与路由。至少需要一条绑定。
        </p>
        <n-data-table
          :columns="bindingColumns"
          :data="form.bindings"
          :pagination="false"
          :bordered="false"
          size="small"
          :row-key="(row: BindingRow) => row._key"
        />
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
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { DataTableColumns } from 'naive-ui'
import {
  NCard, NButton, NForm, NFormItem, NInput, NInputNumber, NSelect,
  NSwitch, NSpin, NDataTable, NPopconfirm, useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const isEdit = computed(() => route.name === 'model-edit')
const modelId = computed(() => Number(route.params.id))

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

interface BindingRow {
  _key: number
  id?: number
  provider_id: number | null
  upstream_model: string
  priority: number
  weight: number
  enabled: boolean
  _saving?: boolean
}

let keySeq = 1

const form = reactive({
  name: '',
  enabled: true,
  routing_strategy: 'auto',
  auto_mode: 'priority',
  currency: 'USD',
  input_price: null as number | null,
  output_price: null as number | null,
  cache_read_price: null as number | null,
  cache_write_price: null as number | null,
  bindings: [] as BindingRow[],
})

const providers = ref<any[]>([])
const providerOptions = computed(() =>
  providers.value.map((p) => ({ label: p.name, value: p.id })),
)

const loading = ref(false)
const saving = ref(false)

function addBinding() {
  form.bindings.push({
    _key: keySeq++,
    provider_id: null,
    upstream_model: '',
    priority: 100,
    weight: 1,
    enabled: true,
  })
}

function removeBinding(key: number) {
  const idx = form.bindings.findIndex((b) => b._key === key)
  if (idx >= 0) form.bindings.splice(idx, 1)
}

const bindingColumns: DataTableColumns<BindingRow> = [
  {
    title: '供应商',
    key: 'provider_id',
    render(row) {
      return h(NSelect, {
        value: row.provider_id,
        options: providerOptions.value,
        placeholder: '选择供应商',
        filterable: true,
        size: 'small',
        style: 'min-width: 160px',
        onUpdateValue: (v: number) => { row.provider_id = v },
      })
    },
  },
  {
    title: '上游模型',
    key: 'upstream_model',
    render(row) {
      return h(NInput, {
        value: row.upstream_model,
        placeholder: '例如 gpt-4o-2024-08-06',
        size: 'small',
        onUpdateValue: (v: string) => { row.upstream_model = v },
      })
    },
  },
  {
    title: '优先级',
    key: 'priority',
    width: 120,
    render(row) {
      return h(NInputNumber, {
        value: row.priority,
        min: 1,
        size: 'small',
        style: 'width: 100%',
        onUpdateValue: (v: number | null) => { if (v != null) row.priority = v },
      })
    },
  },
  {
    title: '权重',
    key: 'weight',
    width: 110,
    render(row) {
      return h(NInputNumber, {
        value: row.weight,
        min: 1,
        size: 'small',
        style: 'width: 100%',
        onUpdateValue: (v: number | null) => { if (v != null) row.weight = v },
      })
    },
  },
  {
    title: '启用',
    key: 'enabled',
    width: 70,
    render(row) {
      return h(NSwitch, {
        value: row.enabled,
        size: 'small',
        onUpdateValue: (v: boolean) => { row.enabled = v },
      })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 90,
    render(row) {
      return h(NPopconfirm, {
        disabled: form.bindings.length <= 1,
        onPositiveClick: () => removeBinding(row._key),
      }, {
        trigger: () => h(NButton, {
          size: 'small',
          type: 'error',
          disabled: form.bindings.length <= 1,
        }, { default: () => '删除' }),
        default: () => '确定删除该绑定?',
      })
    },
  },
]

async function loadProviders() {
  try {
    const res = await http.get('/api/providers')
    providers.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  }
}

async function loadModel() {
  if (!isEdit.value) {
    addBinding()
    return
  }
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
    const bindings = m.bindings || []
    if (bindings.length === 0) {
      addBinding()
    } else {
      for (const b of bindings) {
        form.bindings.push({
          _key: keySeq++,
          id: b.id,
          provider_id: b.provider_id,
          upstream_model: b.upstream_model || '',
          priority: b.priority ?? 100,
          weight: b.weight ?? 1,
          enabled: !!b.enabled,
        })
      }
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
  if (!form.currency) return '请选择币种'
  if (form.input_price == null) return '请填写输入价格'
  if (form.output_price == null) return '请填写输出价格'
  if (form.input_price < 0 || form.output_price < 0) return '价格不能为负'
  if (form.bindings.length === 0) return '至少需要一条上游绑定'
  for (let i = 0; i < form.bindings.length; i++) {
    const b = form.bindings[i]
    if (!b.provider_id) return `第 ${i + 1} 条绑定未选择供应商`
    if (!b.upstream_model.trim()) return `第 ${i + 1} 条绑定未填写上游模型`
    if (b.priority <= 0 || b.weight <= 0) return `第 ${i + 1} 条绑定的优先级/权重需大于 0`
  }
  return null
}

function priceBody() {
  const body: any = {
    name: form.name,
    enabled: form.enabled,
    currency: form.currency,
    input_price: form.input_price,
    output_price: form.output_price,
    routing_strategy: form.routing_strategy,
    auto_mode: form.routing_strategy === 'auto' ? form.auto_mode : '',
  }
  if (form.cache_read_price != null) body.cache_read_price = form.cache_read_price
  if (form.cache_write_price != null) body.cache_write_price = form.cache_write_price
  return body
}

async function saveExisting() {
  // 1) 更新模型基本信息 + 定价 + 路由策略
  await http.put(`/api/models/${modelId.value}`, priceBody())

  // 2) 同步 bindings:已存在的更新、新的创建、被移除的删除
  const current = await http.get(`/api/models/${modelId.value}/bindings`)
  const existing: any[] = current.data?.bindings || []
  const keptIds = new Set(form.bindings.filter((b) => b.id).map((b) => b.id))

  for (const old of existing) {
    if (!keptIds.has(old.id)) {
      await http.delete(`/api/models/${modelId.value}/bindings/${old.id}`)
    }
  }
  for (const b of form.bindings) {
    const payload = {
      provider_id: b.provider_id,
      upstream_model: b.upstream_model.trim(),
      priority: b.priority,
      weight: b.weight,
      enabled: b.enabled,
    }
    if (b.id) {
      await http.put(`/api/models/${modelId.value}/bindings/${b.id}`, payload)
    } else {
      await http.post(`/api/models/${modelId.value}/bindings`, payload)
    }
  }
}

async function saveNew() {
  const body = {
    ...priceBody(),
    bindings: form.bindings.map((b) => ({
      provider_id: b.provider_id,
      upstream_model: b.upstream_model.trim(),
      priority: b.priority,
      weight: b.weight,
      enabled: b.enabled,
    })),
  }
  await http.post('/api/models', body)
}

async function onSave() {
  const err = validate()
  if (err) {
    message.warning(err)
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await saveExisting()
    } else {
      await saveNew()
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
  await Promise.all([loadProviders(), loadModel()])
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
