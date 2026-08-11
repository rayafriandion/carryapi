<template>
  <div class="users">
    <n-card>
      <div class="toolbar">
        <n-button type="primary" @click="openCreate">新建用户</n-button>
      </div>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="false" :bordered="false" />
    </n-card>

    <!-- 新建 -->
    <n-modal v-model:show="createShow" preset="card" title="新建用户" style="width: 440px">
      <n-form>
        <n-form-item label="邮箱">
          <n-input v-model:value="createForm.email" placeholder="you@example.com" />
        </n-form-item>
        <n-form-item label="密码">
          <n-input v-model:value="createForm.password" type="password" show-password-on="click" placeholder="至少 8 位" />
        </n-form-item>
        <n-form-item label="角色">
          <n-select v-model:value="createForm.role" :options="roleOptions" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="createShow = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="onCreate">创建</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 编辑 -->
    <n-modal v-model:show="editShow" preset="card" title="编辑用户" style="width: 440px">
      <n-form>
        <n-form-item label="邮箱">
          <n-input :value="editForm.email" disabled />
        </n-form-item>
        <n-form-item label="角色">
          <n-select v-model:value="editForm.role" :options="roleOptions" />
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
import { NButton, NCard, NDataTable, NForm, NFormItem, NInput, NModal, NSelect, NPopconfirm, useMessage } from 'naive-ui'
import { http, errorMessage } from '../api/http'
import { useAuthStore } from '../stores/auth'

const message = useMessage()
const auth = useAuthStore()

const rows = ref<any[]>([])
const loading = ref(false)

const roleOptions = [
  { label: '用户', value: 'user' },
  { label: '管理员', value: 'admin' },
]
const statusOptions = [
  { label: '正常', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const columns = [
  { title: 'ID', key: 'id' },
  { title: '邮箱', key: 'email' },
  { title: '角色', key: 'role' },
  { title: '状态', key: 'status' },
  { title: '创建时间', key: 'created_at' },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openEdit(row) }, { default: () => '编辑' }),
        row.id === auth.user?.id
          ? h('span', { class: 'self-tag' }, '当前用户')
          : h(NPopconfirm, { onPositiveClick: () => onDelete(row) }, {
            trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
            default: () => '确定删除该用户?该操作不可恢复。',
          }),
      ])
    },
  },
]

async function load() {
  loading.value = true
  try {
    const res = await http.get('/api/users')
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
const createForm = reactive({ email: '', password: '', role: 'user' })

function openCreate() {
  createForm.email = ''
  createForm.password = ''
  createForm.role = 'user'
  createShow.value = true
}

async function onCreate() {
  if (!createForm.email || !createForm.password) {
    message.warning('请填写邮箱与密码')
    return
  }
  creating.value = true
  try {
    await http.post('/api/users', {
      email: createForm.email,
      password: createForm.password,
      role: createForm.role,
    })
    createShow.value = false
    message.success('已创建')
    load()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    creating.value = false
  }
}

// ---- edit ----
const editShow = ref(false)
const editing = ref(false)
const editForm = reactive({ id: 0, email: '', role: 'user', status: 'active' })

function openEdit(row: any) {
  editForm.id = row.id
  editForm.email = row.email
  editForm.role = row.role
  editForm.status = row.status || 'active'
  editShow.value = true
}

async function onEdit() {
  editing.value = true
  try {
    const body: any = { role: editForm.role, status: editForm.status }
    await http.put(`/api/users/${editForm.id}`, body)
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
    await http.delete(`/api/users/${row.id}`)
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
.row-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.self-tag {
  color: #999;
  font-size: 12px;
}
</style>
