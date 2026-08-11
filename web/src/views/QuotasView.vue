<template>
  <div class="quotas">
    <n-alert type="info" :show-icon="true" class="note">
      当前后端 /api/quotas 仅返回登录用户自己的配额(用户范围列表)。此页面编辑的是当前管理员自身的配额限制。
    </n-alert>
    <n-card>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="false" :bordered="false" />
    </n-card>

    <n-modal v-model:show="editShow" preset="card" title="编辑配额" style="width: 460px">
      <n-form>
        <n-form-item label="范围">
          <n-input :value="editForm.scope" disabled />
        </n-form-item>
        <n-form-item label="周期">
          <n-input :value="editForm.period" disabled />
        </n-form-item>
        <n-form-item label="Token 上限">
          <n-input-number v-model:value="editForm.limit_tokens" placeholder="留空表示不限制" clearable style="width: 100%" />
        </n-form-item>
        <n-form-item label="费用上限">
          <n-input-number v-model:value="editForm.limit_cost" placeholder="留空表示不限制" clearable style="width: 100%" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="editShow = false">取消</n-button>
          <n-button type="primary" :loading="saving" @click="onSave">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, h } from 'vue'
import { NAlert, NButton, NCard, NDataTable, NForm, NFormItem, NInput, NInputNumber, NModal, NPopconfirm, useMessage } from 'naive-ui'
import { http, errorMessage } from '../api/http'

const message = useMessage()
const rows = ref<any[]>([])
const loading = ref(false)

const columns = [
  { title: 'ID', key: 'ID' },
  { title: '范围', key: 'Scope' },
  { title: '范围 ID', key: 'ScopeID' },
  { title: '周期', key: 'Period' },
  {
    title: 'Token 上限',
    key: 'LimitTokens',
    render(row: any) {
      return row.LimitTokens ?? '-'
    },
  },
  {
    title: '费用上限',
    key: 'LimitCost',
    render(row: any) {
      return row.LimitCost ?? '-'
    },
  },
  { title: '已用 Token', key: 'UsedTokens' },
  { title: '已用费用', key: 'UsedCost' },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h(NButton, { size: 'small', onClick: () => openEdit(row) }, { default: () => '编辑' })
    },
  },
]

async function load() {
  loading.value = true
  try {
    const res = await http.get('/api/quotas')
    rows.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}

const editShow = ref(false)
const saving = ref(false)
const editForm = reactive({ id: 0, scope: '', period: '', limit_tokens: null as number | null, limit_cost: null as number | null })

function openEdit(row: any) {
  editForm.id = row.ID
  editForm.scope = row.Scope
  editForm.period = row.Period
  editForm.limit_tokens = row.LimitTokens ?? null
  editForm.limit_cost = row.LimitCost ?? null
  editShow.value = true
}

async function onSave() {
  saving.value = true
  try {
    const body: any = {}
    if (editForm.limit_tokens != null) body.limit_tokens = editForm.limit_tokens
    if (editForm.limit_cost != null) body.limit_cost = editForm.limit_cost
    await http.put(`/api/quotas/${editForm.id}`, body)
    editShow.value = false
    message.success('已保存')
    load()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.note {
  margin-bottom: 16px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
