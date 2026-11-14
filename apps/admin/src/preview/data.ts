export type PreviewSection =
  | 'dashboard'
  | 'launch-process'
  | 'delivery'
  | 'notifications'
  | 'sites'
  | 'forms'
  | 'takers'
  | 'promoters'
  | 'inventory'
  | 'marketing'
  | 'chat'
  | 'ads'
  | 'media'
  | 'moderation'
  | 'community'
  | 'trade'
  | 'reports'
  | 'miniapp'
  | 'service-account'
  | 'withdrawals'
  | 'identities'
  | 'feedbacks'
  | 'users'
  | 'navigation'
  | 'share-settings'
  | 'feature-policy'
  | 'general-settings'
  | 'takeout'

export type Tone = 'success' | 'warning' | 'danger' | 'info' | 'neutral' | 'accent'

export interface DeliveryOrder {
  id: string
  customer: string
  rider: string
  pickup: string
  dropoff: string
  amountCents: number
  platformFeeCents: number
  status: '待接单' | '配送中' | '待确认' | '已完成' | '已取消'
  createdAt: string
  note: string
}

export interface ReviewItem {
  id: string
  kind: '帖子' | '商品' | '商家' | '骑手' | '举报'
  title: string
  submitter: string
  risk: '低风险' | '需关注' | '高风险'
  status: '待审核' | '审核中' | '已通过' | '已拒绝'
  source: '人工提交' | '自动检测'
  createdAt: string
  reason?: string
}

export interface CommunityItem {
  id: string
  title: string
  author: string
  topic: string
  status: '已公开' | '待审核' | '已下架'
  createdAt: string
}

export interface ProductItem {
  id: string
  title: string
  seller: string
  priceCents: number
  category: string
  status: '出售中' | '已预留' | '已下架'
  createdAt: string
}

export interface ReportItem {
  id: string
  target: string
  reason: string
  reporter: string
  status: '待处理' | '处理中' | '已解决'
  createdAt: string
}

export interface MediaItem {
  id: string
  name: string
  kind: '图片' | '视频'
  purpose: string
  status: '已通过' | '待审核' | '异常'
  size: string
  createdAt: string
}

export interface IdentityItem {
  id: string
  applicant: string
  role: '商家' | '骑手'
  business: string
  status: '待认证' | '已通过' | '已拒绝'
  submittedAt: string
  reason?: string
}

export interface FeedbackItem {
  id: string
  user: string
  category: '功能异常' | '产品建议' | '内容问题' | '账号问题'
  content: string
  status: '待处理' | '处理中' | '已解决' | '不处理'
  note?: string
  createdAt: string
}

export interface UserItem {
  id: string
  name: string
  phone: string
  role: '普通用户' | '商家' | '骑手' | '管理员'
  status: '正常' | '禁言' | '停用'
  campus: string
  createdAt: string
}

export interface ServiceEntry {
  id: string
  title: string
  subtitle: string
  path: string
  color: string
  enabled: boolean
  sort: number
}

export interface MessageTemplateItem {
  id: string
  name: string
  type: '新订单' | '接单通知' | '配送完成' | '审核结果' | '提现结果'
  content: string
  enabled: boolean
  updatedAt: string
}

export interface WebhookItem {
  id: string
  name: string
  type: '企业微信' | '飞书'
  endpoint: string
  enabled: boolean
  lastTest: '未测试' | '测试成功' | '测试失败'
  updatedAt: string
}

export interface NotificationDelivery {
  id: string
  templateName: string
  channel: '小程序通知' | '微信订阅' | '服务号' | '值班群'
  recipient: string
  status: '已送达' | '发送中' | '失败' | '已跳过'
  attempts: number
  error?: string
  createdAt: string
}

export interface PreviewSettings {
  share: {
    title: string
    description: string
    coverName: string
  }
  features: {
    delivery: boolean
    merchant: boolean
    rider: boolean
    notifications: boolean
    takeout: boolean
    community: boolean
  }
}

export interface LegacyTakeoutMerchant {
  id: string
  name: string
  category: string
  ordersToday: number
  revenueCents: number
  status: '营业中' | '休息中'
}

export const templateVariables = [
  { key: '{orderNo}', label: '订单号' },
  { key: '{customerName}', label: '用户称呼' },
  { key: '{pickup}', label: '取件地点' },
  { key: '{dropoff}', label: '送达地点' },
  { key: '{amount}', label: '订单金额' },
  { key: '{deliveryTime}', label: '预计送达' },
  { key: '{riderName}', label: '骑手称呼' }
]

export const seedOrders: DeliveryOrder[] = [
  {
    id: 'CC20261114001',
    customer: '林同学',
    rider: '周师傅',
    pickup: '北区快递站',
    dropoff: '东区 7 号宿舍',
    amountCents: 680,
    platformFeeCents: 120,
    status: '配送中',
    createdAt: '2026-11-14T09:18:00+08:00',
    note: '快递两件，电话联系后放置门口。'
  },
  {
    id: 'CC20261114002',
    customer: '陈同学',
    rider: '待分配',
    pickup: '西区外卖柜',
    dropoff: '图书馆南门',
    amountCents: 520,
    platformFeeCents: 100,
    status: '待接单',
    createdAt: '2026-11-14T09:42:00+08:00',
    note: '午餐配送，请注意保温。'
  },
  {
    id: 'CC20261114003',
    customer: '吴同学',
    rider: '黄师傅',
    pickup: '南区驿站',
    dropoff: '实训楼 A 座',
    amountCents: 760,
    platformFeeCents: 140,
    status: '待确认',
    createdAt: '2026-11-14T08:21:00+08:00',
    note: '文件袋，需本人签收。'
  },
  {
    id: 'CC20261114004',
    customer: '赵同学',
    rider: '李师傅',
    pickup: '北门便利店',
    dropoff: '北区 3 号宿舍',
    amountCents: 450,
    platformFeeCents: 80,
    status: '已完成',
    createdAt: '2026-11-14T07:55:00+08:00',
    note: '已完成，用户五星好评。'
  },
  {
    id: 'CC20261113018',
    customer: '许同学',
    rider: '周师傅',
    pickup: '东区快递站',
    dropoff: '行政楼',
    amountCents: 580,
    platformFeeCents: 100,
    status: '已完成',
    createdAt: '2026-11-13T17:12:00+08:00',
    note: '已完成。'
  },
  {
    id: 'CC20261113011',
    customer: '孙同学',
    rider: '待分配',
    pickup: '南区食堂',
    dropoff: '体育馆',
    amountCents: 600,
    platformFeeCents: 110,
    status: '已取消',
    createdAt: '2026-11-13T11:06:00+08:00',
    note: '用户主动取消。'
  }
]

export const seedReviews: ReviewItem[] = [
  { id: 'RV-301', kind: '帖子', title: '校园吉他社周末招新', submitter: '林同学', risk: '低风险', status: '待审核', source: '人工提交', createdAt: '2026-11-14T09:36:00+08:00' },
  { id: 'RV-302', kind: '商品', title: '九成新机械键盘', submitter: '陈同学', risk: '需关注', status: '待审核', source: '自动检测', createdAt: '2026-11-14T09:18:00+08:00' },
  { id: 'RV-303', kind: '商家', title: '北区轻食铺入驻资料', submitter: '黄先生', risk: '需关注', status: '审核中', source: '人工提交', createdAt: '2026-11-14T08:52:00+08:00' },
  { id: 'RV-304', kind: '骑手', title: '骑手身份认证', submitter: '周师傅', risk: '低风险', status: '已通过', source: '人工提交', createdAt: '2026-11-13T16:40:00+08:00' },
  { id: 'RV-305', kind: '举报', title: '商品描述与实物不符', submitter: '匿名用户', risk: '高风险', status: '待审核', source: '自动检测', createdAt: '2026-11-13T14:06:00+08:00' }
]

export const seedCommunity: CommunityItem[] = [
  { id: 'PO-101', title: '期末复习资料交换帖', author: '林同学', topic: '学习互助', status: '已公开', createdAt: '2026-11-14T08:42:00+08:00' },
  { id: 'PO-102', title: '周末羽毛球搭子招募', author: '陈同学', topic: '校园活动', status: '待审核', createdAt: '2026-11-14T08:12:00+08:00' },
  { id: 'PO-103', title: '图书馆失物招领', author: '吴同学', topic: '失物招领', status: '已公开', createdAt: '2026-11-13T19:35:00+08:00' },
  { id: 'PO-104', title: '校外兼职信息合集', author: '匿名用户', topic: '兼职信息', status: '已下架', createdAt: '2026-11-13T16:22:00+08:00' }
]

export const seedProducts: ProductItem[] = [
  { id: 'PD-220', title: '九成新机械键盘', seller: '陈同学', priceCents: 16800, category: '数码', status: '出售中', createdAt: '2026-11-14T08:48:00+08:00' },
  { id: 'PD-221', title: '考研数学全套资料', seller: '林同学', priceCents: 8600, category: '书籍', status: '已预留', createdAt: '2026-11-14T08:10:00+08:00' },
  { id: 'PD-222', title: '折叠露营椅两把', seller: '黄同学', priceCents: 9200, category: '生活用品', status: '出售中', createdAt: '2026-11-13T20:12:00+08:00' },
  { id: 'PD-223', title: '旧版课程资料合集', seller: '匿名用户', priceCents: 2000, category: '书籍', status: '已下架', createdAt: '2026-11-13T14:00:00+08:00' }
]

export const seedReports: ReportItem[] = [
  { id: 'RP-61', target: '商品 PD-220', reason: '描述与实物不符', reporter: '匿名用户', status: '待处理', createdAt: '2026-11-14T09:02:00+08:00' },
  { id: 'RP-62', target: '帖子 PO-104', reason: '疑似未经核实兼职', reporter: '林同学', status: '处理中', createdAt: '2026-11-13T17:30:00+08:00' },
  { id: 'RP-63', target: '用户 U-108', reason: '私信中包含不友善内容', reporter: '吴同学', status: '已解决', createdAt: '2026-11-13T10:18:00+08:00' }
]

export const seedMedia: MediaItem[] = [
  { id: 'AS-901', name: '社团招新封面.jpg', kind: '图片', purpose: '帖子封面', status: '已通过', size: '1.8 MB', createdAt: '2026-11-14T08:34:00+08:00' },
  { id: 'AS-902', name: '键盘实拍 1.jpg', kind: '图片', purpose: '商品图片', status: '待审核', size: '2.4 MB', createdAt: '2026-11-14T08:20:00+08:00' },
  { id: 'AS-903', name: '商家后厨巡检.mp4', kind: '视频', purpose: '商家认证', status: '异常', size: '18.2 MB', createdAt: '2026-11-13T16:05:00+08:00' },
  { id: 'AS-904', name: '校园地图插画.png', kind: '图片', purpose: '导航素材', status: '已通过', size: '820 KB', createdAt: '2026-11-13T11:40:00+08:00' }
]

export const seedIdentities: IdentityItem[] = [
  { id: 'ID-501', applicant: '北区轻食铺', role: '商家', business: '食品经营 · 北区商业街', status: '待认证', submittedAt: '2026-11-14T08:50:00+08:00' },
  { id: 'ID-502', applicant: '周师傅', role: '骑手', business: '电动车 · 身份证已上传', status: '待认证', submittedAt: '2026-11-14T08:38:00+08:00' },
  { id: 'ID-503', applicant: '小满文具店', role: '商家', business: '零售 · 东区生活广场', status: '已通过', submittedAt: '2026-11-13T15:18:00+08:00' }
]

export const seedFeedbacks: FeedbackItem[] = [
  { id: 'FB-81', user: '林同学', category: '功能异常', content: '商品图片上传后偶尔不显示，刷新后恢复。', status: '处理中', createdAt: '2026-11-14T09:12:00+08:00' },
  { id: 'FB-82', user: '陈同学', category: '产品建议', content: '希望通知中心可以按订单类型筛选。', status: '待处理', createdAt: '2026-11-14T08:55:00+08:00' },
  { id: 'FB-83', user: '吴同学', category: '账号问题', content: '更换手机号后旧手机号仍显示在资料页。', status: '已解决', note: '已刷新用户资料缓存并完成验证。', createdAt: '2026-11-13T18:20:00+08:00' }
]

export const seedUsers: UserItem[] = [
  { id: 'U-101', name: '林同学', phone: '138****4612', role: '普通用户', status: '正常', campus: '湛科生活圈', createdAt: '2026-10-18' },
  { id: 'U-102', name: '陈同学', phone: '155****0218', role: '普通用户', status: '禁言', campus: '湛科生活圈', createdAt: '2026-10-19' },
  { id: 'U-103', name: '北区轻食铺', phone: '186****1077', role: '商家', status: '正常', campus: '湛科生活圈', createdAt: '2026-10-03' },
  { id: 'U-104', name: '周师傅', phone: '139****2631', role: '骑手', status: '正常', campus: '湛科生活圈', createdAt: '2026-09-28' },
  { id: 'U-105', name: '运营管理员', phone: '178****0021', role: '管理员', status: '正常', campus: '总部', createdAt: '2026-08-11' }
]

export const seedServiceEntries: ServiceEntry[] = [
  { id: 'NV-1', title: '取快递', subtitle: '校园快递代取', path: 'pages/express/index', color: '#6f45a8', enabled: true, sort: 10 },
  { id: 'NV-2', title: '打印资料', subtitle: '上传文件后到店取件', path: 'pages/print/index', color: '#3e8f74', enabled: true, sort: 20 },
  { id: 'NV-3', title: '校园跑腿', subtitle: '发布同校互助任务', path: 'pages/errand/index', color: '#bf7a32', enabled: true, sort: 30 },
  { id: 'NV-4', title: '失物招领', subtitle: '查看最新失物信息', path: 'pages/lost-found/index', color: '#ad5d7a', enabled: false, sort: 40 }
]

export const seedTemplates: MessageTemplateItem[] = [
  {
    id: 'TP-1',
    name: '新订单值班提醒',
    type: '新订单',
    content: '收到新订单 {orderNo}\n取件：{pickup}\n送达：{dropoff}\n请尽快确认并安排骑手。',
    enabled: true,
    updatedAt: '2026-11-14T08:20:00+08:00'
  },
  {
    id: 'TP-2',
    name: '骑手接单通知',
    type: '接单通知',
    content: '订单 {orderNo} 已由 {riderName} 接单，预计在 {deliveryTime} 前送达。',
    enabled: true,
    updatedAt: '2026-11-13T17:42:00+08:00'
  },
  {
    id: 'TP-3',
    name: '审核结果通知',
    type: '审核结果',
    content: '您提交的资料已完成审核，请打开小程序查看结果。',
    enabled: true,
    updatedAt: '2026-11-13T14:03:00+08:00'
  },
  {
    id: 'TP-4',
    name: '配送完成回访',
    type: '配送完成',
    content: '订单 {orderNo} 已完成，感谢使用校园配送服务。',
    enabled: false,
    updatedAt: '2026-11-12T11:16:00+08:00'
  }
]

export const seedWebhooks: WebhookItem[] = [
  { id: 'WH-1', name: '商家值班群', type: '企业微信', endpoint: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=preview-merchant', enabled: true, lastTest: '测试成功', updatedAt: '2026-11-14T09:00:00+08:00' },
  { id: 'WH-2', name: '骑手调度群', type: '企业微信', endpoint: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=preview-rider', enabled: true, lastTest: '未测试', updatedAt: '2026-11-13T18:12:00+08:00' },
  { id: 'WH-3', name: '产品运营通知', type: '飞书', endpoint: 'https://open.feishu.cn/open-apis/bot/v2/hook/preview-ops', enabled: false, lastTest: '测试失败', updatedAt: '2026-11-12T10:22:00+08:00' }
]

export const seedDeliveries: NotificationDelivery[] = [
  { id: 'ND-9001', templateName: '骑手接单通知', channel: '小程序通知', recipient: '林同学', status: '已送达', attempts: 1, createdAt: '2026-11-14T09:31:00+08:00' },
  { id: 'ND-9002', templateName: '新订单值班提醒', channel: '值班群', recipient: '骑手调度群', status: '已送达', attempts: 1, createdAt: '2026-11-14T09:24:00+08:00' },
  { id: 'ND-9003', templateName: '审核结果通知', channel: '微信订阅', recipient: '周师傅', status: '失败', attempts: 2, error: '用户本次订阅额度已用完', createdAt: '2026-11-14T09:08:00+08:00' },
  { id: 'ND-9004', templateName: '配送完成回访', channel: '服务号', recipient: '吴同学', status: '发送中', attempts: 1, createdAt: '2026-11-14T08:58:00+08:00' },
  { id: 'ND-9005', templateName: '新订单值班提醒', channel: '值班群', recipient: '骑手调度群', status: '已跳过', attempts: 0, error: '模板已停用', createdAt: '2026-11-14T08:42:00+08:00' }
]

export const seedSettings: PreviewSettings = {
  share: {
    title: '湛科生活圈｜校园服务一站直达',
    description: '发现校园互助、二手好物与生活服务。',
    coverName: '校园服务默认分享封面.png'
  },
  features: {
    delivery: true,
    merchant: true,
    rider: true,
    notifications: true,
    takeout: false,
    community: true
  }
}

export const seedTakeoutMerchants: LegacyTakeoutMerchant[] = [
  { id: 'TM-1', name: '北区轻食铺', category: '轻食简餐', ordersToday: 18, revenueCents: 62800, status: '营业中' },
  { id: 'TM-2', name: '小满文具店', category: '文具零售', ordersToday: 6, revenueCents: 14300, status: '营业中' },
  { id: 'TM-3', name: '东区热卤', category: '小吃热卤', ordersToday: 0, revenueCents: 0, status: '休息中' }
]

export function formatMoney(cents: number) {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2
  }).format(cents / 100)
}

export function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

export function maskEndpoint(endpoint: string) {
  return endpoint.replace(/(key=|hook\/)([^&/]{4})[^&/]+/g, '$1$2••••')
}

export function createId(prefix: string) {
  return `${prefix}-${Date.now().toString(36).toUpperCase()}`
}
