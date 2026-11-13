import { AdminBadge, type AdminBadgeTone } from './AdminBadge'

const labels: Record<string, string> = {
  active: '正常', muted: '已禁言', banned: '已封禁', closed: '已注销',
  user: '普通用户', admin: '管理员', superadmin: '超级管理员',
  submitted: '待审核', reviewing: '审核中', approved: '已通过', rejected: '已拒绝',
  pending: '待处理', open: '营业中', resting: '休息中', enabled: '已启用', disabled: '已停用',
  in_progress: '处理中', resolved: '已解决', ignored: '不处理',
  published: '已发布', off_shelf: '已下架', sold: '已售出', draft: '草稿',
  selling: '在售', reserved: '已预订', taken_down: '已下架', removed: '已移除',
  locked: '已锁定',
  pending_fee_payment: '待付配送费', ready_for_claim: '等待接单', claimed: '已接单',
  to_pickup: '前往取货', picked_up: '已取货', delivering: '配送中', delivered: '已送达',
  cancelled: '已取消', payment_failed: '支付失败', settled: '已结算', paid_offline: '已登记结款',
  queued: '已排队', blocked: '已暂停', succeeded: '已完成', failed: '处理失败',
  retry_pending: '等待重试', refund_pending: '退款处理中', refunded: '已退款',
  pending_upload: '等待上传', uploaded: '已上传', validating: '正在检测', promoting: '正在保存',
  final_ready: '已就绪', review_failed: '检测异常', delete_pending: '等待清理',
  delete_deadletter: '清理失败', deleted: '已删除', unconsumed: '等待处理',
  processing: '处理中', retry_wait: '等待重试', deadletter: '多次失败',
  created: '已创建', authorization_issued: '等待上传授权', consumed: '已使用',
  expired: '已过期', not_required: '无需审核', review_pending: '待图片审核',
  paid: '已支付', declined: '已拒绝', unknown: '状态待确认',
  following: '已关注', not_following: '未关注',
}

const tones: Record<string, AdminBadgeTone> = {
  active: 'success', approved: 'success', open: 'success', enabled: 'success', published: 'success', selling: 'success',
  delivered: 'success', settled: 'success', paid_offline: 'success', succeeded: 'success',
  final_ready: 'success', following: 'success', paid: 'success', consumed: 'success',
  submitted: 'warning', reviewing: 'info', pending: 'warning', muted: 'warning', resting: 'warning',
  pending_fee_payment: 'warning', ready_for_claim: 'warning', queued: 'warning', retry_pending: 'warning',
  refund_pending: 'warning', pending_upload: 'warning', uploaded: 'info', validating: 'info',
  promoting: 'info', processing: 'info', retry_wait: 'warning', unknown: 'warning',
  claimed: 'info', to_pickup: 'info', picked_up: 'info', delivering: 'info',
  banned: 'danger', rejected: 'danger', closed: 'danger', disabled: 'neutral', off_shelf: 'neutral', sold: 'neutral', taken_down: 'neutral', removed: 'neutral', reserved: 'warning', locked: 'danger',
  cancelled: 'danger', payment_failed: 'danger', failed: 'danger', blocked: 'danger',
  review_failed: 'danger', delete_deadletter: 'danger', deadletter: 'danger', not_following: 'danger',
  superadmin: 'info', admin: 'info', user: 'neutral',
}

export function statusLabel(value: string) { return labels[value] || value || '未设置' }

export function AdminStatusBadge({ value }: { value: string }) {
  return <AdminBadge tone={tones[value] || 'neutral'}>{statusLabel(value)}</AdminBadge>
}
