import type { AdminIconName } from './components'
import type { AdminSection } from './types'

export type AdminNavigationGroup = '运营中心' | '内容治理' | '平台管理'

export interface AdminNavigationItem {
  key: AdminSection
  label: string
  description: string
  icon: AdminIconName
  group: AdminNavigationGroup
}

export const adminNavigation: AdminNavigationItem[] = [
  { key: 'media', label: '媒体运营', description: '检查素材状态、引用关系与失败任务', icon: 'media', group: '运营中心' },
  { key: 'delivery', label: '配送运营', description: '处理配送订单、资金与异常记录', icon: 'delivery', group: '运营中心' },
  { key: 'moderation', label: '统一审核', description: '按状态处理内容审核与人工改判', icon: 'moderation', group: '内容治理' },
  { key: 'community', label: '帖子管理', description: '查看已提交帖子并执行合规下架', icon: 'community', group: '内容治理' },
  { key: 'trade', label: '商品管理', description: '查看商品状态并衔接审核或下架', icon: 'trade', group: '内容治理' },
  { key: 'reports', label: '举报审核', description: '查看用户举报及其审核状态', icon: 'reports', group: '内容治理' },
  { key: 'identities', label: '身份审核', description: '处理骑手与商家身份申请', icon: 'identities', group: '内容治理' },
  { key: 'dashboard', label: '数据统计', description: '查看平台总量、今日新增与待处理内容', icon: 'dashboard', group: '平台管理' },
  { key: 'users', label: '用户管理', description: '管理用户状态和管理员角色', icon: 'users', group: '平台管理' },
  { key: 'navigation', label: '导航管理', description: '维护普通导航入口与合作商家目录', icon: 'navigation', group: '平台管理' },
]

export const adminNavigationGroups: AdminNavigationGroup[] = ['运营中心', '内容治理', '平台管理']

