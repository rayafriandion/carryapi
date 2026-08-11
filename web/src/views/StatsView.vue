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
          <n-data-table :columns="costColumns" :data="costRows" size="small" />
        </n-card>
      </n-tab-pane>

      <n-tab-pane name="success" tab="成功率">
        <n-card>
          <n-data-table :columns="successColumns" :data="successRows" size="small" />
        </n-card>
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import * as echarts from 'echarts'
import { NTabs, NTabPane, NCard, NDataTable, NRadioGroup, NRadioButton } from 'naive-ui'
import { http } from '../api/http'

const summary = ref<any>(null)
const costRows = ref<any[]>([])
const successRows = ref<any[]>([])
const granularity = ref<'day' | 'hour'>('day')
const trendEl = ref<HTMLElement>()
let chart: echarts.ECharts | null = null

const modelColumns = [
  { title: '模型', key: 'Model' },
  { title: '请求', key: 'Requests' },
  { title: '输入 Token', key: 'InputTokens' },
  { title: '输出 Token', key: 'OutputTokens' },
  { title: '费用', key: 'Cost' },
]
const providerColumns = [
  { title: 'Provider', key: 'ProviderName' },
  { title: '请求', key: 'Requests' },
  { title: '费用', key: 'Cost' },
]
const keyColumns = [
  { title: 'Key', key: 'KeyPrefix' },
  { title: 'Label', key: 'Label' },
  { title: '请求', key: 'Requests' },
  { title: '费用', key: 'Cost' },
]
const costColumns = [
  { title: '分组', key: 'Group' },
  { title: '请求', key: 'Requests' },
  { title: '费用', key: 'TotalCost' },
]
const successColumns = [
  { title: '分组', key: 'Group' },
  { title: '总数', key: 'Total' },
  { title: '成功', key: 'Success' },
  { title: '失败', key: 'Failed' },
  { title: '成功率', key: 'SuccessRate' },
  { title: '平均耗时(ms)', key: 'AvgDurationMs' },
]

function renderTrend(pts: any[]) {
  if (!trendEl.value) return
  if (!chart) chart = echarts.init(trendEl.value)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['请求数', '成功数', '输入 Token', '输出 Token'] },
    xAxis: { type: 'category', data: pts.map((p) => p.Bucket) },
    yAxis: { type: 'value' },
    series: [
      { name: '请求数', type: 'line', smooth: true, data: pts.map((p) => p.Requests) },
      { name: '成功数', type: 'line', smooth: true, data: pts.map((p) => p.SuccessCount) },
      { name: '输入 Token', type: 'line', smooth: true, data: pts.map((p) => p.InputTok) },
      { name: '输出 Token', type: 'line', smooth: true, data: pts.map((p) => p.OutputTok) },
    ],
  })
}

async function loadTrend() {
  try {
    const res = await http.get('/api/stats/trend', { params: { granularity: granularity.value } })
    renderTrend(res.data || [])
  } catch {
    // 静默
  }
}

onMounted(async () => {
  try {
    const s = await http.get('/api/stats/summary')
    summary.value = s.data
  } catch { /* 静默 */ }
  await loadTrend()
  try {
    const [c, sr] = await Promise.all([
      http.get('/api/stats/cost', { params: { group: 'model' } }),
      http.get('/api/stats/success-rate', { params: { group: 'model' } }),
    ])
    costRows.value = c.data || []
    successRows.value = sr.data || []
  } catch { /* 静默 */ }
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
.chart {
  height: 400px;
}
</style>
