<template>
  <div class="stats">
    <n-tabs type="line" animated>
      <n-tab-pane name="summary" tab="汇总">
        <n-card title="按模型" class="section">
          <n-data-table :columns="modelColumns" :data="summary?.ByModel || []" size="small" />
        </n-card>
        <n-card title="按上游 Provider" class="section">
          <n-data-table :columns="providerColumns" :data="summary?.ByProvider || []" size="small" />
        </n-card>
        <n-card title="按 API Key" class="section">
          <n-data-table :columns="keyColumns" :data="summary?.ByKey || []" size="small" />
        </n-card>
      </n-tab-pane>

      <n-tab-pane name="trend" tab="趋势">
        <div class="toolbar">
          <n-radio-group v-model:value="granularity" @update:value="loadTrend">
            <n-radio-button value="day">按天</n-radio-button>
            <n-radio-button value="hour">按小时</n-radio-button>
          </n-radio-group>
        </div>
        <n-card>
          <div ref="trendEl" class="chart" />
        </n-card>
      </n-tab-pane>

      <n-tab-pane name="cost" tab="费用">
        <n-card>
          <template #header>
            <div class="card-header">
              <span>费用</span>
              <n-button size="small" @click="exportCost">导出 CSV</n-button>
            </div>
          </template>
          <n-data-table :columns="costColumns" :data="costRows" size="small" />
        </n-card>
      </n-tab-pane>

      <n-tab-pane name="success" tab="成功率">
        <n-card>
          <template #header>
            <div class="card-header">
              <span>成功率</span>
              <n-button size="small" @click="exportSuccess">导出 CSV</n-button>
            </div>
          </template>
          <n-data-table :columns="successColumns" :data="successRows" size="small" />
        </n-card>
      </n-tab-pane>
    </n-tabs>

    <n-modal v-model:show="drillVisible" preset="dialog" :title="`失败类型明细${drillTitle ? ' - ' + drillTitle : ''}`">
      <n-data-table :columns="drillColumns" :data="drillRows" size="small" />
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { h, onMounted, onUnmounted, ref } from 'vue'
import * as echarts from 'echarts'
import {
  NTabs, NTabPane, NCard, NDataTable, NRadioGroup, NRadioButton, NButton, NModal, useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'
import { formatMoney, loadCurrency } from '../utils/currency'
import { toCSV, downloadCSV } from '../utils/csv'

const message = useMessage()
const systemCurrency = ref('USD')

const summary = ref<any>(null)
const costRows = ref<any[]>([])
const successRows = ref<any[]>([])
const granularity = ref<'day' | 'hour'>('day')
const trendEl = ref<HTMLElement>()
let chart: echarts.ECharts | null = null

const costRender = (row: any) => formatMoney(row.Cost, systemCurrency.value)
const totalCostRender = (row: any) => formatMoney(row.TotalCost, systemCurrency.value)

const modelColumns = [
  { title: '模型', key: 'Model' },
  { title: '请求', key: 'Requests' },
  { title: '输入 Token', key: 'InputTokens' },
  { title: '输出 Token', key: 'OutputTokens' },
  { title: '费用', key: 'Cost', render: costRender },
]
const providerColumns = [
  { title: 'Provider', key: 'ProviderName' },
  { title: '请求', key: 'Requests' },
  { title: '费用', key: 'Cost', render: costRender },
]
const keyColumns = [
  { title: 'Key', key: 'KeyPrefix' },
  { title: 'Label', key: 'Label' },
  { title: '请求', key: 'Requests' },
  { title: '费用', key: 'Cost', render: costRender },
]
const costColumns = [
  { title: '分组', key: 'Group' },
  { title: '请求', key: 'Requests' },
  { title: '费用', key: 'TotalCost', render: totalCostRender },
]
const successColumns = [
  { title: '分组', key: 'Group' },
  { title: '总数', key: 'Total' },
  { title: '成功', key: 'Success' },
  {
    title: '失败',
    key: 'Failed',
    render: (row: any) =>
      h(
        NButton,
        { size: 'small', disabled: !row.Failed, onClick: () => openDrill(row) },
        { default: () => row.Failed ?? 0 }
      ),
  },
  { title: '成功率', key: 'SuccessRate' },
  { title: '平均耗时(ms)', key: 'AvgDurationMs' },
]

// —— 失败类型 drill-down ——
const drillVisible = ref(false)
const drillTitle = ref('')
const drillRows = ref<any[]>([])
const drillColumns = [
  { title: '错误类型', key: 'ErrorType' },
  { title: '数量', key: 'Count' },
]

async function openDrill(row: any) {
  drillVisible.value = true
  drillTitle.value = row.Group
  drillRows.value = []
  try {
    const res = await http.get('/api/logs', { params: { model: row.Group, page: 1, page_size: 200 } })
    const items = res.data?.items || []
    const counts: Record<string, number> = {}
    for (const it of items) {
      if (it.ErrorType && it.ErrorType !== 'none') {
        counts[it.ErrorType] = (counts[it.ErrorType] || 0) + 1
      }
    }
    drillRows.value = Object.entries(counts)
      .map(([ErrorType, Count]) => ({ ErrorType, Count }))
      .sort((a, b) => b.Count - a.Count)
  } catch (e) {
    message.error(errorMessage(e))
  }
}

// —— CSV 导出 ——
function exportCost() {
  const headers = costColumns.map((c: any) => ({ key: c.key, label: c.title }))
  const csv = toCSV(headers, costRows.value)
  downloadCSV(`费用_${new Date().toISOString().slice(0, 10)}.csv`, csv)
}
function exportSuccess() {
  const headers = successColumns.map((c: any) => ({ key: c.key, label: c.title }))
  const csv = toCSV(headers, successRows.value)
  downloadCSV(`成功率_${new Date().toISOString().slice(0, 10)}.csv`, csv)
}

// —— 趋势图(双轴:请求数/左轴 + 成功率/右轴) ——
function renderTrend(pts: any[]) {
  if (!trendEl.value) return
  if (!chart) chart = echarts.init(trendEl.value)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['请求数', '成功数', '成功率'] },
    xAxis: { type: 'category', data: pts.map((p) => p.Bucket) },
    yAxis: [
      { type: 'value', name: '请求数' },
      { type: 'value', name: '成功率', min: 0, max: 100, axisLabel: { formatter: '{value}%' } },
    ],
    series: [
      { name: '请求数', type: 'bar', yAxisIndex: 0, data: pts.map((p) => p.Requests ?? 0) },
      { name: '成功数', type: 'line', smooth: true, yAxisIndex: 0, data: pts.map((p) => p.SuccessCount ?? 0) },
      {
        name: '成功率',
        type: 'line',
        smooth: true,
        yAxisIndex: 1,
        data: pts.map((p) => (p.Requests ? +(((p.SuccessCount ?? 0) / p.Requests) * 100).toFixed(1) : 0)),
      },
    ],
  })
}

async function loadTrend() {
  try {
    const res = await http.get('/api/stats/trend', { params: { granularity: granularity.value } })
    renderTrend(res.data || [])
  } catch (e) {
    message.error(errorMessage(e))
  }
}

onMounted(async () => {
  loadCurrency().then((c) => { systemCurrency.value = c })
  try {
    const s = await http.get('/api/stats/summary')
    summary.value = s.data
  } catch (e) {
    message.error(errorMessage(e))
  }
  await loadTrend()
  try {
    const [c, sr] = await Promise.all([
      http.get('/api/stats/cost', { params: { group: 'model' } }),
      http.get('/api/stats/success-rate', { params: { group: 'model' } }),
    ])
    costRows.value = c.data || []
    successRows.value = sr.data || []
  } catch (e) {
    message.error(errorMessage(e))
  }
})

onUnmounted(() => {
  chart?.dispose()
  chart = null
})
</script>

<style scoped>
.section {
  margin-bottom: 16px;
}
.toolbar {
  margin-bottom: 12px;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.chart {
  height: 400px;
}
</style>
