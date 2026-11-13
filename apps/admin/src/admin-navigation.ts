import type { AdminIconName } from './components'
import type { AdminSection } from './types'

export type AdminNavigationGroup = '工作台' | '业务运营' | '内容与用户' | '平台设置'

export interface AdminNavigationItem {
  key: AdminSection
  label: string
  description: string
  icon: AdminIconName
  group: AdminNavigationGroup
}

export const adminNavigation: AdminNavigationItem[] = [
  { key: 'dashboard', label: '经营总览', description: '查看营收、订单、用户与待办事项', icon: 'dashboard', group: '工作台' },
  { key: 'delivery', label: '配送与结算', description: '管理配送订单、骑手报酬、提现与退款', icon: 'delivery', group: '业务运营' },
  { key: 'notifications', label: '消息通知', description: '检查订单提醒、服务号绑定与值班群送达情况', icon: 'notifications', group: '业务运营' },
  { key: 'media', label: '素材中心', description: '检查图片视频、审核状态与异常处理记录', icon: 'media', group: '业务运营' },
  { key: 'moderation', label: '审核中心', description: '处理待审内容并记录人工改判依据', icon: 'moderation', group: '内容与用户' },
  { key: 'community', label: '社区内容', description: '查看帖子状态并对公开内容执行下架', icon: 'community', group: '内容与用户' },
  { key: 'trade', label: '二手商品', description: '查看商品状态并衔接审核或下架', icon: 'trade', group: '内容与用户' },
  { key: 'reports', label: '举报处理', description: '查看用户举报与最终处置进度', icon: 'reports', group: '内容与用户' },
  { key: 'identities', label: '商家骑手认证', description: '审核商家与骑手身份材料', icon: 'identities', group: '内容与用户' },
  { key: 'feedbacks', label: '意见反馈', description: '查看用户提交的问题与建议', icon: 'reports', group: '内容与用户' },
  { key: 'users', label: '用户与权限', description: '管理账号状态与后台管理员角色', icon: 'users', group: '平台设置' },
  { key: 'navigation', label: '服务入口', description: '维护校园服务导航与外部商家入口', icon: 'navigation', group: '平台设置' },
  { key: 'share-settings', label: '分享设置', description: '维护小程序转发标题、封面与内容卡片', icon: 'settings', group: '平台设置' },
]

export const featurePolicyNavigationItem: AdminNavigationItem = {
  key: 'feature-policy',
  label: '功能开关',
  description: '控制小程序模块、互动能力与新业务开放状态',
  icon: 'settings',
  group: '平台设置'
}

export const adminNavigationGroups: AdminNavigationGroup[] = ['工作台', '业务运营', '内容与用户', '平台设置']
