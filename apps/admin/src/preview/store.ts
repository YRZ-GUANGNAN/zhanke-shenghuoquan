import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import {
  FEATURE_KEYS,
  seedAdminCredentials,
  seedCommunity,
  seedFeeConfig,
  seedFundingSources,
  seedMediaDecisions,
  seedMediaEvents,
  seedMediaGc,
  seedMerchantOverrides,
  seedMerchantWallets,
  seedPayoutQueue,
  seedRefunds,
  seedRiderEarnings,
  seedRiderWallets,
  seedWithdrawals,
  seedDeliveries,
  seedFeedbacks,
  seedIdentities,
  seedMedia,
  seedOrders,
  seedProducts,
  seedReports,
  seedRiders,
  seedReviews,
  seedServiceEntries,
  seedSettings,
  seedTakeoutMerchants,
  seedTemplates,
  seedUsers,
  seedWebhooks,
  type AdminCredentialItem,
  type CommunityItem,
  type DeliveryOrder,
  type FeeConfig,
  type FeedbackItem,
  type FundingSource,
  type MediaDecision,
  type MediaEventItem,
  type MediaGcJob,
  type IdentityItem,
  type LegacyTakeoutMerchant,
  type MerchantOverride,
  type MerchantWallet,
  type PayoutQueueItem,
  type RefundItem,
  type RiderEarning,
  type RiderWallet,
  type WithdrawalItem,
  type MediaItem,
  type MessageTemplateItem,
  type NotificationDelivery,
  type PreviewSection,
  type PreviewSettings,
  type ProductItem,
  type ReportItem,
  type RiderItem,
  type ReviewItem,
  type ServiceEntry,
  type UserItem,
  type WebhookItem
} from './data'

interface PreviewState {
  section: PreviewSection
  orders: DeliveryOrder[]
  reviews: ReviewItem[]
  community: CommunityItem[]
  products: ProductItem[]
  reports: ReportItem[]
  media: MediaItem[]
  identities: IdentityItem[]
  feedbacks: FeedbackItem[]
  users: UserItem[]
  serviceEntries: ServiceEntry[]
  templates: MessageTemplateItem[]
  webhooks: WebhookItem[]
  deliveries: NotificationDelivery[]
  settings: PreviewSettings
  takeoutMerchants: LegacyTakeoutMerchant[]
  merchantWallets: MerchantWallet[]
  riderWallets: RiderWallet[]
  riderEarnings: RiderEarning[]
  payoutQueue: PayoutQueueItem[]
  withdrawals: WithdrawalItem[]
  refunds: RefundItem[]
  feeConfig: FeeConfig
  merchantOverrides: MerchantOverride[]
  fundingSources: FundingSource[]
  mediaDecisions: MediaDecision[]
  mediaEvents: MediaEventItem[]
  mediaGc: MediaGcJob[]
  adminCredentials: AdminCredentialItem[]
  riders: RiderItem[]
  updateFeeConfig: (deliveryFeeCents: number, platformCutCents: number) => void
  toggleMerchantOverride: (merchantId: string) => void
  payWithdrawal: (id: string, reference: string) => void
  rejectWithdrawal: (id: string, note: string) => void
  markPayoutPaid: (id: string) => void
  addAdminCredential: (value: AdminCredentialItem) => void
  removeAdminCredential: (username: string) => void
  resetAdminCredential: (username: string, passwordUpdatedAt: string) => void
  reassignOrders: (ids: string[], rider: string) => void
  cancelOrders: (ids: string[], reason: string) => void
  setRiderState: (id: string, state: RiderItem['state']) => void
  hideComment: (postId: string, commentId: string) => void
  setSection: (section: PreviewSection) => void
  updateOrderStatus: (id: string, status: DeliveryOrder['status']) => void
  assignRider: (id: string, rider: string) => void
  decideReview: (id: string, status: ReviewItem['status'], reason: string) => void
  setCommunityStatus: (id: string, status: CommunityItem['status']) => void
  setProductStatus: (id: string, status: ProductItem['status']) => void
  setReportStatus: (id: string, status: ReportItem['status']) => void
  setMediaStatus: (id: string, status: MediaItem['status']) => void
  decideIdentity: (id: string, status: IdentityItem['status'], reason: string) => void
  updateFeedbackStatus: (id: string, status: FeedbackItem['status'], note: string) => void
  updateUserStatus: (id: string, status: UserItem['status']) => void
  addServiceEntry: (entry: ServiceEntry) => void
  toggleServiceEntry: (id: string) => void
  addTemplate: (template: MessageTemplateItem) => void
  updateTemplate: (template: MessageTemplateItem) => void
  toggleTemplate: (id: string) => void
  addWebhook: (webhook: WebhookItem) => void
  updateWebhook: (webhook: WebhookItem) => void
  toggleWebhook: (id: string) => void
  testWebhook: (id: string) => '测试成功' | '测试失败'
  deleteWebhook: (id: string) => void
  testSend: (templateId: string, channel: NotificationDelivery['channel'], recipient: string) => NotificationDelivery
  retryDelivery: (id: string) => void
  saveSettings: (settings: PreviewSettings) => void
  resetPreview: () => void
}

const initialData = () => ({
  orders: structuredClone(seedOrders),
  reviews: structuredClone(seedReviews),
  community: structuredClone(seedCommunity),
  products: structuredClone(seedProducts),
  reports: structuredClone(seedReports),
  media: structuredClone(seedMedia),
  identities: structuredClone(seedIdentities),
  feedbacks: structuredClone(seedFeedbacks),
  users: structuredClone(seedUsers),
  serviceEntries: structuredClone(seedServiceEntries),
  templates: structuredClone(seedTemplates),
  webhooks: structuredClone(seedWebhooks),
  deliveries: structuredClone(seedDeliveries),
  settings: structuredClone(seedSettings),
  takeoutMerchants: structuredClone(seedTakeoutMerchants),
  merchantWallets: structuredClone(seedMerchantWallets),
  riderWallets: structuredClone(seedRiderWallets),
  riderEarnings: structuredClone(seedRiderEarnings),
  payoutQueue: structuredClone(seedPayoutQueue),
  withdrawals: structuredClone(seedWithdrawals),
  refunds: structuredClone(seedRefunds),
  feeConfig: structuredClone(seedFeeConfig),
  merchantOverrides: structuredClone(seedMerchantOverrides),
  fundingSources: structuredClone(seedFundingSources),
  mediaDecisions: structuredClone(seedMediaDecisions),
  mediaEvents: structuredClone(seedMediaEvents),
  mediaGc: structuredClone(seedMediaGc),
  adminCredentials: structuredClone(seedAdminCredentials),
  riders: structuredClone(seedRiders)
})

const now = () => new Date().toISOString()

export const usePreviewStore = create<PreviewState>()(
  persist(
    (set, get) => ({
      section: 'dashboard',
      ...initialData(),
      updateFeeConfig: (deliveryFeeCents, platformCutCents) => set(state => ({
        feeConfig: { deliveryFeeCents, platformCutCents, rowVersion: state.feeConfig.rowVersion + 1, updatedAt: now() }
      })),
      toggleMerchantOverride: merchantId => set(state => ({
        merchantOverrides: state.merchantOverrides.map(item => item.merchantId === merchantId ? { ...item, enabled: !item.enabled } : item)
      })),
      payWithdrawal: (id, reference) => set(state => ({
        withdrawals: state.withdrawals.map(item => item.id === id ? { ...item, status: '已打款', reference, note: '已线下转账' } : item),
        payoutQueue: state.payoutQueue.map(item => item.id === id ? { ...item, status: '已打款' } : item)
      })),
      rejectWithdrawal: (id, note) => set(state => ({
        withdrawals: state.withdrawals.map(item => item.id === id ? { ...item, status: '已退回', note } : item)
      })),
      markPayoutPaid: id => set(state => ({
        payoutQueue: state.payoutQueue.map(item => item.id === id ? { ...item, status: '已打款' } : item)
      })),
      addAdminCredential: value => set(state => ({ adminCredentials: [...state.adminCredentials, value] })),
      removeAdminCredential: username => set(state => ({ adminCredentials: state.adminCredentials.filter(item => item.username !== username) })),
      resetAdminCredential: (username, passwordUpdatedAt) => set(state => ({
        adminCredentials: state.adminCredentials.map(item => item.username === username ? { ...item, passwordUpdatedAt, status: '正常' } : item)
      })),
      reassignOrders: (ids, rider) => set(state => ({
    orders: state.orders.map(order => ids.includes(order.id)
      ? { ...order, rider, status: order.status === '待接单' ? '配送中' : order.status, timeline: [...order.timeline, { label: `改派给${rider}`, at: now() }] }
      : order)
  })),
  cancelOrders: (ids, reason) => set(state => ({
    orders: state.orders.map(order => ids.includes(order.id)
      ? { ...order, status: '已取消', cancelReason: reason, timeline: [...order.timeline, { label: `已取消：${reason}`, at: now() }] }
      : order)
  })),
  setRiderState: (id, riderState) => set(state => ({
    riders: state.riders.map(item => item.id === id ? { ...item, state: riderState } : item)
  })),
  hideComment: (postId, commentId) => set(state => ({
    community: state.community.map(item => item.id === postId
      ? { ...item, comments: (item.comments || []).map(comment => comment.id === commentId ? { ...comment, status: '已隐藏' } : comment) }
      : item)
  })),
  setSection: section => set({ section }),
      updateOrderStatus: (id, status) => set(state => ({
        orders: state.orders.map(order => order.id === id ? { ...order, status } : order)
      })),
      assignRider: (id, rider) => set(state => ({
        orders: state.orders.map(order => order.id === id
          ? { ...order, rider, status: order.status === '待接单' ? '配送中' : order.status }
          : order)
      })),
      decideReview: (id, status, reason) => set(state => ({
        reviews: state.reviews.map(review => review.id === id
          ? { ...review, status, reason }
          : review)
      })),
      setCommunityStatus: (id, status) => set(state => ({
        community: state.community.map(item => item.id === id ? { ...item, status } : item)
      })),
      setProductStatus: (id, status) => set(state => ({
        products: state.products.map(item => item.id === id ? { ...item, status } : item)
      })),
      setReportStatus: (id, status) => set(state => ({
        reports: state.reports.map(item => item.id === id ? { ...item, status } : item)
      })),
      setMediaStatus: (id, status) => set(state => ({
        media: state.media.map(item => item.id === id ? { ...item, status } : item)
      })),
      decideIdentity: (id, status, reason) => set(state => ({
        identities: state.identities.map(item => item.id === id ? { ...item, status, reason } : item)
      })),
      updateFeedbackStatus: (id, status, note) => set(state => ({
        feedbacks: state.feedbacks.map(item => item.id === id ? { ...item, status, note } : item)
      })),
      updateUserStatus: (id, status) => set(state => ({
        users: state.users.map(item => item.id === id ? { ...item, status } : item)
      })),
      addServiceEntry: entry => set(state => ({
        serviceEntries: [...state.serviceEntries, entry]
      })),
      toggleServiceEntry: id => set(state => ({
        serviceEntries: state.serviceEntries.map(item => item.id === id
          ? { ...item, enabled: !item.enabled }
          : item)
      })),
      addTemplate: template => set(state => ({
        templates: [template, ...state.templates]
      })),
      updateTemplate: template => set(state => ({
        templates: state.templates.map(item => item.id === template.id ? template : item)
      })),
      toggleTemplate: id => set(state => ({
        templates: state.templates.map(item => item.id === id ? { ...item, enabled: !item.enabled, updatedAt: now() } : item)
      })),
      addWebhook: webhook => set(state => ({
        webhooks: [webhook, ...state.webhooks]
      })),
      updateWebhook: webhook => set(state => ({
        webhooks: state.webhooks.map(item => item.id === webhook.id ? webhook : item)
      })),
      toggleWebhook: id => set(state => ({
        webhooks: state.webhooks.map(item => item.id === id ? { ...item, enabled: !item.enabled, updatedAt: now() } : item)
      })),
      deleteWebhook: id => set(state => ({ webhooks: state.webhooks.filter(item => item.id !== id) })),
      testWebhook: id => {
        const webhook = get().webhooks.find(item => item.id === id)
        if (!webhook || !webhook.endpoint.trim()) return '测试失败'
        const result = webhook.enabled ? '测试成功' : '测试失败'
        const failure = !webhook.endpoint.trim()
          ? '地址为空'
          : !webhook.enabled
            ? '渠道已停用，未发起请求'
            : !webhook.endpoint.startsWith('https://')
              ? '地址必须以 https:// 开头'
              : ''
        set(state => ({
          webhooks: state.webhooks.map(item => item.id === id
            ? { ...item, lastTest: result, lastTestAt: now(), lastError: result === '测试成功' ? '' : failure, updatedAt: now() }
            : item)
        }))
        return result
      },
      testSend: (templateId, channel, recipient) => {
        const template = get().templates.find(item => item.id === templateId)
        const record: NotificationDelivery = {
          id: `ND-${Date.now().toString().slice(-6)}`,
          templateName: template?.name || '未命名模板',
          channel,
          recipient,
          status: template?.enabled ? '已送达' : '已跳过',
          attempts: template?.enabled ? 1 : 0,
          error: template?.enabled ? undefined : '模板已停用',
          createdAt: now()
        }
        set(state => ({ deliveries: [record, ...state.deliveries] }))
        return record
      },
      retryDelivery: id => set(state => ({
        deliveries: state.deliveries.map(item => item.id === id
          ? { ...item, status: '已送达', attempts: item.attempts + 1, error: undefined }
          : item)
      })),
      saveSettings: settings => set({ settings }),
      resetPreview: () => set({ section: 'dashboard', ...initialData() })
    }),
    {
      name: 'campus-circle-admin-preview-v1',
      version: 5,
      // v2 补服务入口 appId；v3 把旧的 6 项开关迁移成真实的 20 项功能策略。
      migrate: (persisted: unknown) => {
        const state = persisted as {
          serviceEntries?: Array<Record<string, unknown>>
          settings?: { features?: Record<string, boolean> }
          media?: Array<Record<string, unknown>>
          mediaDecisions?: Array<Record<string, unknown>>
          mediaEvents?: Array<Record<string, unknown>>
          mediaGc?: Array<Record<string, unknown>>
          identities?: Array<Record<string, unknown>>
        } | undefined
        if (!state) return persisted as never
        const features = state.settings?.features || {}
        const normalized = Object.fromEntries(FEATURE_KEYS.map(key => {
          const legacy = key === 'forum' ? features.community : key === 'secondhand' ? features.community : (features as Record<string, boolean>)[key]
          return [key, legacy !== false]
        })) as Record<string, boolean>
        // v4：素材补了规格与引用字段，旧缓存缺字段时直接用种子重建素材相关数据。
        const mediaNeedsReset = (state.media || []).some(item => item.dimensions === undefined)
        // v5：认证资料补了联系方式和证明材料，旧缓存缺字段时重建。
        const identityNeedsReset = (state.identities || []).some(item => item.materials === undefined)
        return {
          ...state,
          ...(identityNeedsReset ? { identities: structuredClone(seedIdentities) } : {}),
          ...(mediaNeedsReset ? {
            media: structuredClone(seedMedia),
            mediaDecisions: structuredClone(seedMediaDecisions),
            mediaEvents: structuredClone(seedMediaEvents),
            mediaGc: structuredClone(seedMediaGc)
          } : {}),
          serviceEntries: (state.serviceEntries || []).map(entry => ({
            ...entry,
            appId: entry.appId || 'wx0dacef976daa8e55'
          })),
          settings: {
            ...(state.settings || {}),
            features: normalized,
            presentationMode: (state.settings as { presentationMode?: string })?.presentationMode || 'standard',
            postingMode: (state.settings as { postingMode?: string })?.postingMode || 'all_users',
            rowVersion: (state.settings as { rowVersion?: number })?.rowVersion || 25,
            updatedAt: (state.settings as { updatedAt?: string })?.updatedAt || '2026-10-18T15:37:29+08:00'
          }
        } as never
      },
      partialize: state => ({
        orders: state.orders,
        reviews: state.reviews,
        community: state.community,
        products: state.products,
        reports: state.reports,
        media: state.media,
        identities: state.identities,
        feedbacks: state.feedbacks,
        users: state.users,
        serviceEntries: state.serviceEntries,
        templates: state.templates,
        webhooks: state.webhooks,
        deliveries: state.deliveries,
        settings: state.settings,
        takeoutMerchants: state.takeoutMerchants,
        merchantWallets: state.merchantWallets,
        riderWallets: state.riderWallets,
        riderEarnings: state.riderEarnings,
        payoutQueue: state.payoutQueue,
        withdrawals: state.withdrawals,
        refunds: state.refunds,
        feeConfig: state.feeConfig,
        merchantOverrides: state.merchantOverrides,
        fundingSources: state.fundingSources,
        mediaDecisions: state.mediaDecisions,
        mediaEvents: state.mediaEvents,
        mediaGc: state.mediaGc,
        adminCredentials: state.adminCredentials,
        riders: state.riders
      })
    }
  )
)
