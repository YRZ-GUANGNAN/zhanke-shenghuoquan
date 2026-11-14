export type PreviewSection =
  | 'dashboard'
  | 'delivery'
  | 'notifications'
  | 'media'
  | 'moderation'
  | 'community'
  | 'trade'
  | 'reports'
  | 'identities'
  | 'feedbacks'
  | 'users'
  | 'navigation'
  | 'share-settings'
  | 'feature-policy'
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
  appId: string
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
  features: Record<FeatureKey, boolean>
  presentationMode: 'standard' | 'campus_info'
  postingMode: 'admin_only' | 'all_users'
  rowVersion: number
  updatedAt: string
}

// 与真实小程序功能策略一一对应的 20 项开关，分组与文案取自生产后台。
export const FEATURE_KEYS = [
  'forum', 'post', 'comment', 'following', 'likes', 'favorites', 'notifications',
  'secondhand', 'secondhand_publish', 'secondhand_chat', 'secondhand_comments',
  'takeout', 'campus_tools', 'delivery', 'merchant', 'rider', 'identity_application',
  'messages', 'search', 'publish'
] as const
export type FeatureKey = typeof FEATURE_KEYS[number]

export interface FeatureGroup {
  title: string
  description: string
  keys: Array<{ key: FeatureKey; label: string; description: string }>
}

export const FEATURE_GROUPS: FeatureGroup[] = [
  {
    title: '社区内容',
    description: '只控制用户端入口与写入能力，不删除既有内容。',
    keys: [
      { key: 'forum', label: '论坛入口', description: '隐藏社区首页、帖子列表与旧链接入口' },
      { key: 'post', label: '发帖', description: '允许用户创建新的社区帖子' },
      { key: 'comment', label: '评论', description: '允许用户对社区帖子发表评论' },
      { key: 'following', label: '关注', description: '显示关注关系与关注动态入口' },
      { key: 'likes', label: '点赞', description: '帖子点赞入口与请求' },
      { key: 'favorites', label: '收藏', description: '帖子和商品收藏入口与请求' },
      { key: 'notifications', label: '互动通知', description: '评论与互动通知列表' }
    ]
  },
  {
    title: '交易与服务',
    description: '二手与校园服务可以独立切换，避免关闭外卖时误伤校内工具。',
    keys: [
      { key: 'secondhand', label: '二手市场', description: '商品浏览、详情与历史链接' },
      { key: 'secondhand_publish', label: '发布二手商品', description: '允许创建和编辑新的商品' },
      { key: 'secondhand_chat', label: '二手沟通', description: '商品联系与交易会话入口' },
      { key: 'secondhand_comments', label: '商品留言', description: '商品留言列表与回复，不受社区发言模式影响' },
      { key: 'takeout', label: '外卖商家', description: '合作外卖商家目录，不影响常用校园工具' },
      { key: 'campus_tools', label: '校园服务', description: '校内服务导航与工具入口' },
      { key: 'delivery', label: '跑腿配送', description: '新发单与抢单；已有订单履约和资金不受影响' },
      { key: 'merchant', label: '商家工作台', description: '工作台入口与新业务；历史订单处理仍保留' },
      { key: 'rider', label: '骑手工作台', description: '工作台入口与抢单；已接订单仍可完成' },
      { key: 'identity_application', label: '身份申请', description: '新增骑手/商家申请，不删除已有身份' }
    ]
  },
  {
    title: '交流关系',
    description: '关闭入口不会删除会话、关注或历史记录。',
    keys: [
      { key: 'messages', label: '私信', description: '通用消息与会话入口' },
      { key: 'search', label: '搜索', description: '社区、商品与商家的搜索入口' },
      { key: 'publish', label: '统一发布入口', description: '底部加号与发布选择页，关闭后也限制帖子和商品发布' }
    ]
  }
]

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
  { id: 'NV-1', title: '取快递', subtitle: '校园快递代取', appId: 'wx0dacef976daa8e55', path: 'pages/express/index', color: '#6f45a8', enabled: true, sort: 10 },
  { id: 'NV-2', title: '打印资料', subtitle: '上传文件后到店取件', appId: 'wx0dacef976daa8e55', path: 'pages/print/index', color: '#3e8f74', enabled: true, sort: 20 },
  { id: 'NV-3', title: '校园跑腿', subtitle: '发布同校互助任务', appId: 'wx0dacef976daa8e55', path: 'pages/errand/index', color: '#bf7a32', enabled: true, sort: 30 },
  { id: 'NV-4', title: '失物招领', subtitle: '查看最新失物信息', appId: 'wx0dacef976daa8e55', path: 'pages/lost-found/index', color: '#ad5d7a', enabled: false, sort: 40 }
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
  features: Object.fromEntries(FEATURE_KEYS.map(key => [key, key !== 'takeout'])) as Record<FeatureKey, boolean>,
  presentationMode: 'standard',
  postingMode: 'all_users',
  rowVersion: 25,
  updatedAt: '2026-10-18T15:37:29+08:00'
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

// 渠道健康统一口径：按真实投递记录聚合，总览与通知页共用。
export interface ChannelHealth {
  name: string
  delivered: number
  failed: number
  pending: number
  rate: number | null
}

const channelDisplayName: Record<string, string> = {
  小程序通知: '小程序通知中心',
  微信订阅: '微信订阅消息',
  服务号: '微信服务号',
  值班群: '值班群 Webhook'
}

export function channelHealthFrom(deliveries: NotificationDelivery[]): ChannelHealth[] {
  const buckets = new Map<string, ChannelHealth>()
  for (const record of deliveries) {
    const name = channelDisplayName[record.channel] || record.channel
    const bucket = buckets.get(name) || { name, delivered: 0, failed: 0, pending: 0, rate: null }
    if (record.status === '已送达') bucket.delivered += 1
    else if (record.status === '发送中') bucket.pending += 1
    else bucket.failed += 1  // 失败与已跳过都算未送达，避免成功率被抬高
    buckets.set(name, bucket)
  }
  return [...buckets.values()].map(bucket => {
    const settled = bucket.delivered + bucket.failed
    return { ...bucket, rate: settled > 0 ? Math.round(bucket.delivered / settled * 100) : null }
  })
}

// 「今日」按本地日期计算，不能写死某一天，否则换天演示时全部归零。
export function todayKey(now: Date = new Date()): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

export function isToday(value: string): boolean {
  return typeof value === 'string' && value.startsWith(todayKey())
}

/* ===== 配送资金与费率 ===== */
export interface MerchantWallet { merchantId: string; name: string; balanceCents: number; frozenCents: number; updatedAt: string }
export interface RiderWallet { riderId: string; name: string; balanceCents: number; pendingCents: number; updatedAt: string }
export interface RiderEarning { id: string; rider: string; orderId: string; amountCents: number; status: '待结' | '已结'; createdAt: string }
export interface PayoutQueueItem { id: string; rider: string; amountCents: number; status: '排队中' | '已打款'; requestedAt: string }
export interface WithdrawalItem { id: string; rider: string; amountCents: number; status: '待处理' | '已打款' | '已退回'; createdAt: string; reference: string; note: string }
export interface RefundItem { id: string; orderId: string; amountCents: number; status: '处理中' | '已退款' | '失败'; reason: string; createdAt: string }
export interface FeeConfig { deliveryFeeCents: number; platformCutCents: number; rowVersion: number; updatedAt: string }
export interface MerchantOverride { merchantId: string; name: string; deliveryFeeCents: number; platformCutCents: number; enabled: boolean }
export interface FundingSource { name: string; expectedCents: number; actualCents: number }

export const seedMerchantWallets: MerchantWallet[] = [
  { merchantId: 'M-01', name: '北区轻食铺', balanceCents: 38600, frozenCents: 0, updatedAt: '2026-11-14T09:20:00+08:00' },
  { merchantId: 'M-02', name: '西区外卖柜', balanceCents: 12400, frozenCents: 680, updatedAt: '2026-11-14T08:55:00+08:00' },
  { merchantId: 'M-03', name: '南区驿站', balanceCents: 0, frozenCents: 0, updatedAt: '2026-11-13T18:02:00+08:00' }
]

export const seedRiderWallets: RiderWallet[] = [
  { riderId: 'R-01', name: '周师傅', balanceCents: 8600, pendingCents: 1200, updatedAt: '2026-11-14T09:30:00+08:00' },
  { riderId: 'R-02', name: '黄师傅', balanceCents: 4200, pendingCents: 0, updatedAt: '2026-11-14T09:02:00+08:00' },
  { riderId: 'R-03', name: '李师傅', balanceCents: 1500, pendingCents: 800, updatedAt: '2026-11-13T20:41:00+08:00' }
]

export const seedRiderEarnings: RiderEarning[] = [
  { id: 'RE-101', rider: '周师傅', orderId: 'CC20261114001', amountCents: 120, status: '待结', createdAt: '2026-11-14T09:18:00+08:00' },
  { id: 'RE-102', rider: '黄师傅', orderId: 'CC20261114003', amountCents: 140, status: '待结', createdAt: '2026-11-14T08:36:00+08:00' },
  { id: 'RE-103', rider: '李师傅', orderId: 'CC20261114004', amountCents: 80, status: '已结', createdAt: '2026-11-13T19:12:00+08:00' },
  { id: 'RE-104', rider: '周师傅', orderId: 'CC2026111302', amountCents: 100, status: '已结', createdAt: '2026-11-13T17:40:00+08:00' }
]

export const seedPayoutQueue: PayoutQueueItem[] = [
  { id: 'PO-31', rider: '李师傅', amountCents: 800, status: '排队中', requestedAt: '2026-11-14T08:10:00+08:00' },
  { id: 'PO-30', rider: '周师傅', amountCents: 1200, status: '排队中', requestedAt: '2026-11-14T07:55:00+08:00' }
]

export const seedWithdrawals: WithdrawalItem[] = [
  { id: 'WD-51', rider: '周师傅', amountCents: 1200, status: '待处理', createdAt: '2026-11-14T07:55:00+08:00', reference: '', note: '' },
  { id: 'WD-50', rider: '黄师傅', amountCents: 500, status: '已打款', createdAt: '2026-11-13T16:20:00+08:00', reference: '转账回单 1129', note: '已线下转账' },
  { id: 'WD-49', rider: '李师傅', amountCents: 300, status: '已退回', createdAt: '2026-11-12T15:05:00+08:00', reference: '', note: '收款信息有误' }
]

export const seedRefunds: RefundItem[] = [
  { id: 'RF-21', orderId: 'CC2026111305', amountCents: 520, status: '处理中', reason: '商家取消出餐', createdAt: '2026-11-14T08:44:00+08:00' },
  { id: 'RF-20', orderId: 'CC2026111202', amountCents: 680, status: '已退款', reason: '超时未接单', createdAt: '2026-11-12T12:10:00+08:00' },
  { id: 'RF-19', orderId: 'CC2026111101', amountCents: 450, status: '失败', reason: '微信返回退款失败', createdAt: '2026-11-11T19:30:00+08:00' }
]

export const seedFeeConfig: FeeConfig = { deliveryFeeCents: 100, platformCutCents: 20, rowVersion: 3, updatedAt: '2026-11-10T10:00:00+08:00' }

export const seedMerchantOverrides: MerchantOverride[] = [
  { merchantId: 'M-02', name: '西区外卖柜', deliveryFeeCents: 120, platformCutCents: 30, enabled: true },
  { merchantId: 'M-01', name: '北区轻食铺', deliveryFeeCents: 80, platformCutCents: 10, enabled: false }
]

export const seedFundingSources: FundingSource[] = [
  { name: '商家充值', expectedCents: 51000, actualCents: 51000 },
  { name: '配送费支付', expectedCents: 3200, actualCents: 3120 },
  { name: '退款支出', expectedCents: 680, actualCents: 680 }
]

/* ===== 素材处理链路 ===== */
export interface MediaDecision { id: string; assetId: string; result: '通过' | '拒绝'; note: string; decidedAt: string }
export interface MediaEventItem { id: string; assetId: string; action: string; status: '已完成' | '失败'; at: string }
export interface MediaGcJob { id: string; assetId: string; kind: string; status: '已完成' | '待处理'; at: string }

export const seedMediaDecisions: MediaDecision[] = [
  { id: 'MD-11', assetId: 'AS-1', result: '通过', note: '自动审核：未检出违规', decidedAt: '2026-11-14T09:02:00+08:00' },
  { id: 'MD-12', assetId: 'AS-2', result: '拒绝', note: '人工复核：含外部联系方式', decidedAt: '2026-11-14T08:31:00+08:00' }
]
export const seedMediaEvents: MediaEventItem[] = [
  { id: 'ME-21', assetId: 'AS-1', action: '上传完成并转码', status: '已完成', at: '2026-11-14T09:01:00+08:00' },
  { id: 'ME-22', assetId: 'AS-3', action: '清理任务入队', status: '失败', at: '2026-11-14T07:48:00+08:00' }
]
export const seedMediaGc: MediaGcJob[] = [
  { id: 'MG-31', assetId: 'AS-4', kind: '对象存储删除', status: '已完成', at: '2026-11-13T22:10:00+08:00' },
  { id: 'MG-32', assetId: 'AS-3', kind: '对象存储删除', status: '待处理', at: '2026-11-14T07:48:00+08:00' }
]

/* ===== 后台登录账号 ===== */
export interface AdminCredentialItem { username: string; name: string; role: '管理员' | '超级管理员'; status: '正常' | '已锁定'; passwordUpdatedAt: string }

export const seedAdminCredentials: AdminCredentialItem[] = [
  { username: 'administrator', name: '校园用户', role: '超级管理员', status: '正常', passwordUpdatedAt: '2026-11-13T19:10:00+08:00' },
  { username: 'ops.zhang', name: '张运营', role: '管理员', status: '正常', passwordUpdatedAt: '2026-11-13T19:35:00+08:00' },
  { username: 'ops.li', name: '李客服', role: '管理员', status: '已锁定', passwordUpdatedAt: '2026-11-01T10:12:00+08:00' }
]
