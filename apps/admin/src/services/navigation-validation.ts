import type { NavigationItemInput } from '../types'

// Immediate editor feedback. The backend remains authoritative for full path/permission checks.
export function navigationItemError(value: NavigationItemInput): string {
  if (!value.title.trim() || [...value.title.trim()].length > 64) return '入口名称请填写 1–64 个字。'
  if (!/^wx[a-zA-Z0-9]{16}$/.test(value.appId.trim()) || value.appId.trim() === 'wx0000000000000000') return '请填写真实小程序 AppID：wx 开头加 16 位字母或数字。'
  const path = value.path.trim()
  if (!path || [...path].length > 240 || path.startsWith('/') || /^[a-z][a-z\d+.-]*:/i.test(path) || /[\\#\u0000-\u001f]/.test(path)) return 'Path 应为小程序相对页面路径，不是网址；不要以 / 开头。'
  try {
    const decoded = decodeURIComponent(path.split('?')[0])
    if (decoded.startsWith('/') || decoded.includes('//') || decoded.split('/').includes('..')) return 'Path 包含不支持的路径，请向服务方确认。'
  } catch { return 'Path 的编码无效，请向服务方确认。' }
  if ([...value.subtitle].length > 120 || [...value.iconText].length > 12) return '用途说明最多 120 字，图标文字最多 12 字。'
  if (value.color && !/^#[0-9a-f]{6}$/i.test(value.color)) return '图标颜色请填写六位色值，如 #2E7DD4，或留空使用主题色。'
  if (!Number.isSafeInteger(value.sort)) return '排序请填写整数。'
  return ''
}
