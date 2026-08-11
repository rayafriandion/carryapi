import { describe, it, expect, vi } from 'vitest'
import { toCSV, downloadCSV } from './csv'

describe('toCSV', () => {
  const headers = [
    { key: 'name', label: '名称' },
    { key: 'count', label: '数量' },
  ]

  it('generates header row and data rows from object rows', () => {
    const csv = toCSV(headers, [{ name: 'a', count: 1 }, { name: 'b', count: 2 }])
    expect(csv).toBe('名称,数量\na,1\nb,2')
  })

  it('escapes commas, quotes and newlines in cell values', () => {
    const csv = toCSV([{ key: 'v' }], [{ v: 'a,b' }, { v: 'say "hi"' }, { v: 'line1\nline2' }])
    expect(csv).toBe('v\n"a,b"\n"say ""hi"""\n"line1\nline2"')
  })

  it('handles array rows in order', () => {
    const csv = toCSV(headers, [['x', 10], ['y', 20]])
    expect(csv).toBe('名称,数量\nx,10\ny,20')
  })

  it('treats null/undefined as empty cell', () => {
    const csv = toCSV([{ key: 'a' }, { key: 'b' }], [{ a: 1, b: null }])
    expect(csv).toBe('a,b\n1,')
  })

  it('returns only header row when no data', () => {
    const csv = toCSV(headers, [])
    expect(csv).toBe('名称,数量')
  })
})

describe('downloadCSV', () => {
  it('triggers a download with a BOM-prefixed csv blob', () => {
    const createObjectURL = vi.fn(() => 'blob:mock')
    const revokeObjectURL = vi.fn()
    const click = vi.fn()
    URL.createObjectURL = createObjectURL as any
    URL.revokeObjectURL = revokeObjectURL as any

    const a = { href: '', download: '', click } as any
    const appendChild = vi.fn()
    const removeChild = vi.fn()
    document.createElement = vi.fn(() => a) as any
    document.body.appendChild = appendChild as any
    document.body.removeChild = removeChild as any

    downloadCSV('test.csv', 'a,b\n1,2')

    expect(createObjectURL).toHaveBeenCalled()
    expect(appendChild).toHaveBeenCalledWith(a)
    expect(removeChild).toHaveBeenCalledWith(a)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:mock')
    expect(click).toHaveBeenCalled()
    expect(a.download).toBe('test.csv')
  })
})
