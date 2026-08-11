<template>
  <div class="keys">
    <n-card>
      <div class="toolbar">
        <n-button type="primary" @click="openCreate">新建 API Key</n-button>
      </div>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="false" :bordered="false" />
    </n-card>

    <!-- 新建 -->
    <n-modal v-model:show="createShow" preset="card" title="新建 API Key" style="width: 420px">
      <n-form>
        <n-form-item label="标签">
          <n-input v-model:value="createForm.label" placeholder="例如:生产环境" @keydown.enter="onCreate" />
        </n-form-item>
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
    <n-modal v-model:show="editShow" preset="card" title="编辑 API Key" style="width: 420px">
      <n-form>
        <n-form-item label="标签">
          <n-input v-model:value="editForm.label" @keydown.enter="onEdit" />
        </n-form-item>
        <n-form-item label="状态">
          <n-select v-model:value="editForm.status" :options="statusOptions" />
        </n-form-item>
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
import { NButton, NCard, NDataTable, NForm, NFormItem, NInput, NModal, NSelect, NAlert, NPopconfirm, useMessage } from 'naive-ui'
import { http, errorMessage } from '../api/http'

const message = useMessage()
const rows = ref<any[]>([])
const loading = ref(false)

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const columns = [
  { title: 'Key 前缀', key: 'key_prefix' },
  { title: '标签', key: 'label' },
  { title: '状态', key: 'status' },
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
const createForm = reactive({ label: '' })
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
    const res = await http.post('/api/keys', { label: createForm.label })
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
const editForm = reactive({ id: 0, label: '', status: 'active' })

function openEdit(row: any) {
  editForm.id = row.id
  editForm.label = row.label
  editForm.status = row.status || 'active'
  editShow.value = true
}

async function onEdit() {
  editing.value = true
  try {
    await http.put(`/api/keys/${editForm.id}`, { label: editForm.label, status: editForm.status })
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

onMounted(load)
</script>

<style scoped>
.toolbar {
  margin-bottom: 16px;
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
