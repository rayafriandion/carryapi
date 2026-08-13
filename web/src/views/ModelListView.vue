<template>
  <div class="model-list">
    <n-card>
      <div class="toolbar">
        <div class="title">模型列表</div>
        <n-radio-group v-model:value="viewMode" size="small">
          <n-radio-button value="card">卡片视图</n-radio-button>
          <n-radio-button value="table">列表视图</n-radio-button>
        </n-radio-group>
      </div>
    </n-card>

    <n-spin :show="loading">
      <div v-if="viewMode === 'card'" class="card-grid">
        <n-card
          v-for="m in models"
          :key="m.id"
          class="model-card"
          hoverable
          role="button"
          tabindex="0"
          @click="openDetail(m.id)"
          @keyup.enter="openDetail(m.id)"
        >
          <div class="model-head">
            <n-text strong>{{ m.name }}</n-text>
            <n-space :size="6" align="center">
              <n-tag size="small" :bordered="false" type="info">{{ m.currency === 'CNY' ? '￥ CNY' : '$ USD' }}</n-tag>
              <n-tag size="small" :type="successTagType(m.success_rate)">{{ formatPercent(m.success_rate) }}</n-tag>
            </n-space>
          </div>
          <div class="bindings">
            <div class="bindings-title">
              上游绑定 <n-tag size="tiny" :bordered="false">{{ (m.bindings || []).length }}</n-tag>
            </div>
            <div class="binding-tags">
              <n-tag
                v-for="(b, i) in (m.bindings || [])"
                :key="i"
                size="small"
                :bordered="false"
                :type="b.enabled ? 'default' : 'warning'"
              >
                {{ b.provider_name }} / {{ b.upstream_model }}
              </n-tag>
              <span v-if="!(m.bindings || []).length" class="muted">—</span>
            </div>
          </div>
          <div class="price-grid">
            <div><span class="label">输入</span><span class="value">{{ formatPrice(m.input_price, m.currency) }}</span></div>
            <div><span class="label">输出</span><span class="value">{{ formatPrice(m.output_price, m.currency) }}</span></div>
            <div><span class="label">缓存读取</span><span class="value">{{ formatPrice(m.cache_read_price, m.currency) }}</span></div>
            <div><span class="label">平均延迟</span><span class="value">{{ formatLatency(m.avg_duration_ms) }}</span></div>
          </div>
          <div class="model-foot">
            <span>近 30 天请求: {{ m.total_requests }}</span>
            <span>成功: {{ m.success_count }}</span>
          </div>
        </n-card>
      </div>

      <n-card v-else class="section">
        <n-data-table
          :columns="columns"
          :data="models"
          :pagination="false"
          :bordered="false"
          size="small"
          :row-props="(row: CatalogModel) => ({ style: 'cursor: pointer', onClick: () => openDetail(row.id) })"
        />
      </n-card>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard, NRadioGroup, NRadioButton, NSpin, NTag, NText, NDataTable, NSpace,
  useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'

interface CatalogBinding {
  provider_id: number
  provider_name: string
  protocol: string
  upstream_model: string
  priority: number
  weight: number
  enabled: boolean
}

interface CatalogModel {
  id: number
  name: string
  upstream_model: string
  provider_name: string
  protocol: string
  bindings: CatalogBinding[]
  input_price: number | null
  output_price: number | null
  cache_read_price: number | null
  currency: string
  total_requests: number
  success_count: number
  success_rate: number
  avg_duration_ms: number
}

const router = useRouter()
const message = useMessage()
const models = ref<CatalogModel[]>([])
const loading = ref(false)
const viewMode = ref<'card' | 'table'>('card')

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
function successTagType(rate: number): 'success' | 'warning' | 'error' | 'default' {
  if (rate >= 95) return 'success'
  if (rate >= 80) return 'warning'
  if (rate > 0) return 'error'
  return 'default'
}

function openDetail(id: number) {
  router.push({ name: 'model-catalog-detail', params: { id } })
}

function renderBindingTags(row: CatalogModel) {
  const bindings = row.bindings || []
  if (!bindings.length) return '—'
  return h('div', { style: 'display:flex;flex-wrap:wrap;gap:4px' },
    bindings.map((b) => h(NTag, {
      size: 'small',
      bordered: false,
      type: b.enabled ? 'default' : 'warning',
    }, { default: () => b.enabled ? `${b.provider_name} / ${b.upstream_model}` : `${b.provider_name} / ${b.upstream_model} (停用)` })))
}

const columns = [
  { title: '模型名称', key: 'name' },
  { title: '上游绑定', key: 'bindings', render: renderBindingTags },
  {
    title: '币种',
    key: 'currency',
    render: (r: CatalogModel) => (r.currency === 'CNY' ? '￥ CNY' : '$ USD'),
  },
  {
    title: '输入价格',
    key: 'input_price',
    render: (r: CatalogModel) => formatPrice(r.input_price, r.currency),
  },
  {
    title: '输出价格',
    key: 'output_price',
    render: (r: CatalogModel) => formatPrice(r.output_price, r.currency),
  },
  {
    title: '缓存读取价格',
    key: 'cache_read_price',
    render: (r: CatalogModel) => formatPrice(r.cache_read_price, r.currency),
  },
  {
    title: '成功率',
    key: 'success_rate',
    render: (r: CatalogModel) => h(
      NTag,
      { size: 'small', type: successTagType(r.success_rate) },
      { default: () => formatPercent(r.success_rate) },
    ),
  },
  {
    title: '平均延迟',
    key: 'avg_duration_ms',
    render: (r: CatalogModel) => formatLatency(r.avg_duration_ms),
  },
  { title: '请求数(30天)', key: 'total_requests' },
  { title: '成功数(30天)', key: 'success_count' },
]

onMounted(async () => {
  loading.value = true
  try {
    const res = await http.get('/api/catalog')
    models.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.title {
  font-size: 16px;
  font-weight: 600;
}
.card-grid {
  margin-top: 16px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}
.model-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  cursor: pointer;
  outline: none;
}
.model-card:focus-visible {
  box-shadow: 0 0 0 2px var(--n-primary-color, #2080f0);
}
.model-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.bindings {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.bindings-title {
  color: #666;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.binding-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.muted {
  color: #bbb;
  font-size: 12px;
}
.price-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.price-grid > div {
  display: flex;
  flex-direction: column;
}
.price-grid .label {
  color: #999;
  font-size: 12px;
}
.price-grid .value {
  font-weight: 600;
}
.model-foot {
  display: flex;
  justify-content: space-between;
  color: #666;
  font-size: 12px;
}
.section {
  margin-top: 16px;
}
</style>
