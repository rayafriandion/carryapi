<template>
  <div class="models">
    <n-tabs type="line" animated>
      <!-- 供应商 -->
      <n-tab-pane name="providers" tab="供应商">
        <n-card>
          <div class="toolbar">
            <n-button type="primary" @click="openProviderCreate">新建供应商</n-button>
          </div>
          <n-data-table :columns="providerColumns" :data="providers" :loading="providersLoading" :pagination="false" :bordered="false" />
        </n-card>
      </n-tab-pane>

      <!-- 模型 -->
      <n-tab-pane name="models" tab="模型">
        <n-card>
          <div class="toolbar">
            <n-button type="primary" @click="openModelCreate">新建模型</n-button>
          </div>
          <n-data-table :columns="modelColumns" :data="models" :loading="modelsLoading" :pagination="false" :bordered="false" />
        </n-card>
      </n-tab-pane>

      <!-- 定价 -->
      <n-tab-pane name="pricing" tab="定价">
        <n-card>
          <div class="toolbar">
            <n-select
              v-model:value="priceModelId"
              :options="modelOptions"
              placeholder="选择模型"
              clearable
              style="width: 280px"
              @update:value="onSelectModel"
            />
          </div>
          <template v-if="priceModelId">
            <n-descriptions label-placement="left" bordered :column="1" class="section">
              <n-descriptions-item label="模型">{{ priceModelName }}</n-descriptions-item>
              <n-descriptions-item label="输入价格 (每百万 token)">{{ priceForm.input_price }}</n-descriptions-item>
              <n-descriptions-item label="输出价格 (每百万 token)">{{ priceForm.output_price }}</n-descriptions-item>
              <n-descriptions-item label="缓存读 (每百万 token)">{{ cacheRead ?? '-' }}</n-descriptions-item>
              <n-descriptions-item label="缓存写 (每百万 token)">{{ cacheWrite ?? '-' }}</n-descriptions-item>
              <n-descriptions-item label="生效时间">{{ priceEffectiveFrom || '-' }}</n-descriptions-item>
            </n-descriptions>
            <div class="edit-section">
              <p>修改价格:</p>
              <div class="price-fields">
                <n-input-number v-model:value="priceForm.input_price" placeholder="输入价格" />
                <n-input-number v-model:value="priceForm.output_price" placeholder="输出价格" />
                <n-input-number v-model:value="cacheRead" placeholder="缓存读(可选)" clearable />
                <n-input-number v-model:value="cacheWrite" placeholder="缓存写(可选)" clearable />
              </div>
              <n-button type="primary" :loading="savingPrice" @click="onSavePrice">保存价格</n-button>
            </div>
          </template>
          <n-empty v-else description="请选择要配置价格的模型" />
        </n-card>
      </n-tab-pane>
    </n-tabs>

    <!-- 供应商新建/编辑 -->
    <n-modal v-model:show="providerShow" preset="card" :title="providerEditing ? '编辑供应商' : '新建供应商'" style="width: 460px">
      <n-form>
        <n-form-item label="名称">
          <n-input v-model:value="providerForm.name" placeholder="例如:OpenAI" />
        </n-form-item>
        <n-form-item label="Base URL">
          <n-input v-model:value="providerForm.base_url" placeholder="https://api.openai.com/v1" />
        </n-form-item>
        <n-form-item label="API Key">
          <n-input v-model:value="providerForm.api_key" type="password" show-password-on="click" :placeholder="providerEditing ? '留空则不修改' : '上游 API Key'" />
        </n-form-item>
        <n-form-item label="协议">
          <n-select v-model:value="providerForm.protocol" :options="protocolOptions" />
        </n-form-item>
        <n-form-item v-if="providerEditing" label="状态">
          <n-select v-model:value="providerForm.status" :options="activeStatusOptions" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="providerShow = false">取消</n-button>
          <n-button type="primary" :loading="providerSaving" @click="onProviderSave">保存</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 模型新建/编辑 -->
    <n-modal v-model:show="modelShow" preset="card" :title="modelEditing ? '编辑模型' : '新建模型'" style="width: 460px">
      <n-form>
        <n-form-item label="名称">
          <n-input v-model:value="modelForm.name" placeholder="对外模型名,例如 gpt-4o" />
        </n-form-item>
        <n-form-item label="供应商">
          <n-select v-model:value="modelForm.provider_id" :options="providerOptions" />
        </n-form-item>
        <n-form-item label="上游模型">
          <n-input v-model:value="modelForm.upstream_model" placeholder="上游实际模型名" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="modelShow = false">取消</n-button>
          <n-button type="primary" :loading="modelSaving" @click="onModelSave">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, h } from 'vue'
import {
  NButton, NCard, NDataTable, NTabs, NTabPane, NForm, NFormItem, NInput,
  NInputNumber, NModal, NSelect, NDescriptions, NDescriptionsItem, NEmpty,
  NSwitch, NPopconfirm, useMessage,
} from 'naive-ui'
import { http, errorMessage } from '../api/http'

const message = useMessage()

const protocolOptions = [
  { label: 'OpenAI Chat', value: 'openai_chat' },
  { label: 'OpenAI Responses', value: 'openai_responses' },
  { label: 'Anthropic', value: 'anthropic' },
]
const activeStatusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

// ---- providers ----
const providers = ref<any[]>([])
const providersLoading = ref(false)
const providerColumns = [
  { title: 'ID', key: 'id' },
  { title: '名称', key: 'name' },
  { title: 'Base URL', key: 'base_url' },
  { title: '协议', key: 'protocol' },
  { title: '状态', key: 'status' },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openProviderEdit(row) }, { default: () => '编辑' }),
        h(NPopconfirm, { onPositiveClick: () => onProviderDelete(row) }, {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
          default: () => '确定删除该供应商?',
        }),
      ])
    },
  },
]

const providerOptions = computed(() => providers.value.map((p) => ({ label: p.name, value: p.id })))

async function loadProviders() {
  providersLoading.value = true
  try {
    const res = await http.get('/api/providers')
    providers.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    providersLoading.value = false
  }
}

const providerShow = ref(false)
const providerEditing = ref(false)
const providerSaving = ref(false)
const providerForm = reactive({ id: 0, name: '', base_url: '', api_key: '', protocol: 'openai_chat', status: 'active' })

function openProviderCreate() {
  providerEditing.value = false
  providerForm.id = 0
  providerForm.name = ''
  providerForm.base_url = ''
  providerForm.api_key = ''
  providerForm.protocol = 'openai_chat'
  providerForm.status = 'active'
  providerShow.value = true
}
function openProviderEdit(row: any) {
  providerEditing.value = true
  providerForm.id = row.id
  providerForm.name = row.name
  providerForm.base_url = row.base_url
  providerForm.api_key = ''
  providerForm.protocol = row.protocol
  providerForm.status = row.status || 'active'
  providerShow.value = true
}
async function onProviderSave() {
  if (!providerForm.name || !providerForm.base_url) {
    message.warning('请填写名称与 Base URL')
    return
  }
  providerSaving.value = true
  try {
    if (providerEditing.value) {
      const body: any = {
        name: providerForm.name,
        base_url: providerForm.base_url,
        protocol: providerForm.protocol,
        status: providerForm.status,
      }
      if (providerForm.api_key) body.api_key = providerForm.api_key
      await http.put(`/api/providers/${providerForm.id}`, body)
    } else {
      await http.post('/api/providers', {
        name: providerForm.name,
        base_url: providerForm.base_url,
        api_key: providerForm.api_key,
        protocol: providerForm.protocol,
      })
    }
    providerShow.value = false
    message.success('已保存')
    loadProviders()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    providerSaving.value = false
  }
}
async function onProviderDelete(row: any) {
  try {
    await http.delete(`/api/providers/${row.id}`)
    message.success('已删除')
    loadProviders()
  } catch (e) {
    message.error(errorMessage(e))
  }
}

// ---- models ----
const models = ref<any[]>([])
const modelsLoading = ref(false)
const modelColumns = [
  { title: 'ID', key: 'id' },
  { title: '名称', key: 'name' },
  { title: '供应商 ID', key: 'provider_id' },
  { title: '上游模型', key: 'upstream_model' },
  {
    title: '启用',
    key: 'enabled',
    render(row: any) {
      return h(NSwitch, { value: row.enabled, onUpdateValue: (v: boolean) => onToggleModel(row, v) })
    },
  },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openModelEdit(row) }, { default: () => '编辑' }),
        h(NPopconfirm, { onPositiveClick: () => onModelDelete(row) }, {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
          default: () => '确定删除该模型?',
        }),
      ])
    },
  },
]

async function loadModels() {
  modelsLoading.value = true
  try {
    const res = await http.get('/api/models')
    models.value = res.data || []
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    modelsLoading.value = false
  }
}

const modelShow = ref(false)
const modelEditing = ref(false)
const modelSaving = ref(false)
const modelForm = reactive({ id: 0, name: '', provider_id: null as number | null, upstream_model: '' })

function openModelCreate() {
  modelEditing.value = false
  modelForm.id = 0
  modelForm.name = ''
  modelForm.provider_id = null
  modelForm.upstream_model = ''
  modelShow.value = true
}
function openModelEdit(row: any) {
  modelEditing.value = true
  modelForm.id = row.id
  modelForm.name = row.name
  modelForm.provider_id = row.provider_id
  modelForm.upstream_model = row.upstream_model
  modelShow.value = true
}
async function onModelSave() {
  if (!modelForm.name || !modelForm.provider_id || !modelForm.upstream_model) {
    message.warning('请填写完整信息')
    return
  }
  modelSaving.value = true
  try {
    const body = {
      name: modelForm.name,
      provider_id: modelForm.provider_id,
      upstream_model: modelForm.upstream_model,
    }
    if (modelEditing.value) {
      await http.put(`/api/models/${modelForm.id}`, body)
    } else {
      await http.post('/api/models', body)
    }
    modelShow.value = false
    message.success('已保存')
    loadModels()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    modelSaving.value = false
  }
}
async function onToggleModel(row: any, enabled: boolean) {
  try {
    await http.put(`/api/models/${row.id}`, {
      name: row.name,
      provider_id: row.provider_id,
      upstream_model: row.upstream_model,
      enabled,
    })
    row.enabled = enabled
    message.success(enabled ? '已启用' : '已禁用')
  } catch (e) {
    message.error(errorMessage(e))
  }
}
async function onModelDelete(row: any) {
  try {
    await http.delete(`/api/models/${row.id}`)
    message.success('已删除')
    loadModels()
  } catch (e) {
    message.error(errorMessage(e))
  }
}

// ---- pricing ----
const priceModelId = ref<number | null>(null)
const priceModelName = computed(() => models.value.find((m) => m.id === priceModelId.value)?.name || '')
const modelOptions = computed(() => models.value.map((m) => ({ label: `${m.name} (${m.upstream_model})`, value: m.id })))
const priceForm = reactive({ input_price: 0, output_price: 0 })
const priceEffectiveFrom = ref('')
const cacheRead = ref<number | null>(null)
const cacheWrite = ref<number | null>(null)
const savingPrice = ref(false)

async function onSelectModel(id: number | null) {
  priceModelId.value = id
  if (id == null) return
  priceForm.input_price = 0
  priceForm.output_price = 0
  cacheRead.value = null
  cacheWrite.value = null
  priceEffectiveFrom.value = ''
  try {
    const res = await http.get(`/api/models/${id}/price`)
    const price = res.data?.price
    if (price) {
      priceForm.input_price = price.input_price ?? 0
      priceForm.output_price = price.output_price ?? 0
      cacheRead.value = price.cache_read_price ?? null
      cacheWrite.value = price.cache_write_price ?? null
      priceEffectiveFrom.value = price.effective_from || ''
    }
  } catch (e) {
    message.error(errorMessage(e))
  }
}

async function onSavePrice() {
  if (!priceModelId.value) return
  savingPrice.value = true
  try {
    const body: any = {
      input_price: priceForm.input_price ?? 0,
      output_price: priceForm.output_price ?? 0,
    }
    if (cacheRead.value != null) body.cache_read_price = cacheRead.value
    if (cacheWrite.value != null) body.cache_write_price = cacheWrite.value
    await http.put(`/api/models/${priceModelId.value}/price`, body)
    message.success('价格已保存')
    onSelectModel(priceModelId.value)
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    savingPrice.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadProviders(), loadModels()])
})
</script>

<style scoped>
.toolbar {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.section {
  margin-bottom: 16px;
}
.edit-section {
  margin-top: 16px;
}
.price-fields {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.row-actions {
  display: flex;
  gap: 8px;
}
</style>
