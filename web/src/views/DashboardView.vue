<template>
  <div class="dashboard">
    <n-card class="section base-url-card">
      <div class="base-url">
        <span class="label">API Base URL</span>
        <n-text code>{{ baseUrl || '—' }}</n-text>
        <n-button size="small" tertiary round type="primary" :disabled="!baseUrl" @click="copyBaseUrl">
          复制
        </n-button>
      </div>
    </n-card>

    <n-grid :cols="6" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
      <n-grid-item span="6 s:3 m:2">
        <n-card><n-statistic label="总请求" :value="summary?.TotalRequests ?? 0" /></n-card>
      </n-grid-item>
      <n-grid-item span="6 s:3 m:2">
        <n-card><n-statistic label="成功" :value="summary?.SuccessCount ?? 0" /></n-card>
      </n-grid-item>
      <n-grid-item span="6 s:3 m:2">
        <n-card><n-statistic label="失败" :value="summary?.ErrorCount ?? 0" /></n-card>
      </n-grid-item>
      <n-grid-item span="6 s:3 m:2">
        <n-card><n-statistic label="Token 输入/输出" :value="`${summary?.TotalInputTok ?? 0} / ${summary?.TotalOutputTok ?? 0}`" /></n-card>
      </n-grid-item>
      <n-grid-item span="6 s:3 m:2">
        <n-card><n-statistic label="费用" :value="(summary?.TotalCost ?? 0).toFixed(4)" /></n-card>
      </n-grid-item>
      <n-grid-item span="6 s:3 m:2">
        <n-card><n-statistic label="平均耗时 (ms)" :value="(summary?.AvgDurationMs ?? 0).toFixed(2)" /></n-card>
      </n-grid-item>
    </n-grid>

    <n-card title="最近请求趋势" class="section">
      <div ref="chartEl" class="chart" />
    </n-card>

    <n-card title="最近日志" class="section">
      <n-data-table :columns="columns" :data="logs" :pagination="false" :bordered="false" size="small" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import * as echarts from 'echarts'
import { NGrid, NGridItem, NCard, NStatistic, NDataTable, NText, NButton, useMessage } from 'naive-ui'

const message = useMessage()

import { http, errorMessage } from '../api/http'

const summary = ref<any>(null)
const logs = ref<any[]>([])
const chartEl = ref<HTMLElement>()
const baseUrl = ref('')
let chart: echarts.ECharts | null = null

async function copyBaseUrl() {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(baseUrl.value)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = baseUrl.value
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    message.success('已复制')
  } catch {
    message.error('复制失败')
  }
}

const columns = [
  { title: '时间', key: 'CreatedAt' },
  { title: 'RequestID', key: 'RequestID' },
  { title: '模型', key: 'CustomModel' },
  { title: '状态码', key: 'StatusCode' },
  { title: '错误类型', key: 'ErrorType' },
  { title: '耗时(ms)', key: 'DurationMs' },
]

function renderChart(pts: any[]) {
  if (!chartEl.value) return
  if (!chart) chart = echarts.init(chartEl.value)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['请求数', '成功数'] },
    xAxis: { type: 'category', data: pts.map((p) => p.Bucket) },
    yAxis: { type: 'value' },
    series: [
      { name: '请求数', type: 'line', smooth: true, data: pts.map((p) => p.Requests) },
      { name: '成功数', type: 'line', smooth: true, data: pts.map((p) => p.SuccessCount) },
    ],
  })
}

onMounted(async () => {
  try {
    const info = await http.get('/api/gateway/info')
    baseUrl.value = info.data?.base_url || ''
    const [s, t, l] = await Promise.all([
      http.get('/api/stats/summary'),
      http.get('/api/stats/trend', { params: { granularity: 'day' } }),
      http.get('/api/logs', { params: { page: 1, page_size: 10 } }),
    ])
    summary.value = s.data
    renderChart(t.data || [])
    logs.value = l.data.items || []
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
  margin-top: 16px;
}
.base-url-card {
  margin-bottom: 16px;
}
.base-url {
  display: flex;
  align-items: center;
  gap: 12px;
}
.base-url .label {
  font-weight: 600;
}
.chart {
  height: 300px;
}
</style>
