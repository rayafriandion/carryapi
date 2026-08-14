import { http } from '../api/http'

export interface CurrencyPreset {
  code: string
  name: string
  symbol: string
}

export const currencyPresets: CurrencyPreset[] = [
  { code: 'USD', name: '美元', symbol: '$' },
  { code: 'CNY', name: '人民币', symbol: '¥' },
  { code: 'EUR', name: '欧元', symbol: '€' },
  { code: 'GBP', name: '英镑', symbol: '£' },
  { code: 'JPY', name: '日元', symbol: '¥' },
  { code: 'HKD', name: '港元', symbol: 'HK$' },
  { code: 'AUD', name: '澳元', symbol: 'A$' },
  { code: 'CAD', name: '加元', symbol: 'C$' },
  { code: 'SGD', name: '新加坡元', symbol: 'S$' },
  { code: 'KRW', name: '韩元', symbol: '₩' },
  { code: 'TWD', name: '新台币', symbol: 'NT$' },
  { code: 'CHF', name: '瑞士法郎', symbol: 'CHF' },
]

export function currencySymbol(code: string | null | undefined): string {
  if (!code) return '$'
  const c = code.toUpperCase()
  const preset = currencyPresets.find((x) => x.code === c)
  return preset ? preset.symbol : c
}

export function currencyName(code: string | null | undefined): string {
  if (!code) return '美元'
  const c = code.toUpperCase()
  const preset = currencyPresets.find((x) => x.code === c)
  return preset ? preset.name : c
}

export function currencyLabel(code: string | null | undefined): string {
  const c = (code || '').toUpperCase()
  if (!c) return '美元 (USD)'
  const preset = currencyPresets.find((x) => x.code === c)
  return preset ? preset.name + ' (' + preset.code + ')' : c
}

export function formatMoney(v: number | null | undefined, code: string | null | undefined): string {
  if (v === null || v === undefined) return '—'
  return currencySymbol(code) + Number(v).toFixed(4)
}

export function formatPricePerM(v: number | null | undefined, code: string | null | undefined): string {
  if (v === null || v === undefined) return '—'
  return currencySymbol(code) + Number(v).toFixed(4) + ' / M tokens'
}

let cachedCurrency: string | null = null

// loadCurrency 拉取系统统一币种并缓存;供各页面费用/价格展示使用。
export async function loadCurrency(): Promise<string> {
  if (cachedCurrency) return cachedCurrency
  try {
    const res = await http.get('/api/settings/pricing')
    cachedCurrency = (res.data?.currency as string) || 'USD'
  } catch {
    cachedCurrency = 'USD'
  }
  return cachedCurrency
}

// refreshCurrency 强制重新拉取(币种变更后调用)。
export async function refreshCurrency(): Promise<string> {
  cachedCurrency = null
  return loadCurrency()
}
