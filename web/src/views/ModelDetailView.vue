<template>
  <div class="model-detail">
    <n-card>
      <div class="page-head">
        <n-button quaternary size="small" @click="goBack">← 返回</n-button>
        <div class="title">
          {{ model?.name || '加载中...' }}
          <n-tag v-if="model" size="small" :bordered="false" :type="model.enabled ? 'success' : 'default'" style="margin-left: 8px">
            {{ model.enabled ? '已启用' : '已禁用' }}
          </n-tag>
        </div>
        <n-button v-if="auth.isAdmin && model" size="small" @click="goEdit">编辑模型</n-button>
      </div>
    </n-card>

    <n-spin :show="loading">
      <div v-if="model" class="content">
        <n-card class="section" title="概览">
          <n-descriptions :column="3" label-placement="left" bordered size="small">
            <n-descriptions-item label="模型名称">{{ model.name }}</n-descriptions-item>
            <n-descriptions-item label="币种">{{ model.currency === 'CNY' ? '人民币 (￥)' : '美元 ($)' }}</n-descriptions-item>
            <n-descriptions-item label="路由策略">{{ routingStrategyLabel }}</n-descriptions-item>
            <n-descriptions-item label="输入价格">{{ formatPrice(model.input_price, model.currency) }}</n-descriptions-item>
            <n-descriptions-item label="输出价格">{{ formatPrice(model.output_price, model.currency) }}</n-descriptions-item>
            <n-descriptions-item label="缓存读价格">{{ formatPrice(model.cache_read_price, model.currency) }}</n-descriptions-item>
            <n-descriptions-item label="缓存写价格">{{ formatPrice(model.cache_write_price, model.currency) }}</n-descriptions-item>
            <n-descriptions-item label="创建时间">{{ formatTime(model.created_at) }}</n-descriptions-item>
            <n-descriptions-item label="绑定数量">{{ model.bindings.length }}</n-descriptions-item>
          </n-descriptions>
        </n-card>

        <n-card class="section" title="近 30 天统计">
          <n-grid :cols="4" responsive="screen" :x-gap="12">
            <n-gi>
              <div class="stat-box">
                <div class="stat-label">总请求数</div>
                <div class="stat-value">{{ model.total_requests }}</div>
              </div>
            </n-gi>
            <n-gi>
              <div class="stat-box">
                <div class="stat-label">成功数</div>
                <div class="stat-value">{{ model.success_count }}</div>
              </div>
            </n-gi>
            <n-gi>
              <div class="stat-box">
                <div class="stat-label">成功率</div>
                <div class="stat-value">
                  <n-tag size="small" :bordered="false" :type="successTagType(model.success_rate)">
                    {{ formatPercent(model.success_rate) }}
                  </n-tag>
                </div>
              </div>
            </n-gi>
            <n-gi>
              <div class="stat-box">
                <div class="stat-label">平均延迟</div>
                <div class="stat-value">{{ formatLatency(model.avg_duration_ms) }}</div>
              </div>
            </n-gi>
          </n-grid>
        </n-card>

        <n-card class="section" title="上游绑定">
          <n-data-table
            :columns="bindingColumns"
            :data="model.bindings"
            :pagination="false"
            :bordered="false"
            size="small"
            :row-key="(row: BindingDetail) => row.provider_id + ':' + row.upstream_model"
          />
        </n-card>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NCard, NButton, NSpin, NTag, NDescriptions, NDescriptionsItem,
  NDataTable, NGrid, NGi, useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'
import { useAuthStore } from '../stores/auth'

interface BindingDetail {
  provider_id: number
  provider_name: string
  provider_status: string
  protocol: string
  upstream_model: string
  priority: number
  weight: number
  enabled: boolean
  total_requests_24h: number
  success_rate_24h: number
  avg_latency_ms: number
  avg_ttft_ms: number
  timeline: string[]
  last_request_at: string | null
}

interface ModelDetail {
  id: number
  name: string
  enabled: boolean
  routing_strategy: string
  auto_mode: string
  created_at: string
  input_price: number | null
  output_price: number | null
  cache_read_price: number | null
  cache_write_price: number | null
  currency: string
  total_requests: number
  success_count: number
  success_rate: number
  avg_duration_ms: number
  bindings: BindingDetail[]
}

const route = useRoute()
const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const model = ref<ModelDetail | null>(null)
const loading = ref(false)

const modelId = computed(() => Number(route.params.id))

const routingStrategyLabel = computed(() => {
  if (!model.value) return ''
  if (model.value.routing_strategy === 'random') return '随机使用'
  const mode = { priority: '优先级', failover: '故障转移', health: '健康感知' }[model.value.auto_mode] || '优先级'
  return `自动路由 (${mode})`
})

function formatPrice(v: number | null, currency: string): string {
  if (v === null || v === undefined) return '—'
  const sym = currency === 'CNY' ? '￥' : '$'
  return `${sym}${v.toFixed(4)} / M tokens`
}
function formatPercent(v: number): string {
  if (!v) return '0%'
  return `${v.toFixed(1)}%`
}
function formatLatency(v: number): string {
  if (!v) return '—'
  return `${v.toFixed(0)} ms`
}
function formatTime(v: string): string {
  if (!v) return '—'
  try {
    return new Date(v).toLocaleString()
  } catch {
    return v
  }
}
function successTagType(rate: number): 'success' | 'warning' | 'error' | 'default' {
  if (rate >= 95) return 'success'
  if (rate >= 80) return 'warning'
  if (rate > 0) return 'error'
  return 'default'
}
function protocolLabel(p: string): string {
  return ({ openai_chat: 'OpenAI Chat', openai_responses: 'OpenAI Responses', anthropic: 'Anthropic' } as Record<string, string>)[p] || p
}
function timelineDot(status: string) {
  const color = { healthy: '#18a058', warning: '#f0a020', unhealthy: '#d03050', no_data: '#ccc' }[status] || '#ccc'
  return h('span', { style: { display: 'inline-block', width: 8, height: 8, borderRadius: '50%', background: color } })
}

const bindingColumns = [
  {
    title: '供应商',
    key: 'provider_name',
    render(row: BindingDetail) {
      return h('div', { style: 'display:flex;align-items:center;gap:6px' }, [
        h('span', { style: 'font-weight:600' }, row.provider_name),
        h(NTag, { size: 'tiny', bordered: false, type: row.enabled ? 'success' : 'default' }, { default: () => row.enabled ? '启用' : '禁用' }),
      ])
    },
  },
  { title: '协议', key: 'protocol', render: (r: BindingDetail) => protocolLabel(r.protocol) },
  { title: '上游模型', key: 'upstream_model' },
  { title: '优先级', key: 'priority', width: 80 },
  { title: '权重', key: 'weight', width: 70 },
  {
    title: '24h 成功率',
    key: 'success_rate_24h',
    width: 110,
    render(row: BindingDetail) {
      return h(NTag, { size: 'small', bordered: false, type: successTagType(row.success_rate_24h) }, { default: () => formatPercent(row.success_rate_24h) })
    },
  },
  {
    title: '24h 延迟',
    key: 'avg_latency_ms',
    width: 100,
    render: (r: BindingDetail) => formatLatency(r.avg_latency_ms),
  },
  {
    title: '24h 请求',
    key: 'total_requests_24h',
    width: 90,
  },
  {
    title: '健康趋势',
    key: 'timeline',
    width: 110,
    render(row: BindingDetail) {
      return h('div', { style: 'display:flex;gap:4px;align-items:center' },
        (row.timeline || []).map((s) => timelineDot(s)))
    },
  },
]

async function load() {
  loading.value = true
  try {
    const res = await http.get(`/api/catalog/${modelId.value}`)
    model.value = res.data
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push({ name: 'models-catalog' })
}
function goEdit() {
  router.push({ name: 'model-edit', params: { id: modelId.value } })
}

onMounted(load)
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
.content {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.stat-box {
  padding: 8px 4px;
}
.stat-label {
  color: #999;
  font-size: 12px;
  margin-bottom: 6px;
}
.stat-value {
  font-size: 20px;
  font-weight: 600;
}
</style>
