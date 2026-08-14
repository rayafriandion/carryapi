<template>
  <div class="keys">
    <n-card>
      <div class="toolbar">
        <n-button type="primary" @click="openCreate">新建 API Key</n-button>
      </div>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="false" :bordered="false" />
    </n-card>

    <!-- 新建 -->
    <n-modal v-model:show="createShow" preset="card" title="新建 API Key" style="width: 460px">
      <n-form>
        <n-form-item label="标签">
          <n-input v-model:value="createForm.label" placeholder="例如:生产环境" @keydown.enter="onCreate" />
        </n-form-item>
        <n-form-item label="Token 上限">
          <n-input-number v-model:value="createForm.limit_tokens" :min="0" :step="1000" placeholder="留空表示不限制" clearable style="width: 100%" />
        </n-form-item>
        <n-form-item label="费用上限">
          <n-input-number v-model:value="createForm.limit_cost" :min="0" :step="1" placeholder="留空表示不限制" clearable style="width: 100%" />
        </n-form-item>
        <p class="hint">达到上限后使用该 Key 的请求返回 429。留空表示不限制。</p>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="createShow = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="onCreate">创建</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 明文一次性展示 -->
    <n-modal v-model:show="plainShow" preset="card" title="API Key 已创建" style="width: 520px">
      <n-alert type="warning" :show-icon="true">
        请立即复制保存。关闭后无法再次查看明文。
      </n-alert>
      <n-input :value="plainKey" readonly class="plain-input" />
      <template #footer>
        <div class="modal-footer">
          <n-button type="primary" @click="copyPlain">复制</n-button>
          <n-button @click="plainShow = false">我已保存</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 编辑 -->
    <n-modal v-model:show="editShow" preset="card" title="编辑 API Key" style="width: 460px">
      <n-form>
        <n-form-item label="标签">
          <n-input v-model:value="editForm.label" @keydown.enter="onEdit" />
        </n-form-item>
        <n-form-item label="状态">
          <n-select v-model:value="editForm.status" :options="statusOptions" />
        </n-form-item>
        <n-form-item label="Token 上限">
          <n-input-number v-model:value="editForm.limit_tokens" :min="0" :step="1000" placeholder="留空表示不限制" clearable style="width: 100%" />
        </n-form-item>
        <n-form-item label="费用上限">
          <n-input-number v-model:value="editForm.limit_cost" :min="0" :step="1" placeholder="留空表示不限制" clearable style="width: 100%" />
        </n-form-item>
        <p class="hint">两个字段均留空表示移除配额。</p>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="editShow = false">取消</n-button>
          <n-button type="primary" :loading="editing" @click="onEdit">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, h } from 'vue'
import { NButton, NCard, NDataTable, NForm, NFormItem, NInput, NInputNumber, NModal, NSelect, NAlert, NPopconfirm, useMessage } from 'naive-ui'
import { http, errorMessage } from '../api/http'
import { formatMoney, loadCurrency } from '../utils/currency'

const message = useMessage()
const rows = ref<any[]>([])
const loading = ref(false)
const systemCurrency = ref('USD')

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const columns = [
  { title: 'Key 前缀', key: 'key_prefix' },
  { title: '标签', key: 'label' },
  { title: '状态', key: 'status' },
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
  { title: '创建时间', key: 'created_at' },
  { title: '最后使用', key: 'last_used_at' },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openEdit(row) }, { default: () => '编辑' }),
        h(NPopconfirm, { onPositiveClick: () => onDelete(row) }, {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
          default: () => '确定删除该 Key?',
        }),
      ])
    },
  },
]

async function load() {
  loading.value = true
  try {
    const res = await http.get('/api/keys')
    rows.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}

// ---- create ----
const createShow = ref(false)
const creating = ref(false)
const createForm = reactive({ label: '', limit_tokens: null as number | null, limit_cost: null as number | null })
const plainShow = ref(false)
const plainKey = ref('')

function openCreate() {
  createForm.label = ''
  createShow.value = true
}

async function onCreate() {
  if (!createForm.label) {
    message.warning('请输入标签')
    return
  }
  creating.value = true
  try {
    const res = await http.post('/api/keys', {
      label: createForm.label,
      quota: {
        period: 'total',
        limit_tokens: createForm.limit_tokens ?? null,
        limit_cost: createForm.limit_cost ?? null,
      },
    })
    createShow.value = false
    plainKey.value = res.data.key || ''
    plainShow.value = true
    load()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    creating.value = false
  }
}

function copyPlain() {
  if (!plainKey.value) return
  navigator.clipboard.writeText(plainKey.value)
  message.success('已复制')
}

// ---- edit ----
const editShow = ref(false)
const editing = ref(false)
const editForm = reactive({ id: 0, label: '', status: 'active', limit_tokens: null as number | null, limit_cost: null as number | null })

function openEdit(row: any) {
  editForm.id = row.id
  editForm.label = row.label
  editForm.status = row.status || 'active'
  const q = row.quota
  editForm.limit_tokens = q && q.id ? (q.limit_tokens ?? null) : null
  editForm.limit_cost = q && q.id ? (q.limit_cost ?? null) : null
  editShow.value = true
}

async function onEdit() {
  editing.value = true
  try {
    await http.put(`/api/keys/${editForm.id}`, { label: editForm.label, status: editForm.status, quota: { period: 'total', limit_tokens: editForm.limit_tokens ?? null, limit_cost: editForm.limit_cost ?? null } })
    editShow.value = false
    message.success('已保存')
    load()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    editing.value = false
  }
}

// ---- delete ----
async function onDelete(row: any) {
  try {
    await http.delete(`/api/keys/${row.id}`)
    message.success('已删除')
    load()
  } catch (e) {
    message.error(errorMessage(e))
  }
}

onMounted(() => {
  loadCurrency().then((c) => { systemCurrency.value = c })
  load()
})
</script>

<style scoped>
.toolbar {
  margin-bottom: 16px;
}
.hint {
  color: #999;
  font-size: 12px;
  margin: 0 0 12px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.plain-input {
  margin-top: 12px;
}
.row-actions {
  display: flex;
  gap: 8px;
}
</style>
