import { AdminBadge, type AdminBadgeTone } from './AdminBadge'

const labels: Record<string, string> = {
  active: '正常', muted: '已禁言', banned: '已封禁', closed: '已注销',
  user: '普通用户', admin: '管理员', superadmin: '超级管理员',
  submitted: '待审核', reviewing: '审核中', approved: '已通过', rejected: '已拒绝',
  pending: '待处理', open: '营业中', resting: '休息中', enabled: '已启用', disabled: '已停用',
  published: '已发布', off_shelf: '已下架', sold: '已售出', draft: '草稿',
}

const tones: Record<string, AdminBadgeTone> = {
  active: 'success', approved: 'success', open: 'success', enabled: 'success', published: 'success',
  submitted: 'warning', reviewing: 'info', pending: 'warning', muted: 'warning', resting: 'warning',
  banned: 'danger', rejected: 'danger', closed: 'danger', disabled: 'neutral', off_shelf: 'neutral', sold: 'neutral',
  superadmin: 'info', admin: 'info', user: 'neutral',
}

export function statusLabel(value: string) { return labels[value] || value || '未设置' }

export function AdminStatusBadge({ value }: { value: string }) {
  return <AdminBadge tone={tones[value] || 'neutral'}>{statusLabel(value)}</AdminBadge>
}
