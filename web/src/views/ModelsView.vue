<template>
  <div class="models">
    <n-tabs type="line" animated>
      <!-- 供应商 -->
      <n-tab-pane name="providers" tab="供应商">
        <n-card>
          <div class="toolbar">
            <n-button type="primary" @click="openProviderCreate">新建供应商</n-button>
          </div>
          <n-data-table :columns="providerColumns" :data="providers" :loading="providersLoading" :pagination="false" :bordered="false" />
        </n-card>
      </n-tab-pane>

      <!-- 模型 -->
      <n-tab-pane name="models" tab="模型">
        <n-card>
          <div class="toolbar">
            <n-button type="primary" @click="openModelCreate">新建模型</n-button>
            <n-button @click="openImportModal">从供应商导入</n-button>
          </div>
          <n-data-table :columns="modelColumns" :data="models" :loading="modelsLoading" :pagination="false" :bordered="false" />
        </n-card>
      </n-tab-pane>
    </n-tabs>

    <!-- 供应商新建/编辑 -->
    <n-modal v-model:show="providerShow" preset="card" :title="providerEditing ? '编辑供应商' : '新建供应商'" style="width: 460px">
      <n-form>
        <n-form-item label="名称">
          <n-input v-model:value="providerForm.name" placeholder="例如:OpenAI" />
        </n-form-item>
        <n-form-item label="Base URL">
          <n-input v-model:value="providerForm.base_url" placeholder="https://api.openai.com/v1" />
        </n-form-item>
        <n-form-item label="API Key">
          <n-input v-model:value="providerForm.api_key" type="password" show-password-on="click" :placeholder="providerEditing ? '留空则不修改' : '上游 API Key'" />
        </n-form-item>
        <n-form-item label="协议">
          <n-select v-model:value="providerForm.protocol" :options="protocolOptions" />
        </n-form-item>
        <n-form-item v-if="providerEditing" label="状态">
          <n-select v-model:value="providerForm.status" :options="activeStatusOptions" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="providerShow = false">取消</n-button>
          <n-button type="primary" :loading="providerSaving" @click="onProviderSave">保存</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 导入弹窗 -->
    <n-modal v-model:show="importShow" preset="card" title="从供应商导入模型" style="width: 560px">
      <n-form>
        <n-form-item label="供应商">
          <n-select v-model:value="importProviderId" :options="providerOptions" placeholder="选择供应商" @update:value="onFetchModels" />
        </n-form-item>
        <div v-if="fetchedModels.length">
          <n-checkbox-group v-model:value="importSelected">
            <n-space vertical>
              <n-checkbox v-for="m in fetchedModels" :key="m.name" :value="m.name" :disabled="m.exists">
                {{ m.name }} <n-tag v-if="m.exists" size="small" type="warning">已存在</n-tag>
              </n-checkbox>
            </n-space>
          </n-checkbox-group>
        </div>
        <n-empty v-else-if="importProviderId" description="点击上方供应商后获取模型列表" />
      </n-form>
      <template #footer>
        <n-button @click="importShow = false">取消</n-button>
        <n-button type="primary" :loading="importing" :disabled="!importSelected.length" @click="onImport">导入 ({{ importSelected.length }})</n-button>
      </template>
    </n-modal>

    <!-- 供应商多 API Key 管理 -->
    <n-modal v-model:show="keyShow" preset="card" :title="keyProvider ? `管理 API Key - ${keyProvider.name}` : '管理 API Key'" style="width: 760px">
      <n-form inline>
        <n-form-item label="API Key">
          <n-input v-model:value="keyForm.api_key" type="password" show-password-on="click" placeholder="sk-..." style="width: 320px" />
        </n-form-item>
        <n-form-item label="标签">
          <n-input v-model:value="keyForm.label" placeholder="可选,如:team-a" style="width: 160px" />
        </n-form-item>
        <n-form-item>
          <n-button type="primary" :loading="keySaving" @click="onAddKey">添加</n-button>
        </n-form-item>
      </n-form>
      <n-alert v-if="keyHint" type="info" :show-icon="false" class="key-hint">{{ keyHint }}</n-alert>
      <n-data-table :columns="keyColumns" :data="keys" :loading="keysLoading" :pagination="false" :bordered="false" size="small" />
    </n-modal>

    <!-- API Key 调用日志 -->
    <n-modal v-model:show="keyLogShow" preset="card" title="API Key 调用日志" style="width: 820px">
      <n-spin :show="keyLogLoading">
        <n-descriptions v-if="keyLogData.key" :column="3" size="small" bordered class="key-log-desc">
          <n-descriptions-item label="掩码">{{ keyLogData.key.masked }}</n-descriptions-item>
          <n-descriptions-item label="标签">{{ keyLogData.key.label || '—' }}</n-descriptions-item>
          <n-descriptions-item label="状态">
            <n-tag size="small" :type="keyStatusType(keyLogData.key.status)">{{ keyStatusText(keyLogData.key.status) }}</n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="优先级">{{ keyLogData.key.priority }}</n-descriptions-item>
          <n-descriptions-item label="失败次数">{{ keyLogData.key.fail_count }}</n-descriptions-item>
          <n-descriptions-item label="创建时间">{{ fmtTime(keyLogData.key.created_at) }}</n-descriptions-item>
        </n-descriptions>
        <n-tabs type="line" animated class="key-log-tabs">
          <n-tab-pane name="events" tab="生命周期事件">
            <n-data-table :columns="eventColumns" :data="keyLogData.events || []" :pagination="false" :bordered="false" size="small" :max-height="340" />
          </n-tab-pane>
          <n-tab-pane name="requests" tab="调用记录">
            <n-data-table :columns="reqColumns" :data="keyLogData.requests || []" :pagination="false" :bordered="false" size="small" :max-height="340" />
          </n-tab-pane>
        </n-tabs>
      </n-spin>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, h } from 'vue'
import { useRouter } from 'vue-router'
import type { DataTableColumns } from 'naive-ui'
import {
  NButton, NCard, NDataTable, NTabs, NTabPane, NForm, NFormItem, NInput,
  NModal, NSelect, NEmpty, NAlert, NSpin,
  NSwitch, NPopconfirm, NCheckboxGroup, NCheckbox, NSpace, NTag,
  NDescriptions, NDescriptionsItem, useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'
import { formatMoney, loadCurrency } from '../utils/currency'

const message = useMessage()
const router = useRouter()
const systemCurrency = ref('USD')

const protocolOptions = [
  { label: 'OpenAI Chat', value: 'openai_chat' },
  { label: 'OpenAI Responses', value: 'openai_responses' },
  { label: 'Anthropic', value: 'anthropic' },
]
const activeStatusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

// ---- providers ----
const providers = ref<any[]>([])
const providersLoading = ref(false)
const providerColumns = [
  { title: 'ID', key: 'id' },
  { title: '名称', key: 'name' },
  { title: 'Base URL', key: 'base_url' },
  { title: '协议', key: 'protocol' },
  { title: '状态', key: 'status' },
  {
    title: 'API Key',
    key: 'keys',
    render(row: any) {
      const parts = [`${row.key_count || 0} 个`]
      if ((row.active_key_count || 0) > 0) parts.push(`可用 ${row.active_key_count}`)
      if ((row.cooling_key_count || 0) > 0) parts.push(`冷却 ${row.cooling_key_count}`)
      return h('span', parts.join(' · '))
    },
  },
  {
    title: '测试',
    key: 'test',
    render(row: any) {
      return h('span', { class: row._testResult?.startsWith('可用') ? 'ok' : 'bad' }, row._testResult || '-')
    },
  },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', loading: row._testing, onClick: () => onProviderTest(row) }, { default: () => '测试' }),
        h(NButton, { size: 'small', onClick: () => openKeyManage(row) }, { default: () => 'Keys' }),
        h(NButton, { size: 'small', onClick: () => openProviderEdit(row) }, { default: () => '编辑' }),
        h(NPopconfirm, { onPositiveClick: () => onProviderDelete(row) }, {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
          default: () => '确定删除该供应商?',
        }),
      ])
    },
  },
]

const providerOptions = computed(() => providers.value.map((p) => ({ label: p.name, value: p.id })))

async function loadProviders() {
  providersLoading.value = true
  try {
    const res = await http.get('/api/providers')
    providers.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    providersLoading.value = false
  }
}

const providerShow = ref(false)
const providerEditing = ref(false)
const providerSaving = ref(false)
const providerForm = reactive({ id: 0, name: '', base_url: '', api_key: '', protocol: 'openai_chat', status: 'active' })

function openProviderCreate() {
  providerEditing.value = false
  providerForm.id = 0
  providerForm.name = ''
  providerForm.base_url = ''
  providerForm.api_key = ''
  providerForm.protocol = 'openai_chat'
  providerForm.status = 'active'
  providerShow.value = true
}
function openProviderEdit(row: any) {
  providerEditing.value = true
  providerForm.id = row.id
  providerForm.name = row.name
  providerForm.base_url = row.base_url
  providerForm.api_key = ''
  providerForm.protocol = row.protocol
  providerForm.status = row.status || 'active'
  providerShow.value = true
}
async function onProviderSave() {
  if (!providerForm.name || !providerForm.base_url) {
    message.warning('请填写名称与 Base URL')
    return
  }
  providerSaving.value = true
  try {
    if (providerEditing.value) {
      const body: any = {
        name: providerForm.name,
        base_url: providerForm.base_url,
        protocol: providerForm.protocol,
        status: providerForm.status,
      }
      if (providerForm.api_key) body.api_key = providerForm.api_key
      await http.put(`/api/providers/${providerForm.id}`, body)
    } else {
      await http.post('/api/providers', {
        name: providerForm.name,
        base_url: providerForm.base_url,
        api_key: providerForm.api_key,
        protocol: providerForm.protocol,
      })
    }
    providerShow.value = false
    message.success('已保存')
    loadProviders()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    providerSaving.value = false
  }
}
async function onProviderDelete(row: any) {
  try {
    await http.delete(`/api/providers/${row.id}`)
    message.success('已删除')
    loadProviders()
  } catch (e) {
    message.error(errorMessage(e))
  }
}
async function onProviderTest(row: any) {
  row._testing = true
  try {
    const res = await http.post(`/api/providers/${row.id}/test`)
    const d = res.data || {}
    row._testResult = d.ok
      ? `可用 ${d.latency_ms}ms`
      : `不可用: ${d.error || ''}`
  } catch (e) {
    row._testResult = '测试失败'
  } finally {
    row._testing = false
  }
}

// ---- provider api keys(多 key 池) ----
const keyShow = ref(false)
const keysLoading = ref(false)
const keySaving = ref(false)
const keyProvider = ref<any>(null)
const keys = ref<any[]>([])
const keyForm = reactive({ api_key: '', label: '' })
const keyHint = ref('')

async function openKeyManage(row: any) {
  keyProvider.value = row
  keyForm.api_key = ''
  keyForm.label = ''
  keyShow.value = true
  await loadKeys(row.id)
}
async function loadKeys(providerId: number) {
  keysLoading.value = true
  try {
    const res = await http.get(`/api/providers/${providerId}/keys`)
    keys.value = res.data?.keys || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    keysLoading.value = false
  }
}
async function onAddKey() {
  if (!keyProvider.value) return
  if (!keyForm.api_key.trim()) {
    message.warning('请填写 API Key')
    return
  }
  keySaving.value = true
  try {
    await http.post(`/api/providers/${keyProvider.value.id}/keys`, {
      api_key: keyForm.api_key.trim(),
      label: keyForm.label.trim(),
    })
    message.success('已添加')
    keyForm.api_key = ''
    keyForm.label = ''
    keyHint.value = '新 key 已追加到池尾;同一用户的请求会优先使用其用过的 key,以提高缓存命中率。'
    loadKeys(keyProvider.value.id)
    loadProviders()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    keySaving.value = false
  }
}
async function onDeleteKey(row: any) {
  if (!keyProvider.value) return
  try {
    await http.delete(`/api/providers/${keyProvider.value.id}/keys/${row.id}`)
    message.success('已删除')
    loadKeys(keyProvider.value.id)
    loadProviders()
  } catch (e) {
    message.error(errorMessage(e))
  }
}

function keyStatusText(s: string) {
  return { active: '启用', cooling_down: '冷却中', deleted: '已删除' }[s] || s
}
function keyStatusType(s: string) {
  if (s === 'active') return 'success'
  if (s === 'cooling_down') return 'warning'
  return 'default'
}

const keyColumns = [
  { title: '掩码', key: 'masked' },
  { title: '标签', key: 'label' },
  {
    title: '状态',
    key: 'status',
    render(row: any) {
      return h(NTag, { size: 'small', type: keyStatusType(row.status) }, { default: () => keyStatusText(row.status) })
    },
  },
  { title: '优先级', key: 'priority' },
  { title: '失败次数', key: 'fail_count' },
  {
    title: '下次重试',
    key: 'retry_after',
    render(row: any) {
      return row.retry_after ? fmtTime(row.retry_after) : '—'
    },
  },
  {
    title: '最后使用',
    key: 'last_used_at',
    render(row: any) {
      return row.last_used_at ? fmtTime(row.last_used_at) : '—'
    },
  },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openKeyLog(row) }, { default: () => '日志' }),
        h(NPopconfirm, { onPositiveClick: () => onDeleteKey(row) }, {
          trigger: () => h(NButton, { size: 'small', type: 'error', disabled: row.status === 'deleted' }, { default: () => '删除' }),
          default: () => '确定删除该上游 Key?',
        }),
      ])
    },
  },
]

// ---- api key 调用日志 ----
const keyLogShow = ref(false)
const keyLogLoading = ref(false)
const keyLogData = ref<any>({ key: null, events: [], requests: [] })

async function openKeyLog(row: any) {
  if (!keyProvider.value) return
  keyLogData.value = { key: null, events: [], requests: [] }
  keyLogShow.value = true
  keyLogLoading.value = true
  try {
    const res = await http.get(`/api/providers/${keyProvider.value.id}/keys/${row.id}/logs`)
    keyLogData.value = res.data || { key: null, events: [], requests: [] }
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    keyLogLoading.value = false
  }
}

const eventColumns = [
  { title: '时间', key: 'created_at', render(row: any) { return fmtTime(row.created_at) } },
  { title: '事件', key: 'event', render(row: any) { return eventText(row.event) } },
  { title: '详情', key: 'detail' },
]
const reqColumns = [
  { title: '时间', key: 'created_at', render(row: any) { return fmtTime(row.created_at) } },
  { title: '用户', key: 'email' },
  { title: '模型', key: 'custom_model' },
  { title: '上游模型', key: 'upstream_model' },
  { title: '状态码', key: 'status_code' },
  { title: '错误', key: 'error_type' },
  { title: '错误信息', key: 'error_message' },
]

function eventText(e: string) {
  const map: Record<string, string> = {
    created: '创建',
    updated: '更新',
    degraded: '降级(移到优先级末尾并冷却 1h)',
    retry_started: '后台重试开始',
    retry_success: '后台重试成功',
    retry_failed: '后台重试失败',
    recovered: '恢复可用',
    deleted: '已删除(重试仍失败)',
    deleted_manual: '已删除(手动)',
  }
  return map[e] || e
}

function fmtTime(v: any) {
  if (!v) return '—'
  const d = new Date(v)
  if (isNaN(d.getTime())) return String(v)
  return d.toLocaleString()
}

// ---- models ----
const models = ref<any[]>([])
const modelsLoading = ref(false)
const modelColumns: DataTableColumns<any> = [
  { title: 'ID', key: 'id' },
  { title: '名称', key: 'name' },
  {
    title: '启用',
    key: 'enabled',
    render(row: any) {
      return h(NSwitch, { value: row.enabled, onUpdateValue: (v: boolean) => onToggleModel(row, v) })
    },
  },
  {
    title: '配额',
    key: 'quota',
    render(row: any) {
      const q = row.quota
      if (!q || !q.id) return '—'
      const parts: string[] = []
      if (q.limit_tokens != null) parts.push('Token ' + q.limit_tokens)
      if (q.limit_cost != null) parts.push('费用 ' + formatMoney(q.limit_cost, systemCurrency.value))
      if (q.used_tokens > 0 || q.used_cost > 0) {
        parts.push('已用 ' + q.used_tokens + ' / ' + formatMoney(q.used_cost, systemCurrency.value))
      }
      return parts.length ? parts.join(' · ') : '—'
    },
  },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openModelEdit(row) }, { default: () => '编辑' }),
        h(NPopconfirm, { onPositiveClick: () => onModelDelete(row) }, {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
          default: () => '确定删除该模型?',
        }),
      ])
    },
  },
]

async function loadModels() {
  modelsLoading.value = true
  try {
    const res = await http.get('/api/models')
    models.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    modelsLoading.value = false
  }
}

function openModelCreate() {
  router.push({ name: 'model-new' })
}
function openModelEdit(row: any) {
  router.push({ name: 'model-edit', params: { id: row.id } })
}
async function onToggleModel(row: any, enabled: boolean) {
  try {
    const price = row.price || {}
    const body: any = {
      name: row.name,
      enabled,
      currency: price.currency || 'USD',
      input_price: price.input_price ?? 0,
      output_price: price.output_price ?? 0,
    }
    if (price.cache_read_price != null) body.cache_read_price = price.cache_read_price
    if (price.cache_write_price != null) body.cache_write_price = price.cache_write_price
    await http.put(`/api/models/${row.id}`, body)
    row.enabled = enabled
    message.success(enabled ? '已启用' : '已禁用')
  } catch (e) {
    message.error(errorMessage(e))
  }
}
async function onModelDelete(row: any) {
  try {
    await http.delete(`/api/models/${row.id}`)
    message.success('已删除')
    loadModels()
  } catch (e) {
    message.error(errorMessage(e))
  }
}

// ---- import from provider ----
const importShow = ref(false)
const importProviderId = ref<number | null>(null)
const fetchedModels = ref<any[]>([])
const importSelected = ref<string[]>([])
const importing = ref(false)

function openImportModal() {
  importProviderId.value = null
  fetchedModels.value = []
  importSelected.value = []
  importShow.value = true
}
async function onFetchModels() {
  if (importProviderId.value == null) return
  fetchedModels.value = []
  importSelected.value = []
  try {
    const res = await http.post(`/api/providers/${importProviderId.value}/models/fetch`)
    fetchedModels.value = res.data?.models || []
  } catch (e) {
    message.error(errorMessage(e))
  }
}
async function onImport() {
  if (!importProviderId.value) return
  importing.value = true
  try {
    const items = importSelected.value.map((name) => ({ provider_id: importProviderId.value, upstream_model: name }))
    const res = await http.post('/api/models/import', { items })
    const d = res.data || {}
    message.success(`导入 ${d.imported} 个,跳过 ${d.skipped} 个`)
    importShow.value = false
    loadModels()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    importing.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadProviders(), loadModels()])
  loadCurrency().then((c) => { systemCurrency.value = c })
})
</script>

<style scoped>
.toolbar {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.row-actions {
  display: flex;
  gap: 8px;
}
.ok {
  color: #18a058;
}
.bad {
  color: #d03050;
}
.key-hint {
  margin-bottom: 12px;
}
.key-log-desc {
  margin-bottom: 12px;
}
.key-log-tabs {
  margin-top: 8px;
}
</style>
