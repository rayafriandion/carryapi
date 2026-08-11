// CSV 导出工具

// 将单个字段转义为 CSV 单元格(处理逗号、引号、换行)
function escapeCell(value: unknown): string {
  if (value === null || value === undefined) return ''
  const s = String(value)
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`
  }
  return s
}

// 根据表头与行数据生成 CSV 文本。
// rows 中每项可以是对象(按 headers 的 key 取值),也可以是数组(顺序对应 headers)。
export function toCSV(headers: { key: string; label?: string }[], rows: unknown[]): string {
  const head = headers.map((h) => escapeCell(h.label ?? h.key)).join(',')
  const body = rows
    .map((row: any) => {
      if (Array.isArray(row)) {
        return headers.map((_, i) => escapeCell(row[i])).join(',')
      }
      return headers.map((h) => escapeCell(row[h.key])).join(',')
    })
    .join('\n')
  return head + (body ? '\n' + body : '')
}

// 将 CSV 文本作为文件下载
export function downloadCSV(filename: string, csv: string): void {
  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
