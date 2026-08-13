<template>
  <div class="routing">
    <n-card>
      <div class="toolbar">
        <n-button type="primary" :loading="loading" @click="loadStatus">刷新</n-button>
        <span class="hint">模型与上游绑定的 24 小时健康状态时间轴(6 格 = 4 小时/格)</span>
      </div>
      <n-data-table
        :columns="modelColumnsWithExpand"
        :data="models"
        :loading="loading"
        :pagination="false"
        :bordered="false"
        :row-key="(row: RouteModel) => row.model_id"
        :expanded-row-keys="expandedRowKeys"
        @update:expanded-row-keys="(keys: any) => (expandedRowKeys = keys)"
      />
    </n-card>

    <!-- 添加 / 编辑 binding 弹窗 -->
    <n-modal
      v-model:show="editModal.show"
      preset="card"
      :title="editModal.binding ? '编辑绑定' : '添加绑定'"
      style="width: 560px"
    >
      <n-form>
        <n-form-item label="供应商">
          <n-select v-model:value="bindingForm.provider_id" :options="providerOptions" placeholder="选择供应商" />
        </n-form-item>
        <n-form-item label="上游模型">
          <n-input v-model:value="bindingForm.upstream_model" placeholder="上游实际模型名" />
        </n-form-item>
        <div class="price-fields">
          <n-form-item label="优先级">
            <n-input-number v-model:value="bindingForm.priority" :min="1" />
          </n-form-item>
          <n-form-item label="权重">
            <n-input-number v-model:value="bindingForm.weight" :min="1" />
          </n-form-item>
          <n-form-item label="启用">
            <n-switch v-model:value="bindingForm.enabled" />
          </n-form-item>
        </div>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="editModal.show = false">取消</n-button>
          <n-button type="primary" :loading="bindingSaving" @click="onBindingSave">
            {{ editModal.binding ? '更新绑定' : '添加绑定' }}
          </n-button>
        </div>
      </template>
    </n-modal>

    <!-- 路由策略弹窗 -->
    <n-modal v-model:show="routingShow" preset="card" title="路由策略" style="width: 460px">
      <n-form>
        <n-form-item label="路由策略">
          <n-select v-model:value="routingForm.routing_strategy" :options="routingStrategyOptions" />
        </n-form-item>
        <n-form-item v-if="routingForm.routing_strategy === 'auto'" label="自动模式">
          <n-select v-model:value="routingForm.auto_mode" :options="autoModeOptions" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="routingShow = false">取消</n-button>
          <n-button type="primary" :loading="routingSaving" @click="onRoutingSave">保存</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 指标详情弹窗 -->
    <n-modal v-model:show="metricsModal.show" preset="card" title="绑定 24h 指标详情" style="width: 520px">
      <div v-if="metricsModal.data" class="metrics-grid">
        <div class="metric"><span class="metric-label">供应商</span><span class="metric-value">{{ metricsModal.data.provider_name }}</span></div>
        <div class="metric"><span class="metric-label">上游模型</span><span class="metric-value">{{ metricsModal.data.upstream_model }}</span></div>
        <div class="metric"><span class="metric-label">平均延迟</span><span class="metric-value">{{ metricsModal.data.avg_latency_ms }} ms</span></div>
        <div class="metric"><span class="metric-label">平均 TTFT</span><span class="metric-value">{{ metricsModal.data.avg_ttft_ms }} ms</span></div>
        <div class="metric"><span class="metric-label">吞吐(/小时)</span><span class="metric-value">{{ metricsModal.data.throughput_per_hour }}</span></div>
        <div class="metric"><span class="metric-label">24h 请求数</span><span class="metric-value">{{ metricsModal.data.total_requests_24h }}</span></div>
        <div class="metric"><span class="metric-label">成功率</span><span class="metric-value">{{ formatRate(metricsModal.data.success_rate) }}</span></div>
      </div>
      <n-empty v-else description="暂无数据" />
      <template #footer>
        <n-button @click="metricsModal.show = false">关闭</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NInput, NInputNumber,
  NModal, NSelect, NSwitch, NPopconfirm, NEmpty, useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'

interface Binding {
  binding_id: number
  provider_id: number
  provider_name: string
  provider_status: string
  upstream_model: string
  priority: number
  weight: number
  enabled: boolean
  timeline: string[]
  avg_latency_ms: number
  last_request_at: string | null
}
interface RouteModel {
  model_id: number
  name: string
  enabled: boolean
  routing_strategy: string
  auto_mode: string
  bindings: Binding[]
}
interface Provider { id: number; name: string; status: string }

const message = useMessage()
const models = ref<RouteModel[]>([])
const providers = ref<Provider[]>([])
const loading = ref(false)
const expandedRowKeys = ref<number[]>([])
const metricsModal = ref<{ show: boolean; data: any }>({ show: false, data: null })
const editModal = ref<{ show: boolean; binding: Binding | null; modelId: number }>({ show: false, binding: null, modelId: 0 })

const statusColor: Record<string, string> = {
  healthy: '#52c41a',
  warning: '#faad14',
  unhealthy: '#ff4d4f',
  no_data: '#d9d9d9',
}

const routingStrategyOptions = [
  { label: '自动路由', value: 'auto' },
  { label: '随机使用', value: 'random' },
]
const autoModeOptions = [
  { label: '优先级', value: 'priority' },
  { label: '故障转移', value: 'failover' },
  { label: '健康感知', value: 'health' },
]

const providerOptions = computed(() => providers.value.map((p) => ({ label: p.name, value: p.id })))

// 时间轴渲染:6 格色块,tooltip 显示 bucket 序号与状态
function renderTimeline(timeline: string[]) {
  const cells = (timeline && timeline.length ? timeline : Array(6).fill('no_data'))
  return h('div', { class: 'timeline' }, cells.map((st: string, i: number) =>
    h('div', {
      key: i,
      class: 'timeline-cell',
      style: { background: statusColor[st] || statusColor.no_data },
      title: `Bucket ${i + 1}/6: ${st}`,
    })
  ))
}

function formatRate(rate: any): string {
  if (rate == null) return '-'
  const n = Number(rate)
  if (Number.isNaN(n)) return String(rate)
  // 后端返回 0-1 区间则按百分比展示,否则原样
  return n <= 1 ? `${(n * 100).toFixed(1)}%` : `${n}%`
}

function strategyLabel(m: RouteModel): string {
  const strategy = routingStrategyOptions.find((o) => o.value === m.routing_strategy)?.label || m.routing_strategy
  const mode = m.routing_strategy === 'auto'
    ? ` / ${autoModeOptions.find((o) => o.value === m.auto_mode)?.label || m.auto_mode}`
    : ''
  return strategy + mode
}

const modelColumns: DataTableColumns<RouteModel> = [
  { title: 'ID', key: 'model_id', width: 70 },
  { title: '模型名称', key: 'name' },
  {
    title: '路由策略',
    key: 'routing',
    render(row) {
      return strategyLabel(row)
    },
  },
  {
    title: '绑定数',
    key: 'bindings',
    width: 80,
    render(row) {
      return String((row.bindings || []).length)
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    render(row) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openBindingAdd(row) }, { default: () => '添加绑定' }),
        h(NButton, { size: 'small', onClick: () => openRoutingModal(row) }, { default: () => '路由策略' }),
      ])
    },
  },
]

const bindingColumns = computed<DataTableColumns<Binding>>(() => [
  { title: '供应商', key: 'provider_name' },
  { title: '上游模型', key: 'upstream_model' },
  { title: '优先级', key: 'priority', width: 80 },
  { title: '权重', key: 'weight', width: 80 },
  {
    title: '24h 时间轴',
    key: 'timeline',
    render(row) {
      return renderTimeline(row.timeline)
    },
  },
  {
    title: '平均延迟',
    key: 'avg_latency_ms',
    width: 100,
    render(row) {
      return row.avg_latency_ms > 0 ? `${row.avg_latency_ms} ms` : '-'
    },
  },
  {
    title: '启用',
    key: 'enabled',
    width: 70,
    render(row) {
      return h(NSwitch, { value: row.enabled, onUpdateValue: (v: boolean) => onToggleBinding(row, v) })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 240,
    render(row) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => loadMetrics(row.binding_id) }, { default: () => '详情' }),
        h(NButton, { size: 'small', onClick: () => openBindingEdit(row) }, { default: () => '编辑' }),
        h(NPopconfirm, { onPositiveClick: () => onBindingDelete(row) }, {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
          default: () => '确定删除该绑定?',
        }),
      ])
    },
  },
])

// 覆盖模板使用的列对象(含 expand):扩展行内渲染内嵌 binding 表格
const modelColumnsWithExpand = computed<DataTableColumns<RouteModel>>(() => [
  {
    type: 'expand',
    expandable: () => true,
    renderExpand: (row: RouteModel) =>
      h(NDataTable, {
        columns: bindingColumns.value,
        data: row.bindings || [],
        pagination: false,
        bordered: false,
        size: 'small',
      }),
  },
  ...modelColumns,
])

async function loadStatus() {
  loading.value = true
  try {
    const res = await http.get('/api/routing/status')
    models.value = (res.data?.models || []) as RouteModel[]
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  try {
    const res = await http.get('/api/providers')
    providers.value = (res.data || []) as Provider[]
  } catch (e) {
    message.error(errorMessage(e))
  }
}

async function loadMetrics(bindingId: number) {
  try {
    const res = await http.get(`/api/routing/bindings/${bindingId}/metrics`)
    metricsModal.value = { show: true, data: res.data }
  } catch (e) {
    message.error(errorMessage(e))
  }
}

// ---- binding CRUD ----
const bindingSaving = ref(false)
const bindingForm = reactive({
  provider_id: null as number | null,
  upstream_model: '',
  priority: 100,
  weight: 100,
  enabled: true,
})

function resetBindingForm() {
  bindingForm.provider_id = null
  bindingForm.upstream_model = ''
  bindingForm.priority = 100
  bindingForm.weight = 100
  bindingForm.enabled = true
}

function openBindingAdd(row: RouteModel) {
  editModal.value = { show: true, binding: null, modelId: row.model_id }
  resetBindingForm()
}

function openBindingEdit(b: Binding) {
  // 编辑时需找到该 binding 所属的 model_id
  const model = models.value.find((m) => m.bindings?.some((bb) => bb.binding_id === b.binding_id))
  editModal.value = { show: true, binding: b, modelId: model?.model_id || 0 }
  bindingForm.provider_id = b.provider_id
  bindingForm.upstream_model = b.upstream_model
  bindingForm.priority = b.priority
  bindingForm.weight = b.weight
  bindingForm.enabled = b.enabled
}

async function onBindingSave() {
  if (!editModal.value.modelId) {
    message.warning('缺少模型上下文')
    return
  }
  if (!bindingForm.provider_id || !bindingForm.upstream_model) {
    message.warning('请填写供应商和上游模型')
    return
  }
  bindingSaving.value = true
  try {
    const body = {
      provider_id: bindingForm.provider_id,
      upstream_model: bindingForm.upstream_model,
      priority: bindingForm.priority,
      weight: bindingForm.weight,
      enabled: bindingForm.enabled,
    }
    if (editModal.value.binding) {
      await http.put(`/api/models/${editModal.value.modelId}/bindings/${editModal.value.binding.binding_id}`, body)
    } else {
      await http.post(`/api/models/${editModal.value.modelId}/bindings`, body)
    }
    editModal.value.show = false
    message.success('已保存')
    await loadStatus()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    bindingSaving.value = false
  }
}

async function onToggleBinding(b: Binding, enabled: boolean) {
  const model = models.value.find((m) => m.bindings?.some((bb) => bb.binding_id === b.binding_id))
  if (!model) return
  try {
    await http.put(`/api/models/${model.model_id}/bindings/${b.binding_id}`, {
      provider_id: b.provider_id,
      upstream_model: b.upstream_model,
      priority: b.priority,
      weight: b.weight,
      enabled,
    })
    b.enabled = enabled
    message.success(enabled ? '已启用' : '已禁用')
  } catch (e) {
    message.error(errorMessage(e))
  }
}

async function onBindingDelete(b: Binding) {
  const model = models.value.find((m) => m.bindings?.some((bb) => bb.binding_id === b.binding_id))
  if (!model) return
  try {
    await http.delete(`/api/models/${model.model_id}/bindings/${b.binding_id}`)
    message.success('已删除')
    await loadStatus()
  } catch (e) {
    message.error(errorMessage(e))
  }
}

// ---- routing strategy ----
const routingShow = ref(false)
const routingSaving = ref(false)
const routingForm = reactive({ modelId: 0, routing_strategy: 'auto', auto_mode: 'priority' })

function openRoutingModal(row: RouteModel) {
  routingForm.modelId = row.model_id
  routingForm.routing_strategy = row.routing_strategy || 'auto'
  routingForm.auto_mode = row.auto_mode || 'priority'
  routingShow.value = true
}

async function onRoutingSave() {
  routingSaving.value = true
  try {
    await http.put(`/api/models/${routingForm.modelId}/routing`, {
      routing_strategy: routingForm.routing_strategy,
      auto_mode: routingForm.routing_strategy === 'auto' ? routingForm.auto_mode : '',
    })
    message.success('已保存')
    routingShow.value = false
    await loadStatus()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    routingSaving.value = false
  }
}

onMounted(() => {
  loadStatus()
  loadProviders()
})
</script>

<style scoped>
.toolbar {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.hint {
  color: #999;
  font-size: 13px;
}
.price-fields {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
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
.timeline {
  display: inline-flex;
  align-items: center;
}
.timeline-cell {
  width: 16px;
  height: 16px;
  display: inline-block;
  margin-right: 2px;
  border-radius: 2px;
}
.metrics-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
}
.metric {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.metric-label {
  color: #999;
  font-size: 12px;
}
.metric-value {
  font-size: 14px;
  font-weight: 500;
}
</style>
