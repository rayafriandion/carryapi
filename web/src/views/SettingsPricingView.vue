<template>
  <div class="pricing">
    <n-card title="定价设置" class="section">
      <div class="current">
        当前统一币种: <n-tag>{{ currencyLabel(current) }}</n-tag>
        <span class="muted">所有模型价格、费用统计与配额均以此币种统一显示。</span>
      </div>

      <n-form label-placement="left" :label-width="110" class="form">
        <n-form-item label="常用预设">
          <n-select
            v-model:value="selectedPreset"
            :options="presetOptions"
            :disabled="useCustom || saving"
            filterable
            placeholder="选择常用法定货币"
            style="max-width: 320px"
          />
        </n-form-item>
        <n-form-item label="自定义代码">
          <n-switch v-model:value="useCustom" :disabled="saving" />
          <span class="muted" style="margin-left: 8px">开启后手动输入</span>
          <n-input
            v-if="useCustom"
            v-model:value="customCode"
            placeholder="例如 CZK / TRY / VND(3-8 位大写字母)"
            :disabled="saving"
            style="max-width: 260px; margin-left: 12px"
            @keydown.enter="onSave"
          />
        </n-form-item>
      </n-form>

      <n-alert type="warning" :show-icon="true" class="note">
        切换币种会把所有模型价格直接标注为新币种(数值不变)。请确认各模型的价格已按新币种填写,费用统计将随之统一。
      </n-alert>

      <div class="actions">
        <n-button type="primary" :loading="saving" :disabled="!effectiveCode" @click="onSave">保存</n-button>
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NCard, NForm, NFormItem, NSelect, NInput, NSwitch, NTag, NButton, NAlert, useMessage } from 'naive-ui'
import { http, errorMessage } from '../api/http'
import { currencyLabel, currencyPresets, refreshCurrency } from '../utils/currency'

const message = useMessage()
const current = ref('USD')
const saving = ref(false)

const selectedPreset = ref<string | null>(null)
const useCustom = ref(false)
const customCode = ref('')

const presetOptions = computed(() =>
  currencyPresets.map((x) => ({ label: x.name + ' (' + x.code + ' ' + x.symbol + ')', value: x.code })),
)

const effectiveCode = computed(() => {
  if (useCustom.value) {
    const c = customCode.value.trim().toUpperCase()
    return c || null
  }
  return selectedPreset.value
})

async function load() {
  try {
    const res = await http.get('/api/settings/pricing')
    const d = res.data || {}
    current.value = d.currency || 'USD'
    const preset = currencyPresets.find((x) => x.code === current.value.toUpperCase())
    if (preset) {
      selectedPreset.value = preset.code
      useCustom.value = false
    } else {
      useCustom.value = true
      customCode.value = current.value
    }
  } catch (e) {
    message.error(errorMessage(e))
  }
}

async function onSave() {
  const code = effectiveCode.value
  if (!code) {
    message.warning('请选择或输入币种代码')
    return
  }
  saving.value = true
  try {
    await http.put('/api/settings/pricing', { currency: code })
    current.value = code
    await refreshCurrency()
    message.success('已保存,所有模型价格已标注为新币种')
    await load()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.section { margin-bottom: 16px; }
.current { margin-bottom: 16px; display: flex; align-items: center; gap: 8px; }
.muted { color: #999; font-size: 12px; }
.form { margin-top: 8px; }
.note { margin-top: 16px; }
.actions { margin-top: 16px; }
</style>
