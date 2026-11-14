import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import {
  seedCommunity,
  seedDeliveries,
  seedFeedbacks,
  seedIdentities,
  seedMedia,
  seedOrders,
  seedProducts,
  seedReports,
  seedReviews,
  seedServiceEntries,
  seedSettings,
  seedTakeoutMerchants,
  seedTemplates,
  seedUsers,
  seedWebhooks,
  type CommunityItem,
  type DeliveryOrder,
  type FeedbackItem,
  type IdentityItem,
  type LegacyTakeoutMerchant,
  type MediaItem,
  type MessageTemplateItem,
  type NotificationDelivery,
  type PreviewSection,
  type PreviewSettings,
  type ProductItem,
  type ReportItem,
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
  takeoutMerchants: structuredClone(seedTakeoutMerchants)
})

const now = () => new Date().toISOString()

export const usePreviewStore = create<PreviewState>()(
  persist(
    (set, get) => ({
      section: 'dashboard',
      ...initialData(),
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
      testWebhook: id => {
        const webhook = get().webhooks.find(item => item.id === id)
        if (!webhook || !webhook.endpoint.trim()) return '测试失败'
        const result = webhook.enabled ? '测试成功' : '测试失败'
        set(state => ({
          webhooks: state.webhooks.map(item => item.id === id
            ? { ...item, lastTest: result, updatedAt: now() }
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
      version: 2,
      // v2：服务入口新增 appId 字段，旧缓存需要补默认值，避免列表出现空 AppID。
      migrate: (persisted: unknown) => {
        const state = persisted as { serviceEntries?: Array<Record<string, unknown>> } | undefined
        if (!state?.serviceEntries) return persisted as never
        return {
          ...state,
          serviceEntries: state.serviceEntries.map(entry => ({
            ...entry,
            appId: entry.appId || 'wx0dacef976daa8e55'
          }))
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
        takeoutMerchants: state.takeoutMerchants
      })
    }
  )
)
