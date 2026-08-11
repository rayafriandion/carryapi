<template>
  <div class="logs">
    <n-card class="filter-card">
      <n-grid :cols="6" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
        <n-grid-item span="6 s:3 m:1">
          <n-input v-model:value="filters.model" placeholder="模型" clearable @keydown.enter="load(1)" />
        </n-grid-item>
        <n-grid-item span="6 s:3 m:1">
          <n-input v-model:value="filters.error_type" placeholder="错误类型" clearable @keydown.enter="load(1)" />
        </n-grid-item>
        <n-grid-item span="6 s:3 m:1">
          <n-input v-model:value="filters.request_id" placeholder="RequestID" clearable @keydown.enter="load(1)" />
        </n-grid-item>
        <n-grid-item span="6 s:3 m:1">
          <n-input v-model:value="filters.status" placeholder="状态码" clearable @keydown.enter="load(1)" />
        </n-grid-item>
        <n-grid-item span="6 s:3 m:1">
          <n-input v-model:value="filters.start" placeholder="开始(RFC3339)" clearable @keydown.enter="load(1)" />
        </n-grid-item>
        <n-grid-item span="6 s:3 m:1">
          <n-input v-model:value="filters.end" placeholder="结束(RFC3339)" clearable @keydown.enter="load(1)" />
        </n-grid-item>
      </n-grid>
      <div class="filter-actions">
        <n-button type="primary" @click="load(1)">查询</n-button>
        <n-button @click="reset">重置</n-button>
      </div>
    </n-card>

    <n-card>
      <n-data-table remote :columns="columns" :data="rows" :pagination="pagination" :loading="loading" @update:page="load" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { NCard, NGrid, NGridItem, NInput, NButton, NDataTable } from 'naive-ui'
import { http } from '../api/http'

const filters = reactive({
  model: '', error_type: '', request_id: '', status: '', start: '', end: '',
})
const rows = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  showSizePicker: false,
  prefix: (info: any) => `共 ${info.itemCount} 条`,
})

const columns = [
  { title: '时间', key: 'CreatedAt' },
  { title: 'RequestID', key: 'RequestID' },
  { title: '用户', key: 'Email' },
  { title: '模型', key: 'CustomModel' },
  { title: '上游模型', key: 'UpstreamModel' },
  { title: '协议入', key: 'ProtocolIn' },
  { title: '协议出', key: 'ProtocolOut' },
  { title: '输入 Token', key: 'InputTokens' },
  { title: '输出 Token', key: 'OutputTokens' },
  { title: '费用', key: 'Cost' },
  { title: '耗时(ms)', key: 'DurationMs' },
  { title: '状态码', key: 'StatusCode' },
  { title: '错误类型', key: 'ErrorType' },
]

async function load(p: number) {
  loading.value = true
  const params: any = { page: p, page_size: pageSize.value }
  if (filters.model) params.model = filters.model
  if (filters.error_type) params.error_type = filters.error_type
  if (filters.request_id) params.request_id = filters.request_id
  if (filters.status) params.status = filters.status
  if (filters.start) params.start = filters.start
  if (filters.end) params.end = filters.end
  try {
    const res = await http.get('/api/logs', { params })
    rows.value = res.data.items || []
    total.value = res.data.total || 0
    page.value = p
    pagination.page = p
    pagination.itemCount = total.value
  } catch {
    // 静默
  } finally {
    loading.value = false
  }
}

function reset() {
  filters.model = ''
  filters.error_type = ''
  filters.request_id = ''
  filters.status = ''
  filters.start = ''
  filters.end = ''
  load(1)
}

onMounted(() => load(1))
</script>

<style scoped>
.filter-card {
  margin-bottom: 16px;
}
.filter-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
}
</style>
