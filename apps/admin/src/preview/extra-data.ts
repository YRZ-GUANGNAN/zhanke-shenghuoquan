import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface LaunchStep {
  key: string
  title: string
  action: string
  done: boolean
}

export interface LaunchStage {
  key: string
  title: string
  steps: LaunchStep[]
}

export interface SiteTypeRow {
  id: string
  name: string
  platformRate: number
  agentRate: number
  status: '启用' | '停用'
}

export interface SiteRow {
  id: string
  name: string
  type: string
  balanceCents: number
  incomeCents: number
  agents: number
  takers: number
  status: '启用' | '停用'
  updatedAt: string
}

export interface FormRow {
  id: string
  name: string
  designNo: string
  user: string
  status: '待处理' | '已处理'
  createdAt: string
}

export interface TakerRow {
  id: string
  name: string
  phone: string
  level: string
  completed: number
  incomeCents: number
  onTimeRate: number
  rating: number
  status: '正常' | '停用'
}

export interface PromoterRow {
  id: string
  nickname: string
  orders: number
  rewardCents: number
  status: '启用' | '停用'
  createdAt: string
}

export interface SkuRow {
  id: string
  name: string
  stock: number
  sold: number
  status: '上架' | '下架'
}

export interface CouponRow {
  id: string
  name: string
  scope: string
  discountCents: number
  conditionCents: number
  validDays: number
  claimed: number
  used: number
  status: '启用' | '停用'
}

export interface OnceCardRow {
  id: string
  name: string
  validDays: number
  priceCents: number
  description: string
  buyers: number
  used: number
  incomeCents: number
  status: '启用' | '停用'
}

export interface ChatSessionRow {
  id: string
  user1: string
  user2: string
  lastMessage: string
  status: '正常' | '已封禁'
  updatedAt: string
}

export interface AdRow {
  id: string
  kind: '激励广告' | '免单广告'
  name: string
  views: number
  completed: number
  rewards: number
  status: '启用' | '停用'
}

export interface WithdrawalRow {
  id: string
  user: string
  amountCents: number
  account: string
  status: '待处理' | '已处理' | '已拒绝'
  createdAt: string
}

export interface MiniAppSettings {
  appId: string
  name: string
  version: string
  domains: string
  mchId: string
  apiKeyVersion: 'v2' | 'v3'
  callbackUrl: string
}

export interface ServiceAccountSettings {
  appId: string
  name: string
  followers: number
  templateIds: string
}

export interface GeneralSettings {
  systemName: string
  company: string
  icp: string
  tencentMapKey: string
  baiduMapAk: string
  amapKey: string
  prohibitedWords: string
}

export interface ExtraState {
  launchStages: LaunchStage[]
  siteTypes: SiteTypeRow[]
  sites: SiteRow[]
  forms: FormRow[]
  takers: TakerRow[]
  promoters: PromoterRow[]
  skus: SkuRow[]
  coupons: CouponRow[]
  onceCards: OnceCardRow[]
  chatSessions: ChatSessionRow[]
  ads: AdRow[]
  withdrawals: WithdrawalRow[]
  miniApp: MiniAppSettings
  serviceAccount: ServiceAccountSettings
  general: GeneralSettings
  toggleLaunchStep: (stageKey: string, stepKey: string) => void
  setFormStatus: (id: string) => void
  setTakerStatus: (id: string, status: TakerRow['status']) => void
  setPromoterStatus: (id: string, status: PromoterRow['status']) => void
  updateSkuStock: (id: string, stock: number) => void
  toggleSkuStatus: (id: string) => void
  addCoupon: (coupon: CouponRow) => void
  toggleCoupon: (id: string) => void
  toggleOnceCard: (id: string) => void
  setChatStatus: (id: string, status: ChatSessionRow['status']) => void
  toggleAd: (id: string) => void
  processWithdrawal: (id: string, status: WithdrawalRow['status']) => void
  saveMiniApp: (settings: MiniAppSettings) => void
  saveServiceAccount: (settings: ServiceAccountSettings) => void
  saveGeneral: (settings: GeneralSettings) => void
}

export const seedLaunchStages: LaunchStage[] = [
  {
    key: 'prepare',
    title: '基础资料',
    steps: [
      { key: 'service', title: '购买服务包', action: '前往购买服务包', done: true },
      { key: 'site', title: '完善站点信息', action: '完善站点信息', done: true },
      { key: 'map', title: '配置地图密钥', action: '去配置', done: true }
    ]
  },
  {
    key: 'mini',
    title: '小程序配置',
    steps: [
      { key: 'auth', title: '小程序授权', action: '去授权', done: false },
      { key: 'domain', title: '配置业务域名', action: '去配置', done: false },
      { key: 'pay', title: '配置微信支付', action: '去申请', done: false }
    ]
  },
  {
    key: 'business',
    title: '业务配置',
    steps: [
      { key: 'taker', title: '配置接单员', action: '去配置', done: false },
      { key: 'notice', title: '配置通知模板', action: '去配置', done: false },
      { key: 'coupon', title: '配置优惠券', action: '去配置', done: false }
    ]
  },
  {
    key: 'release',
    title: '发布上线',
    steps: [
      { key: 'test', title: '体验版测试', action: '查看详情', done: false },
      { key: 'audit', title: '提交审核', action: '查看详情', done: false },
      { key: 'publish', title: '发布正式版', action: '去发布正式版', done: false }
    ]
  }
]

const initial = () => ({
  launchStages: structuredClone(seedLaunchStages),
  siteTypes: [
    { id: 'ST-1', name: '校园驿站', platformRate: 0.02, agentRate: 0.01, status: '启用' as const },
    { id: 'ST-2', name: '综合服务点', platformRate: 0.03, agentRate: 0.015, status: '启用' as const }
  ],
  sites: [
    { id: 'S-1001', name: '湛科生活圈', type: '校园驿站', balanceCents: 128600, incomeCents: 56200, agents: 2, takers: 15, status: '启用' as const, updatedAt: '2026-11-14 09:20' }
  ],
  forms: [
    { id: 'FM-01', name: '商家入驻申请', designNo: 'D-101', user: '北区轻食铺', status: '待处理' as const, createdAt: '2026-11-14 08:50' },
    { id: 'FM-02', name: '跑腿意见收集', designNo: 'D-202', user: '林同学', status: '已处理' as const, createdAt: '2026-11-13 16:10' }
  ],
  takers: [
    { id: 'TK-1', name: '周师傅', phone: '139****2631', level: 'Lv.2', completed: 186, incomeCents: 43200, onTimeRate: 98.2, rating: 4.9, status: '正常' as const },
    { id: 'TK-2', name: '黄师傅', phone: '136****8052', level: 'Lv.1', completed: 92, incomeCents: 19800, onTimeRate: 96.5, rating: 4.8, status: '正常' as const }
  ],
  promoters: [
    { id: 'PR-1', nickname: '校园推广小分队', orders: 33, rewardCents: 6600, status: '启用' as const, createdAt: '2026-10-02' }
  ],
  skus: [
    { id: 'SKU-1', name: '校园明信片套装', stock: 120, sold: 45, status: '上架' as const },
    { id: 'SKU-2', name: '社团联名帆布袋', stock: 38, sold: 22, status: '上架' as const }
  ],
  coupons: [
    { id: 'CP-1', name: '新用户立减', scope: '全部服务', discountCents: 200, conditionCents: 500, validDays: 7, claimed: 18, used: 6, status: '启用' as const },
    { id: 'CP-2', name: '周五跑腿券', scope: '跑腿服务', discountCents: 100, conditionCents: 1000, validDays: 1, claimed: 12, used: 3, status: '停用' as const }
  ],
  onceCards: [
    { id: 'OC-1', name: '跑腿次卡 10 次', validDays: 60, priceCents: 2900, description: '限本人使用', buyers: 5, used: 21, incomeCents: 14500, status: '启用' as const }
  ],
  chatSessions: [
    { id: 'CH-1', user1: '林同学', user2: '周师傅', lastMessage: '好的，我放东门了', status: '正常' as const, updatedAt: '2026-11-14 09:31' },
    { id: 'CH-2', user1: '陈同学', user2: '北区轻食铺', lastMessage: '谢谢老板', status: '正常' as const, updatedAt: '2026-11-13 18:42' }
  ],
  ads: [
    { id: 'AD-1', kind: '激励广告' as const, name: '订单完成激励', views: 521, completed: 387, rewards: 366, status: '启用' as const },
    { id: 'AD-2', kind: '免单广告' as const, name: '首页免单入口', views: 210, completed: 141, rewards: 8, status: '停用' as const }
  ],
  withdrawals: [
    { id: 'WD-1', user: '周师傅', amountCents: 12000, account: '支付宝 139****', status: '待处理' as const, createdAt: '2026-11-14 09:02' },
    { id: 'WD-2', user: '北区轻食铺', amountCents: 8600, account: '对公账户', status: '已处理' as const, createdAt: '2026-11-13 17:20' }
  ],
  miniApp: {
    appId: 'wx065d8f3ce4a056ee',
    name: '湛科生活圈',
    version: '2026.11.14.1',
    domains: 'https://api.zhanduiwang.top',
    mchId: '',
    apiKeyVersion: 'v3' as const,
    callbackUrl: ''
  },
  serviceAccount: {
    appId: '',
    name: '湛科生活圈服务号',
    followers: 0,
    templateIds: ''
  },
  general: {
    systemName: '湛科生活圈',
    company: '湛科运营团队',
    icp: '',
    tencentMapKey: '',
    baiduMapAk: '',
    amapKey: '',
    prohibitedWords: '诈骗,刷单,兼职返利'
  }
})

export const useExtraStore = create<ExtraState>()(
  persist(
    (set) => ({
      ...initial(),
      toggleLaunchStep: (stageKey, stepKey) => set(state => ({
        launchStages: state.launchStages.map(stage => stage.key !== stageKey
          ? stage
          : {
              ...stage,
              steps: stage.steps.map(step => step.key === stepKey ? { ...step, done: !step.done } : step)
            })
      })),
      setFormStatus: id => set(state => ({
        forms: state.forms.map(item => item.id === id ? { ...item, status: '已处理' as const } : item)
      })),
      setTakerStatus: (id, status) => set(state => ({
        takers: state.takers.map(item => item.id === id ? { ...item, status } : item)
      })),
      setPromoterStatus: (id, status) => set(state => ({
        promoters: state.promoters.map(item => item.id === id ? { ...item, status } : item)
      })),
      updateSkuStock: (id, stock) => set(state => ({
        skus: state.skus.map(item => item.id === id ? { ...item, stock } : item)
      })),
      toggleSkuStatus: id => set(state => ({
        skus: state.skus.map(item => item.id === id ? { ...item, status: item.status === '上架' ? '下架' : '上架' } : item)
      })),
      addCoupon: coupon => set(state => ({ coupons: [coupon, ...state.coupons] })),
      toggleCoupon: id => set(state => ({
        coupons: state.coupons.map(item => item.id === id ? { ...item, status: item.status === '启用' ? '停用' : '启用' } : item)
      })),
      toggleOnceCard: id => set(state => ({
        onceCards: state.onceCards.map(item => item.id === id ? { ...item, status: item.status === '启用' ? '停用' : '启用' } : item)
      })),
      setChatStatus: (id, status) => set(state => ({
        chatSessions: state.chatSessions.map(item => item.id === id ? { ...item, status } : item)
      })),
      toggleAd: id => set(state => ({
        ads: state.ads.map(item => item.id === id ? { ...item, status: item.status === '启用' ? '停用' : '启用' } : item)
      })),
      processWithdrawal: (id, status) => set(state => ({
        withdrawals: state.withdrawals.map(item => item.id === id ? { ...item, status } : item)
      })),
      saveMiniApp: settings => set({ miniApp: settings }),
      saveServiceAccount: settings => set({ serviceAccount: settings }),
      saveGeneral: settings => set({ general: settings })
    }),
    {
      name: 'campus-circle-admin-preview-extra-v1',
      partialize: state => ({
        launchStages: state.launchStages,
        siteTypes: state.siteTypes,
        sites: state.sites,
        forms: state.forms,
        takers: state.takers,
        promoters: state.promoters,
        skus: state.skus,
        coupons: state.coupons,
        onceCards: state.onceCards,
        chatSessions: state.chatSessions,
        ads: state.ads,
        withdrawals: state.withdrawals,
        miniApp: state.miniApp,
        serviceAccount: state.serviceAccount,
        general: state.general
      })
    }
  )
)
