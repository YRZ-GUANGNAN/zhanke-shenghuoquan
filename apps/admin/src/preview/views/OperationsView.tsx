import { useCallback, useEffect, useRef, useState } from 'react'
import { AdminAPIError, adminAPI } from '../../services/f-admin'
import type {
  MediaAssetDetail,
  MediaAssetRow,
  MediaDecisionRow,
  MediaDeadletters,
  MediaEventRow,
  MediaGCJobRow,
  MediaSummary,
  Page,
  PostRow,
  ProductRow,
  ReportDetail,
  ReportRow,
  ReviewTask,
  Stats
} from '../../types'
import type { PreviewSection, Tone } from '../data'
import {
  Button,
  Drawer,
  EmptyState,
  Modal,
  PageHeader,
  Pagination,
  Panel,
  SearchBox,
  Segmented,
  StatCard,
  StatusPill,
  TextAreaField,
  formatWait,
  usePreviewToast
} from '../ui'
import { RealImage, RealImageCell, RealImageStrip, adminAssetContentUrl } from '../media-thumbs'

type OperationSection = Extract<PreviewSection, 'media' | 'moderation' | 'community' | 'trade' | 'reports'>

// 与后端保持一致的每页条数，全部列表都走真实 page/pageSize/total。
const PAGE_SIZE = 8

// 金额统一 ¥(cents/100).toFixed(2)；时间统一本地化字符串。
function money(cents?: number | null): string {
  if (cents === undefined || cents === null) return '—'
  return `¥${(cents / 100).toFixed(2)}`
}

function displayTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN')
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

// 409 统一提示数据冲突；其余错误原样展示在页面内。
function requestErrorText(error: unknown): string {
  if (error instanceof AdminAPIError) {
    if (error.status === 409) return '数据已被他人修改，请刷新后重试'
    return error.message || '请求失败，请稍后重试'
  }
  if (error instanceof Error) return error.message || '请求失败，请稍后重试'
  return String(error)
}

// 复制自 App.tsx 顶部的映射，保证新旧两套界面的中文口径一致。
const REVIEW_OBJECT_LABELS: Record<string, string> = {
  post: '帖子',
  comment: '评论',
  product: '商品',
  product_comment: '商品留言',
  report: '举报',
  takeout_merchant: '外卖商家',
  takeout_dish: '外卖菜品',
  merchant: '商家',
  rider: '骑手'
}
const REVIEW_KIND_SHORT: Record<string, string> = {
  post: '帖子',
  comment: '评论',
  product: '商品',
  product_comment: '留言',
  report: '举报',
  takeout_merchant: '外卖',
  takeout_dish: '菜品',
  merchant: '商家',
  rider: '骑手'
}
const REVIEW_RISK_LABELS: Record<string, string> = {
  'wechat safety skipped': '微信安全检测未执行',
  wechat_safety_skipped: '微信安全检测未执行',
  'manual:wechat_safety_unconfigured': '微信安全检测未配置（转人工）',
  'takeout merchant onboarding': '外卖商家入驻审核',
  takeout_merchant_onboarding: '外卖商家入驻审核',
  'local secondhand demo': '本地二手演示数据'
}
const REVIEW_STATUS_LABELS: Record<string, string> = {
  submitted: '待审核',
  reviewing: '审核中',
  approved: '已通过',
  rejected: '已拒绝'
}
const REVIEW_STATUS_TONES: Record<string, Tone> = {
  submitted: 'warning',
  reviewing: 'info',
  approved: 'success',
  rejected: 'danger'
}
const REVIEW_DECISION_SOURCE_LABELS: Record<string, string> = {
  manual: '人工审核',
  automatic: '自动审核',
  manual_override: '人工改判'
}

const objectLabel = (objectType: string) => REVIEW_OBJECT_LABELS[objectType] || objectType || '未知类型'
const shortKind = (objectType: string) => REVIEW_KIND_SHORT[objectType] || objectLabel(objectType).slice(0, 2)
const riskText = (risk?: string) => {
  const value = (risk || '').trim()
  if (!value) return '无风险备注'
  return REVIEW_RISK_LABELS[value] || value
}
const reviewStatusLabel = (status: string) => REVIEW_STATUS_LABELS[status] || status || '未知状态'
const reviewStatusTone = (status: string): Tone => REVIEW_STATUS_TONES[status] || 'neutral'
const decisionSourceLabel = (source: string) => REVIEW_DECISION_SOURCE_LABELS[source] || source || '未知来源'

// 只对待处理任务展示积压时长，口径与总览页一致。
function waitingLabel(createdAt: string, status: ReviewTask['status']): string {
  if (status !== 'submitted' && status !== 'reviewing') return ''
  const elapsed = formatWait(createdAt)
  return elapsed ? `已等待 ${elapsed}` : ''
}

// 社区与商品共用的审核状态中文映射（含下架终态）。
const POST_STATUS_LABELS: Record<string, string> = { ...REVIEW_STATUS_LABELS, approved: '已公开', taken_down: '已下架' }
const PRODUCT_STATUS_LABELS: Record<string, string> = { ...REVIEW_STATUS_LABELS, taken_down: '已下架' }
const PRODUCT_BUSINESS_LABELS: Record<string, string> = { selling: '出售中', reserved: '已预留', sold: '已售出' }
const REPORT_TARGET_LABELS: Record<string, string> = { ...REVIEW_OBJECT_LABELS, user: '用户' }
const reportTargetLabel = (targetType: string) => REPORT_TARGET_LABELS[targetType] || targetType || '未知对象'

// 素材中心：审核投影、技术处理、消费与清理状态的中文映射。
const MEDIA_REVIEW_LABELS: Record<string, string> = {
  pending: '待提交',
  submitted: '待审核',
  reviewing: '审核中',
  approved: '已通过',
  rejected: '已拒绝',
  review_failed: '审核异常',
  unavailable: '无审核投影',
  not_required: '无需审核'
}
const MEDIA_REVIEW_TONES: Record<string, Tone> = {
  pending: 'warning',
  submitted: 'warning',
  reviewing: 'info',
  approved: 'success',
  rejected: 'danger',
  review_failed: 'danger',
  unavailable: 'neutral',
  not_required: 'neutral'
}
const MEDIA_TECH_LABELS: Record<string, string> = {
  pending_upload: '待上传',
  uploaded: '已上传',
  validating: '校验中',
  promoting: '处理中',
  final_ready: '已就绪',
  submitted: '已提交',
  reviewing: '审核中',
  approved: '已通过',
  rejected: '已拒绝',
  review_failed: '处理失败',
  delete_pending: '待删除',
  delete_deadletter: '删除异常',
  deleted: '已删除'
}
const MEDIA_PURPOSE_LABELS: Record<string, string> = {
  post: '帖子图片',
  post_image: '帖子图片',
  product: '商品图片',
  product_image: '商品图片',
  avatar: '头像',
  profile_cover: '主页封面',
  identity_application: '认证材料',
  dish: '外卖菜品',
  merchant_logo: '商家标志',
  delivery_proof: '配送凭证',
  delivery_im: '配送会话图片',
  feedback: '反馈附件'
}
const MEDIA_CONSUMER_LABELS: Record<string, string> = {
  unconsumed: '未消费',
  pending: '排队中',
  processing: '处理中',
  retry_wait: '等待重试',
  succeeded: '已完成',
  deadletter: '死信'
}
const MEDIA_CONSUMER_TONES: Record<string, Tone> = {
  unconsumed: 'neutral',
  pending: 'warning',
  processing: 'info',
  retry_wait: 'warning',
  succeeded: 'success',
  deadletter: 'danger'
}
const MEDIA_DECISION_LABELS: Record<string, string> = { approved: '通过', rejected: '拒绝', review_failed: '投影失败' }
const MEDIA_OPERATION_LABELS: Record<string, string> = {
  ReviewProjectionSubmitted: '提交审核',
  ReviewProjectionReviewing: '进入审核',
  ReviewProjectionDecision: '审核结论'
}
const MEDIA_GC_KIND_LABELS: Record<string, string> = { object: '对象存储', auxiliary: '辅助对象' }

const mediaReviewLabel = (status: string) => MEDIA_REVIEW_LABELS[status] || status || '未知状态'
const mediaReviewTone = (status: string): Tone => MEDIA_REVIEW_TONES[status] || 'neutral'
const mediaTechLabel = (status: string) => MEDIA_TECH_LABELS[status] || status || '未知状态'
const mediaPurposeLabel = (purpose: string) => MEDIA_PURPOSE_LABELS[purpose] || purpose || '用途未标注'
const mediaConsumerLabel = (status: string) => MEDIA_CONSUMER_LABELS[status] || status || '未知状态'
const mediaConsumerTone = (status: string): Tone => MEDIA_CONSUMER_TONES[status] || 'neutral'

function mediaKind(asset: MediaAssetRow): '图片' | '视频' | '文件' {
  if (asset.mimeType?.startsWith('video/')) return '视频'
  if (asset.mimeType?.startsWith('image/')) return '图片'
  return '文件'
}

// 统一的真实分页加载：page/pageSize/total 全部来自接口。
interface RemotePageResult<T> {
  data: Page<T> | null
  page: number
  setPage: (page: number) => void
  loading: boolean
  error: string
  reload: () => void
}

function useRemotePage<T>(loader: (page: number, pageSize: number) => Promise<Page<T>>, reloadKey?: string): RemotePageResult<T> {
  const [page, setPageState] = useState(1)
  const [data, setData] = useState<Page<T> | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  // 版本号用于手动刷新：写入成功后重新拉取当前页。
  const [version, setVersion] = useState(0)
  const loaderRef = useRef(loader)
  loaderRef.current = loader
  // 筛选/搜索条件变化（reloadKey）时回到第 1 页，避免停在旧页码上。
  const lastReloadKey = useRef(reloadKey)

  useEffect(() => {
    if (lastReloadKey.current !== reloadKey) {
      lastReloadKey.current = reloadKey
      if (page !== 1) {
        setPageState(1)
        return
      }
    }
    let active = true
    setLoading(true)
    loaderRef.current(page, PAGE_SIZE)
      .then(value => {
        if (!active) return
        if (value.items.length === 0 && page > 1) {
          // 当前页越界：直接跳到最后一页，而不是逐页回退。
          const lastPage = value.total > 0 ? Math.max(1, Math.ceil(value.total / (value.pageSize || PAGE_SIZE))) : 1
          setPageState(current => current === lastPage ? 1 : lastPage)
          return
        }
        setData(value)
        setError('')
      })
      .catch(reason => {
        if (active) setError(requestErrorText(reason))
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [page, version, reloadKey])

  const setPage = useCallback((next: number) => setPageState(Math.max(1, next)), [])
  const reload = useCallback(() => setVersion(current => current + 1), [])
  return { data, page, setPage, loading, error, reload }
}

interface OperationMeta {
  eyebrow: string
  title: string
  description: string
  action: string
}

const metaBySection: Record<OperationSection, OperationMeta> = {
  media: {
    eyebrow: '内容基础设施',
    title: '素材中心',
    description: '检查图片与视频是否已经完成技术处理、审核和业务引用。',
    action: '检查异常素材'
  },
  moderation: {
    eyebrow: '统一审核流',
    title: '审核中心',
    description: '从提交、审核到结论，每一步都记录下来源、风险和原因。',
    action: '刷新审核任务'
  },
  community: {
    eyebrow: '社区运营',
    title: '社区内容',
    description: '查看内容发布状态，并将审核结论连接到公开或下架状态。',
    action: '查看待审内容'
  },
  trade: {
    eyebrow: '交易内容',
    title: '二手商品',
    description: '商品业务状态与审核状态分开管理，下架动作会保留原因。',
    action: '查看出售中商品'
  },
  reports: {
    eyebrow: '用户安全',
    title: '举报处理',
    description: '优先处理影响交易安全与社区体验的举报。',
    action: '处理待办举报'
  }
}

export function OperationsView({ section }: { section: OperationSection }) {
  if (section === 'media') return <MediaOperations />
  if (section === 'moderation') return <ReviewOperations />
  if (section === 'community') return <CommunityOperations />
  if (section === 'trade') return <TradeOperations />
  return <ReportOperations />
}

/* ===== 素材中心 ===== */

type MediaFilter = '全部' | '待审核' | '异常' | '已通过'

const MEDIA_FILTER_STATUS: Record<Exclude<MediaFilter, '全部'>, string> = {
  待审核: 'submitted',
  异常: 'review_failed',
  已通过: 'approved'
}

function MediaOperations() {
  const [tab, setTab] = useState<'assets' | 'decisions' | 'events' | 'gc' | 'deadletters'>('assets')
  const [refreshTick, setRefreshTick] = useState(0)
  const [filter, setFilter] = useState<MediaFilter>('全部')

  return <div>
    <PageHeader
      eyebrow={metaBySection.media.eyebrow}
      title={metaBySection.media.title}
      description={metaBySection.media.description}
      actions={<Button variant="primary" onClick={() => { setTab('assets'); setFilter('异常'); setRefreshTick(tick => tick + 1) }}>{metaBySection.media.action}</Button>}
    />
    <div className="preview-subtabs" role="tablist" aria-label="素材模块">
      {([['assets', '素材'], ['decisions', '审核记录'], ['events', '处理记录'], ['gc', '清理记录'], ['deadletters', '异常记录']] as const).map(([key, label]) => <button
        key={key}
        type="button"
        role="tab"
        aria-selected={tab === key}
        className={tab === key ? 'active' : ''}
        onClick={() => setTab(key)}
      >{label}</button>)}
    </div>

    {tab === 'assets' && <MediaAssetsTab filter={filter} onFilter={setFilter} refreshTick={refreshTick} />}
    {tab === 'decisions' && <MediaDecisionsTab />}
    {tab === 'events' && <MediaEventsTab />}
    {tab === 'gc' && <MediaGCJobsTab />}
    {tab === 'deadletters' && <MediaDeadlettersTab />}
  </div>
}

function MediaAssetsTab({ filter, onFilter, refreshTick }: { filter: MediaFilter; onFilter: (value: MediaFilter) => void; refreshTick: number }) {
  const [search, setSearch] = useState('')
  const debouncedSearch = search.trim()
  const list = useRemotePage<MediaAssetRow>((page, pageSize) => adminAPI.mediaAssets({
    page,
    pageSize,
    ...(debouncedSearch ? { keyword: debouncedSearch } : {}),
    ...(filter === '全部' ? {} : { reviewStatus: MEDIA_FILTER_STATUS[filter] })
  }), `${filter}|${debouncedSearch}|${refreshTick}`)
  const [summary, setSummary] = useState<MediaSummary | null>(null)
  const [summaryError, setSummaryError] = useState('')
  const [detailId, setDetailId] = useState<string | null>(null)
  const [detail, setDetail] = useState<MediaAssetDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')

  const loadSummary = useCallback(async () => {
    try {
      setSummary(await adminAPI.mediaSummary())
      setSummaryError('')
    } catch (error) {
      setSummaryError(requestErrorText(error))
    }
  }, [])
  useEffect(() => { void loadSummary() }, [loadSummary])

  useEffect(() => {
    if (!detailId) return
    let active = true
    setDetail(null)
    setDetailLoading(true)
    setDetailError('')
    adminAPI.mediaAsset(detailId)
      .then(value => {
        if (!active) return
        // 后端对未产生的明细可能返回 null，这里统一兜底为空数组，避免详情渲染崩溃。
        setDetail({
          ...value,
          references: value.references ?? [],
          decisions: value.decisions ?? [],
          events: value.events ?? [],
          gcJobs: value.gcJobs ?? []
        })
      })
      .catch(error => { if (active) setDetailError(requestErrorText(error)) })
      .finally(() => { if (active) setDetailLoading(false) })
    return () => { active = false }
  }, [detailId])

  const items = list.data?.items ?? []

  const changeFilter = (value: MediaFilter) => {
    onFilter(value)
    list.setPage(1)
    list.reload()
  }

  return <>
    <section className="preview-stat-grid">
      <StatCard label="素材总数" value={summary?.assets ?? '—'} meta="图片与视频资源" tone="accent" />
      <StatCard label="待审核" value={summary?.pendingReview ?? '—'} meta="等待审核结论" tone="amber" />
      <StatCard label="消费异常" value={summary?.consumerDeadletters ?? '—'} meta="事件消费进入死信" tone="rose" />
      <StatCard label="清理异常" value={summary?.gcDeadletters ?? '—'} meta="清理任务进入死信" tone="ink" />
    </section>
    {summaryError && <div className="preview-form-error" role="alert">{summaryError}</div>}
    <Panel
      eyebrow="ASSET REVIEW"
      title="素材工作区"
      description="只读巡检素材的技术处理与审核投影状态；素材审核通过统一审核流处理。"
      actions={<div className="preview-row-actions">
        <SearchBox value={search} onChange={setSearch} placeholder="搜索资产编号或工单号" />
        <Segmented value={filter} onChange={changeFilter} options={[
          { value: '全部', label: '全部' },
          { value: '待审核', label: '待审核' },
          { value: '异常', label: '异常' },
          { value: '已通过', label: '已通过' }
        ]} />
      </div>}
    >
      {list.error && <div className="preview-form-error" role="alert">{list.error}</div>}
      {!list.data && list.loading
        ? <EmptyState title="正在读取素材" description="素材数据加载完成后会自动显示。" />
        : items.length === 0 && !list.error
          ? <EmptyState title="当前筛选下没有素材" description="调整状态筛选后再试。" />
          : list.error && items.length === 0
            ? null
          : <div className="preview-media-grid">
            {items.map((asset, index) => {
              const kind = mediaKind(asset)
              return <article key={asset.assetId} className="preview-media-card is-clickable" onClick={() => setDetailId(asset.assetId)}>
                <button type="button" className="preview-media-card__visual preview-media-card__visual--real" onClick={event => { event.stopPropagation(); setDetailId(asset.assetId) }} aria-label={`查看 ${asset.assetId} 详情`}>
                  {asset.storageStage !== 'final'
                    ? <span className="preview-media-card__kind">未定稿</span>
                    : kind === '视频'
                      ? <span className="preview-media-card__kind">▶ 视频</span>
                      : <RealImage src={adminAssetContentUrl(asset.assetId)} alt={`素材 ${asset.assetId}`} className="preview-media-card__img" />}
                </button>
                <div className="preview-media-card__body">
                  <div>
                    <strong>{asset.assetId}</strong>
                    <StatusPill tone={mediaReviewTone(asset.reviewStatus)}>{mediaReviewLabel(asset.reviewStatus)}</StatusPill>
                  </div>
                  <dl>
                    <div><dt>用途</dt><dd>{mediaPurposeLabel(asset.purpose)}</dd></div>
                    <div><dt>大小</dt><dd>{formatBytes(asset.byteSize)}</dd></div>
                    <div><dt>技术状态</dt><dd>{mediaTechLabel(asset.technicalStatus)}</dd></div>
                    <div><dt>时间</dt><dd>{displayTime(asset.createdAt)}</dd></div>
                  </dl>
                  <span className="preview-muted-action">审核通过统一审核流处理</span>
                </div>
              </article>
            })}
          </div>}
      <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="素材列表分页" />
    </Panel>

    <Drawer
      open={detailId !== null}
      onClose={() => setDetailId(null)}
      title={detailId || '素材详情'}
      description={detail ? `${mediaPurposeLabel(detail.asset.purpose)} · ${displayTime(detail.asset.createdAt)}` : undefined}
      footer={<Button onClick={() => setDetailId(null)}>关闭</Button>}
    >
      {detailLoading && !detail && <EmptyState title="正在读取素材详情" description="处理记录、审核记录与清理任务会一并展示。" />}
      {detailError && <div className="preview-form-error" role="alert">{detailError}</div>}
      {detail && <div className="preview-detail-stack">
        <div className="preview-media-detail-image">
          {detail.asset.storageStage === 'final'
            ? <RealImage src={adminAssetContentUrl(detail.asset.assetId)} alt={`素材 ${detail.asset.assetId}`} className="preview-media-detail-image__img" />
            : <p className="preview-image-empty">该素材尚未完成定稿，暂无可预览的文件。</p>}
        </div>
        <div className="preview-detail-banner">
          <div><span>审核投影状态</span><strong>{mediaReviewLabel(detail.asset.reviewStatus)}</strong></div>
          <StatusPill tone={mediaReviewTone(detail.asset.reviewStatus)}>{mediaReviewLabel(detail.asset.reviewStatus)}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>用途</span><strong>{mediaPurposeLabel(detail.asset.purpose)}</strong></div>
          <div><span>技术状态</span><strong>{mediaTechLabel(detail.asset.technicalStatus)}</strong></div>
          <div><span>文件大小</span><strong>{formatBytes(detail.asset.byteSize)}</strong></div>
          <div><span>规格</span><strong>{detail.asset.width && detail.asset.height ? `${detail.asset.width}×${detail.asset.height}` : '—'}</strong></div>
          <div><span>上传人</span><strong>用户 #{detail.asset.ownerId}</strong></div>
          <div><span>被引用</span><strong>{detail.asset.referenceCount} 处（阻断 {detail.asset.blockingReferences}）</strong></div>
          <div><span>存储阶段</span><strong>{detail.asset.storageStage || '—'}</strong></div>
          <div><span>文件类型</span><strong>{detail.asset.mimeType || '—'}</strong></div>
          <div><span>上传时间</span><strong>{displayTime(detail.asset.createdAt)}</strong></div>
          <div><span>最后更新</span><strong>{displayTime(detail.asset.updatedAt)}</strong></div>
        </div>

        <div className="preview-note-block">
          <span>审核投影</span>
          {detail.projection
            ? <p>{mediaReviewLabel(detail.projection.status)} · 任务 #{detail.projection.reviewTaskId ?? '—'} · 依据版本 v{detail.projection.appliedDecisionVersion}{detail.projection.failureCode ? ` · ${detail.projection.failureCode}` : ''} · {displayTime(detail.projection.updatedAt)}</p>
            : <p>该素材尚无审核投影。</p>}
        </div>

        <div className="preview-note-block">
          <span>引用记录（{detail.references.length}）</span>
          {detail.references.length === 0 && <p>暂无业务引用</p>}
          {detail.references.map(reference => <p key={reference.id}>
            {reference.referenceType} #{reference.referenceId} · 位 {reference.slot} · {reference.status} · 可见性 {reference.parentVisibility} · {displayTime(reference.updatedAt)}
          </p>)}
        </div>

        <div className="preview-note-block">
          <span>审核记录（{detail.decisions.length}）</span>
          {detail.decisions.length === 0 && <p>暂无审核记录</p>}
          {detail.decisions.map(decision => <p key={`${decision.projectionId}-${decision.decisionVersion}`}>
            {MEDIA_DECISION_LABELS[decision.decision] || decision.decision} · 任务 #{decision.reviewTaskId} · v{decision.decisionVersion}{decision.resultCode ? ` · ${decision.resultCode}` : ''} · {displayTime(decision.receivedAt)}
          </p>)}
        </div>

        <div className="preview-note-block">
          <span>处理记录（{detail.events.length}）</span>
          {detail.events.length === 0 && <p>暂无处理记录</p>}
          {detail.events.map(event => <p key={event.eventId}>
            {MEDIA_OPERATION_LABELS[event.operation] || event.operation} · {mediaConsumerLabel(event.consumerStatus)} · 尝试 {event.attemptCount} 次{event.lastErrorCode ? ` · ${event.lastErrorCode}` : ''} · {displayTime(event.createdAt)}
          </p>)}
        </div>

        <div className="preview-note-block">
          <span>清理任务（{detail.gcJobs.length}）</span>
          {detail.gcJobs.length === 0 && <p>暂无清理任务</p>}
          {detail.gcJobs.map(job => <p key={job.jobId}>
            {MEDIA_GC_KIND_LABELS[job.kind] || job.kind} · {mediaConsumerLabel(job.status)} · 尝试 {job.attemptCount} 次{job.lastErrorCode ? ` · ${job.lastErrorCode}` : ''} · {displayTime(job.createdAt)}
          </p>)}
        </div>

        <div className="preview-warning-box">
          <strong>素材审核通过统一审核流处理</strong>
          <p>后台只提供素材处理与审核投影的只读查询，没有直接改写素材状态的接口。</p>
        </div>
      </div>}
    </Drawer>
  </>
}

function MediaDecisionsTab() {
  const list = useRemotePage<MediaDecisionRow>((page, pageSize) => adminAPI.mediaDecisions({ page, pageSize }))
  const items = list.data?.items ?? []
  return <Panel eyebrow="REVIEW DECISION" title="审核记录" description="审核投影收到的每次结论都会留下不可修改的版本记录。">
    {list.error && <div className="preview-form-error" role="alert">{list.error}</div>}
    {list.error
      ? null
      : !list.data && list.loading
      ? <EmptyState title="正在读取审核记录" description="结论记录加载完成后会自动显示。" />
      : items.length === 0
        ? <EmptyState title="暂无审核记录" description="素材审核投影还没有产生结论。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="素材审核记录">
          <thead><tr><th>素材</th><th>审核任务</th><th>结论</th><th>结果码</th><th>版本</th><th>时间</th></tr></thead>
          <tbody>{items.map(item => <tr key={`${item.projectionId}-${item.decisionVersion}`}>
            <td><strong>{item.assetId}</strong></td>
            <td>#{item.reviewTaskId}</td>
            <td><StatusPill tone={item.decision === 'approved' ? 'success' : 'danger'}>{MEDIA_DECISION_LABELS[item.decision] || item.decision}</StatusPill></td>
            <td>{item.resultCode || '—'}</td>
            <td>v{item.decisionVersion}</td>
            <td>{displayTime(item.receivedAt)}</td>
          </tr>)}</tbody>
        </table></div>}
    <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="素材审核记录分页" />
  </Panel>
}

function MediaEventsTab() {
  const list = useRemotePage<MediaEventRow>((page, pageSize) => adminAPI.mediaEvents({ page, pageSize }))
  const items = list.data?.items ?? []
  return <Panel eyebrow="PROCESS EVENT" title="处理记录" description="上传、转码、投影等处理动作的消费结果。">
    {list.error && <div className="preview-form-error" role="alert">{list.error}</div>}
    {list.error
      ? null
      : !list.data && list.loading
      ? <EmptyState title="正在读取处理记录" description="事件记录加载完成后会自动显示。" />
      : items.length === 0
        ? <EmptyState title="暂无处理记录" description="素材处理事件还没有产生记录。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="素材处理记录">
          <thead><tr><th>动作</th><th>聚合</th><th>消费状态</th><th>尝试</th><th>错误码</th><th>时间</th></tr></thead>
          <tbody>{items.map(item => <tr key={item.eventId}>
            <td><strong>{MEDIA_OPERATION_LABELS[item.operation] || item.operation}</strong></td>
            <td className="preview-cell-wrap">{item.aggregateId}</td>
            <td><StatusPill tone={mediaConsumerTone(item.consumerStatus)}>{mediaConsumerLabel(item.consumerStatus)}</StatusPill></td>
            <td>{item.attemptCount}</td>
            <td>{item.lastErrorCode || '—'}</td>
            <td>{displayTime(item.createdAt)}</td>
          </tr>)}</tbody>
        </table></div>}
    <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="素材处理记录分页" />
  </Panel>
}

function MediaGCJobsTab() {
  const list = useRemotePage<MediaGCJobRow>((page, pageSize) => adminAPI.mediaGCJobs({ page, pageSize }))
  const items = list.data?.items ?? []
  return <Panel eyebrow="CLEANUP JOB" title="清理记录" description="删除素材后由后台异步清理对象存储，失败会重试。">
    {list.error && <div className="preview-form-error" role="alert">{list.error}</div>}
    {list.error
      ? null
      : !list.data && list.loading
      ? <EmptyState title="正在读取清理记录" description="清理任务加载完成后会自动显示。" />
      : items.length === 0
        ? <EmptyState title="暂无清理任务" description="还没有素材进入对象存储清理流程。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="素材清理记录">
          <thead><tr><th>类型</th><th>任务</th><th>素材</th><th>状态</th><th>尝试</th><th>错误码</th><th>时间</th></tr></thead>
          <tbody>{items.map(item => <tr key={item.jobId}>
            <td>{MEDIA_GC_KIND_LABELS[item.kind] || item.kind}</td>
            <td><strong>{item.jobId}</strong></td>
            <td>{item.assetId}</td>
            <td><StatusPill tone={mediaConsumerTone(item.status)}>{mediaConsumerLabel(item.status)}</StatusPill></td>
            <td>{item.attemptCount}</td>
            <td>{item.lastErrorCode || '—'}</td>
            <td>{displayTime(item.createdAt)}</td>
          </tr>)}</tbody>
        </table></div>}
    <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="素材清理记录分页" />
  </Panel>
}

function MediaDeadlettersTab() {
  const [page, setPage] = useState(1)
  const [data, setData] = useState<MediaDeadletters | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    adminAPI.mediaDeadletters({ page, pageSize: PAGE_SIZE })
      .then(value => {
        if (!active) return
        if (value.consumer.items.length === 0 && value.gc.items.length === 0 && page > 1) {
          setPage(current => Math.max(1, current - 1))
          return
        }
        setData(value)
        setError('')
      })
      .catch(reason => { if (active) setError(requestErrorText(reason)) })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [page, version])

  const consumer = data?.consumer.items ?? []
  const gc = data?.gc.items ?? []
  const empty = consumer.length === 0 && gc.length === 0

  return <Panel eyebrow="DEAD LETTER" title="异常记录" description="失败且已停止重试的消费与清理任务，需要人工介入。">
    {error && <div className="preview-form-error" role="alert">{error}</div>}
    <div className="preview-toolbar">
      <span className="preview-toolbar__hint">
        消费异常 {data?.consumer.total ?? '—'} 条 · 清理异常 {data?.gc.total ?? '—'} 条{loading ? ' · 刷新中…' : ''}
      </span>
      <Button size="sm" onClick={() => setVersion(current => current + 1)}>刷新</Button>
    </div>
    {error
      ? null
      : !data && loading
      ? <EmptyState title="正在读取异常记录" description="死信任务加载完成后会自动显示。" />
      : empty
        ? <EmptyState title="没有异常记录" description="当前处理与清理任务均正常。" />
        : <>
          <div className="preview-toolbar"><span className="preview-toolbar__hint">消费事件死信</span></div>
          {consumer.length > 0 && <div className="preview-table-wrap"><table className="preview-table" aria-label="消费事件死信">
            <thead><tr><th>动作</th><th>聚合</th><th>消费状态</th><th>尝试</th><th>错误码</th><th>时间</th></tr></thead>
            <tbody>{consumer.map(item => <tr key={item.eventId}>
              <td><strong>{MEDIA_OPERATION_LABELS[item.operation] || item.operation}</strong></td>
              <td className="preview-cell-wrap">{item.aggregateId}</td>
              <td><StatusPill tone="danger">{mediaConsumerLabel(item.consumerStatus)}</StatusPill></td>
              <td>{item.attemptCount}</td>
              <td>{item.lastErrorCode || '—'}</td>
              <td>{displayTime(item.createdAt)}</td>
            </tr>)}</tbody>
          </table></div>}
          <div className="preview-toolbar"><span className="preview-toolbar__hint">清理任务死信</span></div>
          {gc.length > 0 && <div className="preview-table-wrap"><table className="preview-table" aria-label="清理任务死信">
            <thead><tr><th>类型</th><th>任务</th><th>素材</th><th>状态</th><th>尝试</th><th>错误码</th><th>时间</th></tr></thead>
            <tbody>{gc.map(item => <tr key={item.jobId}>
              <td>{MEDIA_GC_KIND_LABELS[item.kind] || item.kind}</td>
              <td><strong>{item.jobId}</strong></td>
              <td>{item.assetId}</td>
              <td><StatusPill tone="danger">{mediaConsumerLabel(item.status)}</StatusPill></td>
              <td>{item.attemptCount}</td>
              <td>{item.lastErrorCode || '—'}</td>
              <td>{displayTime(item.createdAt)}</td>
            </tr>)}</tbody>
          </table></div>}
        </>}
    <Pagination
      page={data?.consumer.page ?? page}
      pageSize={data?.consumer.pageSize ?? PAGE_SIZE}
      total={Math.max(data?.consumer.total ?? 0, data?.gc.total ?? 0)}
      onChange={setPage}
      label="素材异常记录分页"
    />
  </Panel>
}

/* ===== 审核中心 ===== */

type ReviewStatusFilter = ReviewTask['status'] | ''
type ReviewDecisionKind = 'approve' | 'reject' | 'override-approved' | 'override-rejected'

interface ReviewDialogState {
  task: ReviewTask
  decision: ReviewDecisionKind
}

function ReviewOperations() {
  const [status, setStatus] = useState<ReviewStatusFilter>('submitted')
  const [search, setSearch] = useState('')
  const [counts, setCounts] = useState<Partial<Record<ReviewTask['status'], number>> | null>(null)
  const [actionError, setActionError] = useState('')
  const [actingId, setActingId] = useState<number | null>(null)
  const [dialog, setDialog] = useState<ReviewDialogState | null>(null)
  const [reason, setReason] = useState('')
  const [dialogError, setDialogError] = useState('')
  const [busy, setBusy] = useState(false)
  const { notify } = usePreviewToast()
  const list = useRemotePage<ReviewTask>((page, pageSize) => adminAPI.reviews({ status, page, pageSize }))

  const loadCounts = useCallback(async () => {
    const statuses: Array<ReviewTask['status']> = ['submitted', 'reviewing', 'approved', 'rejected']
    const results = await Promise.allSettled(statuses.map(value => adminAPI.reviews({ status: value, page: 1, pageSize: 1 })))
    const next: Partial<Record<ReviewTask['status'], number>> = {}
    results.forEach((result, index) => {
      if (result.status === 'fulfilled') next[statuses[index]] = result.value.total
    })
    setCounts(next)
  }, [])
  useEffect(() => { void loadCounts() }, [loadCounts])

  const reloadAll = () => {
    list.reload()
    void loadCounts()
    setActionError('')
  }

  const changeStatus = (next: ReviewStatusFilter) => {
    setStatus(next)
    setSearch('')
    setActionError('')
    list.setPage(1)
    list.reload()
  }

  const openDecision = (task: ReviewTask, decision: ReviewDecisionKind) => {
    setDialog({ task, decision })
    setReason('')
    setDialogError('')
  }

  const startReview = async (task: ReviewTask) => {
    setActingId(task.id)
    setActionError('')
    try {
      await adminAPI.startReview(task.id)
      notify('已进入审核，可继续选择通过或拒绝')
      reloadAll()
    } catch (error) {
      setActionError(requestErrorText(error))
    } finally {
      setActingId(null)
    }
  }

  const submitDecision = async () => {
    if (!dialog) return
    const text = reason.trim()
    if (!text) {
      setDialogError(dialog.decision === 'reject' || dialog.decision === 'override-rejected' ? '请填写拒绝原因' : '请填写审核依据，便于后续追溯')
      return
    }
    setBusy(true)
    setDialogError('')
    let started = false
    try {
      if (dialog.decision === 'approve' || dialog.decision === 'reject') {
        if (dialog.task.status === 'submitted') {
          await adminAPI.startReview(dialog.task.id)
          started = true
        }
        await adminAPI.decideReview(dialog.task.id, dialog.decision, text)
        notify(dialog.decision === 'approve' ? '审核已通过，内容将进入公开流程' : '审核已拒绝，原因已记录', dialog.decision === 'approve' ? 'success' : 'info')
      } else {
        const target = dialog.decision === 'override-approved' ? 'approved' : 'rejected'
        await adminAPI.overrideReview(dialog.task.id, target, text)
        notify(target === 'approved' ? '任务已改判为通过' : '任务已改判为拒绝', 'info')
      }
      setDialog(null)
      reloadAll()
    } catch (error) {
      setDialogError(requestErrorText(error))
      // 如果已经开始审核但结论提交失败，同步一次列表以反映“审核中”状态。
      if (started) {
        setDialog(current => current ? { ...current, task: { ...current.task, status: 'reviewing' } } : current)
        list.reload()
        void loadCounts()
      }
    } finally {
      setBusy(false)
    }
  }

  const items = list.data?.items ?? []
  const normalized = search.trim().toLowerCase()
  const visible = normalized
    ? items.filter(item => `${item.id} ${item.objectType} ${objectLabel(item.objectType)} ${item.objectId} ${riskText(item.risk)} ${item.reason} ${decisionSourceLabel(item.decisionSource)}`.toLowerCase().includes(normalized))
    : items

  const isReject = dialog?.decision === 'reject' || dialog?.decision === 'override-rejected'
  const dialogTitle = !dialog
    ? '审核处理'
    : dialog.decision === 'approve'
      ? '通过审核'
      : dialog.decision === 'reject'
        ? '拒绝审核'
        : dialog.decision === 'override-approved'
          ? '改判为通过'
          : '改判为拒绝'
  const reasonLabel = !dialog
    ? '审核依据'
    : dialog.decision === 'override-approved'
      ? '改判依据'
      : isReject
        ? '拒绝原因'
        : '审核依据'
  const confirmLabel = !dialog
    ? '确认'
    : dialog.decision === 'approve'
      ? '确认通过'
      : dialog.decision === 'reject'
        ? '确认拒绝'
        : dialog.decision === 'override-approved'
          ? '确认改判通过'
          : '确认改判拒绝'
  const warningCopy = !dialog
    ? { title: '', body: '' }
    : dialog.decision === 'approve'
      ? { title: '通过后会进入公开流程', body: '结论会写入统一审核流，审核依据将随任务保存。' }
      : dialog.decision === 'reject'
        ? { title: '拒绝后内容不会对外展示', body: '结论会写入统一审核流，拒绝原因将随任务保存。' }
        : { title: '改判会覆盖原审核结论', body: '改判依据会写入审核记录，供事后追溯。' }

  return <div>
    <PageHeader
      eyebrow={metaBySection.moderation.eyebrow}
      title={metaBySection.moderation.title}
      description={metaBySection.moderation.description}
      actions={<Button variant="primary" onClick={reloadAll}>刷新审核任务</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="待审核" value={counts?.submitted ?? '—'} meta="等待首次处理" tone="amber" />
      <StatCard label="审核中" value={counts?.reviewing ?? '—'} meta="正在核对资料" tone="accent" />
      <StatCard label="已通过" value={counts?.approved ?? '—'} meta="可按业务规则公开展示" tone="mint" />
      <StatCard label="已拒绝" value={counts?.rejected ?? '—'} meta="保留拒绝依据" tone="rose" />
    </section>
    <Panel
      eyebrow="REVIEW QUEUE"
      title="审核任务"
      description="通过、拒绝与改判都必须填写依据；状态筛选由接口返回，搜索仅过滤当前页。"
      actions={<SearchBox value={search} onChange={setSearch} placeholder="搜索任务编号、对象或风险备注" />}
    >
      {(actionError || list.error) && <div className="preview-form-error" role="alert">{actionError || list.error}</div>}
      <div className="preview-toolbar">
        <Segmented value={status} onChange={changeStatus} options={[
          { value: 'submitted', label: '待审核' },
          { value: 'reviewing', label: '审核中' },
          { value: 'approved', label: '已通过' },
          { value: 'rejected', label: '已拒绝' },
          { value: '', label: '全部' }
        ]} />
        <span className="preview-toolbar__hint">{list.data?.total ?? 0} 条任务{list.loading ? ' · 刷新中…' : ''}</span>
      </div>
      {list.error
        ? null
        : !list.data && list.loading
        ? <EmptyState title="正在读取审核任务" description="数据加载完成后会自动显示。" />
        : visible.length === 0
          ? <EmptyState title="当前没有审核任务" description="切换状态或调整搜索条件。" />
          : <div className="preview-review-list">
            {visible.map(item => <article key={item.id} className="preview-review-row">
              <span className="preview-review-row__kind">{shortKind(item.objectType)}</span>
              <div className="preview-review-row__copy">
                <div>
                  <strong>{objectLabel(item.objectType)} #{item.objectId}</strong>
                  <StatusPill tone={item.risk.trim() ? 'warning' : 'neutral'}>{riskText(item.risk)}</StatusPill>
                </div>
                <p>任务 #{item.id} · 决策来源：{decisionSourceLabel(item.decisionSource)} · {displayTime(item.createdAt)}</p>
                {waitingLabel(item.createdAt, item.status) && <p className="preview-review-row__wait">{waitingLabel(item.createdAt, item.status)}</p>}
                {item.reason && <small>决策原因：{item.reason}</small>}
              </div>
              <StatusPill tone={reviewStatusTone(item.status)}>{reviewStatusLabel(item.status)}</StatusPill>
              <div className="preview-row-actions">
                {item.status === 'submitted' && <Button size="sm" variant="ghost" disabled={actingId === item.id} onClick={() => void startReview(item)}>{actingId === item.id ? '处理中…' : '开始审核'}</Button>}
                {(item.status === 'submitted' || item.status === 'reviewing') && <>
                  <Button size="sm" variant="primary" onClick={() => openDecision(item, 'approve')}>通过</Button>
                  <Button size="sm" variant="danger" onClick={() => openDecision(item, 'reject')}>拒绝</Button>
                </>}
                {item.status === 'approved' && <Button size="sm" variant="ghost" onClick={() => openDecision(item, 'override-rejected')}>改判拒绝</Button>}
                {item.status === 'rejected' && <Button size="sm" variant="ghost" onClick={() => openDecision(item, 'override-approved')}>改判通过</Button>}
              </div>
            </article>)}
          </div>}
      <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="审核任务分页" />
    </Panel>
    <Modal
      open={Boolean(dialog)}
      title={dialogTitle}
      description={dialog ? `${objectLabel(dialog.task.objectType)} #${dialog.task.objectId} · 任务 #${dialog.task.id}` : ''}
      onClose={() => { if (!busy) setDialog(null) }}
      footer={<>
        <Button disabled={busy} onClick={() => { if (!busy) setDialog(null) }}>取消</Button>
        <Button variant={isReject ? 'danger' : 'primary'} disabled={busy} onClick={() => void submitDecision()}>{busy ? '提交中…' : confirmLabel}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>{warningCopy.title}</strong>
          <p>{warningCopy.body}</p>
        </div>
        <TextAreaField label={reasonLabel} required rows={5} maxLength={240} hint="最多 240 字，会随任务保存" value={reason} onChange={event => { setReason(event.target.value); setDialogError('') }} placeholder="请填写核对结果、风险判断或拒绝原因" />
        {dialogError && <div className="preview-form-error">{dialogError}</div>}
      </div>
    </Modal>
  </div>
}

/* ===== 社区内容 / 二手商品共用的下架确认 ===== */

function TakedownModal({
  kind,
  target,
  reason,
  error,
  busy,
  onReason,
  onClose,
  onConfirm
}: {
  kind: '帖子' | '商品'
  target: { id: number; title: string } | null
  reason: string
  error: string
  busy: boolean
  onReason: (value: string) => void
  onClose: () => void
  onConfirm: () => void
}) {
  return <Modal
    open={Boolean(target)}
    title={`下架${kind}`}
    description={target ? `${kind} #${target.id} · ${target.title || '未命名'}` : ''}
    onClose={() => { if (!busy) onClose() }}
    footer={<>
      <Button disabled={busy} onClick={() => { if (!busy) onClose() }}>取消</Button>
      <Button variant="danger" disabled={busy} onClick={onConfirm}>{busy ? '处理中…' : '确认下架'}</Button>
    </>}
  >
    <div className="preview-form-stack">
      <div className="preview-warning-box">
        <strong>下架后内容将从用户端消失</strong>
        <p>下架会立即从用户端隐藏内容，请填写可追溯的操作原因。</p>
      </div>
      <TextAreaField label="下架原因" required rows={4} maxLength={240} value={reason} onChange={event => onReason(event.target.value)} placeholder="请填写本次下架的原因" />
      {error && <div className="preview-form-error">{error}</div>}
    </div>
  </Modal>
}

/* ===== 社区内容 ===== */

type CommunityFilter = '全部' | '已公开' | '待审核' | '审核中' | '已拒绝' | '已下架'

const COMMUNITY_FILTER_STATUS: Record<Exclude<CommunityFilter, '全部'>, string> = {
  已公开: 'approved',
  待审核: 'submitted',
  审核中: 'reviewing',
  已拒绝: 'rejected',
  已下架: 'taken_down'
}

function CommunityOperations() {
  const list = useRemotePage<PostRow>((page, pageSize) => adminAPI.posts({ page, pageSize }))
  const [stats, setStats] = useState<Stats | null>(null)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<CommunityFilter>('全部')
  const [detailId, setDetailId] = useState<number | null>(null)
  const [takedownTarget, setTakedownTarget] = useState<PostRow | null>(null)
  const [takedownReason, setTakedownReason] = useState('')
  const [takedownError, setTakedownError] = useState('')
  const [busy, setBusy] = useState(false)
  const { notify } = usePreviewToast()

  useEffect(() => {
    let active = true
    adminAPI.stats()
      .then(value => { if (active) setStats(value) })
      .catch(() => {})
    return () => { active = false }
  }, [])

  const pageItems = list.data?.items ?? []
  const detail = pageItems.find(item => item.id === detailId) ?? null
  const normalized = query.trim().toLowerCase()
  const visible = pageItems.filter(item => {
    const byStatus = status === '全部' || item.reviewStatus === COMMUNITY_FILTER_STATUS[status]
    const haystack = `${item.id}${item.title}${item.author.nickname}${item.topic}`.toLowerCase()
    return byStatus && (!normalized || haystack.includes(normalized))
  })

  const openTakedown = (row: PostRow) => {
    setTakedownTarget(row)
    setTakedownReason('')
    setTakedownError('')
  }

  const confirmTakedown = async () => {
    if (!takedownTarget) return
    if (!takedownReason.trim()) {
      setTakedownError('请填写下架原因')
      return
    }
    setBusy(true)
    setTakedownError('')
    try {
      await adminAPI.takedownPost(takedownTarget.id, takedownReason.trim())
      notify('内容已下架，处理记录已保留', 'info')
      setTakedownTarget(null)
      list.reload()
    } catch (error) {
      setTakedownError(requestErrorText(error))
    } finally {
      setBusy(false)
    }
  }

  return <div>
    <PageHeader eyebrow={metaBySection.community.eyebrow} title={metaBySection.community.title} description={metaBySection.community.description} actions={<Button variant="primary" onClick={() => { setStatus('待审核'); list.setPage(1) }}>查看待审内容</Button>} />
    <section className="preview-stat-grid">
      <StatCard label="帖子总数" value={stats?.totals.posts ?? '—'} meta="社区全部内容" tone="accent" />
      <StatCard label="今日新增" value={stats?.today.posts ?? '—'} meta="今天提交的内容" tone="mint" />
      <StatCard label="本页待审核" value={pageItems.filter(item => item.reviewStatus === 'submitted' || item.reviewStatus === 'reviewing').length} meta="当前页统计" tone="amber" />
      <StatCard label="本页已下架" value={pageItems.filter(item => item.reviewStatus === 'taken_down').length} meta="当前页统计" tone="ink" />
    </section>
    <Panel eyebrow="COMMUNITY FEED" title="社区内容" description="审核状态来自统一审核流；下架需要填写原因并写入处理记录。" actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索标题、作者或话题" />}>
      {list.error && <div className="preview-form-error" role="alert">{list.error}</div>}
      <div className="preview-toolbar">
        <Segmented value={status} onChange={setStatus} options={[
          { value: '全部', label: '全部' },
          { value: '已公开', label: '已公开' },
          { value: '待审核', label: '待审核' },
          { value: '审核中', label: '审核中' },
          { value: '已拒绝', label: '已拒绝' },
          { value: '已下架', label: '已下架' }
        ]} />
        <span className="preview-toolbar__hint">本页展示 {visible.length} 条 · 全库共 {list.data?.total ?? 0} 条</span>
      </div>
      {!list.data && list.loading
        ? <EmptyState title="正在读取社区内容" description="内容加载完成后会自动显示。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="内容列表"><thead><tr><th>内容</th><th>图片</th><th>作者</th><th>话题</th><th>状态</th><th>时间</th><th /></tr></thead><tbody>
          {visible.map(item => <tr key={item.id} className="is-clickable" onClick={() => setDetailId(item.id)}>
            <td><button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); setDetailId(item.id) }}>{item.title || '无标题内容'}<small>#{item.id}</small></button></td>
            <td><RealImageCell url={item.images?.[0]?.url} label={item.title} /></td>
            <td>{item.author.nickname || '未知用户'}</td>
            <td>{item.topic || '未分类'}</td>
            <td><StatusPill>{POST_STATUS_LABELS[item.reviewStatus] || item.reviewStatus}</StatusPill></td>
            <td>{displayTime(item.createdAt)}</td>
            <td onClick={event => event.stopPropagation()}>{item.reviewStatus === 'approved'
              ? <Button size="sm" variant="danger" onClick={() => openTakedown(item)}>下架</Button>
              : item.reviewStatus === 'taken_down'
                ? <span className="preview-muted-action">已下架</span>
                : <span className="preview-muted-action">当前不可下架</span>}</td>
          </tr>)}
        </tbody></table>{visible.length === 0 && !list.error && <EmptyState title="当前页没有匹配内容" description="调整筛选条件，或翻到其他页查找。" />}</div>}
      <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="内容列表分页" />
    </Panel>

    <Drawer
      open={Boolean(detail)}
      onClose={() => setDetailId(null)}
      title={detail?.title || '帖子详情'}
      description={detail ? `${detail.author.nickname || '未知用户'} · ${detail.topic || '未分类'} · ${displayTime(detail.createdAt)}` : undefined}
      footer={detail && <>
        <Button onClick={() => setDetailId(null)}>关闭</Button>
        {detail.reviewStatus === 'approved' && <Button variant="danger" onClick={() => openTakedown(detail)}>下架内容</Button>}
      </>}
    >
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前状态</span><strong>{POST_STATUS_LABELS[detail.reviewStatus] || detail.reviewStatus}</strong></div>
          <StatusPill>{POST_STATUS_LABELS[detail.reviewStatus] || detail.reviewStatus}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>作者</span><strong>{detail.author.nickname || '未知用户'}</strong></div>
          <div><span>话题</span><strong>{detail.topic || '未分类'}</strong></div>
          <div><span>图片</span><strong>{detail.images?.length ?? 0} 张</strong></div>
          <div><span>点赞</span><strong>{detail.likedCount ?? 0}</strong></div>
          <div><span>评论</span><strong>{detail.commentCount ?? 0} 条</strong></div>
          <div><span>收藏</span><strong>{detail.favoriteCount ?? 0}</strong></div>
        </div>
        {detail.tags && detail.tags.length > 0 && <div className="preview-note-block"><span>标签</span><p>{detail.tags.join('、')}</p></div>}
        <div className="preview-note-block"><span>图片（{detail.images?.length ?? 0}）</span><RealImageStrip urls={(detail.images ?? []).map(image => image.url)} label={detail.title} /></div>
        <div className="preview-note-block"><span>正文</span><p>{detail.content || '（该内容没有正文）'}</p></div>
      </div>}
    </Drawer>

    <TakedownModal
      kind="帖子"
      target={takedownTarget}
      reason={takedownReason}
      error={takedownError}
      busy={busy}
      onReason={value => { setTakedownReason(value); setTakedownError('') }}
      onClose={() => setTakedownTarget(null)}
      onConfirm={() => void confirmTakedown()}
    />
  </div>
}

/* ===== 二手商品 ===== */

type TradeFilter = '全部' | '出售中' | '已预留' | '已售出' | '已下架'

const TRADE_FILTER_STATUS: Record<Exclude<TradeFilter, '全部' | '已下架'>, string> = {
  出售中: 'selling',
  已预留: 'reserved',
  已售出: 'sold'
}

function TradeOperations() {
  const list = useRemotePage<ProductRow>((page, pageSize) => adminAPI.products({ page, pageSize }))
  const [stats, setStats] = useState<Stats | null>(null)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<TradeFilter>('全部')
  const [detailId, setDetailId] = useState<number | null>(null)
  const [takedownTarget, setTakedownTarget] = useState<ProductRow | null>(null)
  const [takedownReason, setTakedownReason] = useState('')
  const [takedownError, setTakedownError] = useState('')
  const [busy, setBusy] = useState(false)
  const { notify } = usePreviewToast()

  useEffect(() => {
    let active = true
    adminAPI.stats()
      .then(value => { if (active) setStats(value) })
      .catch(() => {})
    return () => { active = false }
  }, [])

  const pageItems = list.data?.items ?? []
  const detail = pageItems.find(item => item.id === detailId) ?? null
  const normalized = query.trim().toLowerCase()
  const visible = pageItems.filter(item => {
    const byStatus = status === '全部'
      ? true
      : status === '已下架'
        ? item.reviewStatus === 'taken_down'
        : item.reviewStatus !== 'taken_down' && item.status === TRADE_FILTER_STATUS[status]
    const haystack = `${item.id}${item.title}${item.seller.nickname}${item.category}`.toLowerCase()
    return byStatus && (!normalized || haystack.includes(normalized))
  })
  const sellingItems = pageItems.filter(item => item.status === 'selling' && item.reviewStatus !== 'taken_down')

  const openTakedown = (row: ProductRow) => {
    setTakedownTarget(row)
    setTakedownReason('')
    setTakedownError('')
  }

  const confirmTakedown = async () => {
    if (!takedownTarget) return
    if (!takedownReason.trim()) {
      setTakedownError('请填写下架原因')
      return
    }
    setBusy(true)
    setTakedownError('')
    try {
      await adminAPI.takedownProduct(takedownTarget.id, takedownReason.trim())
      notify('商品已下架', 'info')
      setTakedownTarget(null)
      list.reload()
    } catch (error) {
      setTakedownError(requestErrorText(error))
    } finally {
      setBusy(false)
    }
  }

  return <div>
    <PageHeader eyebrow={metaBySection.trade.eyebrow} title={metaBySection.trade.title} description={metaBySection.trade.description} actions={<Button variant="primary" onClick={() => { setStatus('出售中'); list.setPage(1) }}>查看出售中商品</Button>} />
    <section className="preview-stat-grid">
      <StatCard label="商品总数" value={stats?.totals.products ?? '—'} meta="二手市场全部商品" tone="accent" />
      <StatCard label="本页出售中" value={sellingItems.length} meta="当前页统计" tone="mint" />
      <StatCard label="本页已预留" value={pageItems.filter(item => item.status === 'reserved').length} meta="当前页统计" tone="amber" />
      <StatCard label="在售金额（本页）" value={money(sellingItems.reduce((sum, item) => sum + item.price, 0))} meta="当前页出售中商品标价合计" tone="ink" />
    </section>
    <Panel eyebrow="SECONDHAND MARKET" title="商品工作队列" description="审核状态与业务状态分离；下架需要填写原因，只影响展示。" actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索商品、卖家或分类" />}>
      {list.error && <div className="preview-form-error" role="alert">{list.error}</div>}
      <div className="preview-toolbar">
        <Segmented value={status} onChange={setStatus} options={[
          { value: '全部', label: '全部' },
          { value: '出售中', label: '出售中' },
          { value: '已预留', label: '已预留' },
          { value: '已售出', label: '已售出' },
          { value: '已下架', label: '已下架' }
        ]} />
        <span className="preview-toolbar__hint">本页展示 {visible.length} 条 · 全库共 {list.data?.total ?? 0} 条</span>
      </div>
      {!list.data && list.loading
        ? <EmptyState title="正在读取商品" description="商品数据加载完成后会自动显示。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="商品列表"><thead><tr><th>商品</th><th>图片</th><th>卖家</th><th>分类</th><th>价格</th><th>业务状态</th><th>审核状态</th><th>时间</th><th /></tr></thead><tbody>
          {visible.map(item => <tr key={item.id} className="is-clickable" onClick={() => setDetailId(item.id)}>
            <td><button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); setDetailId(item.id) }}>{item.title || '无标题商品'}<small>#{item.id}</small></button></td>
            <td><RealImageCell url={item.images?.[0]?.url} label={item.title} /></td>
            <td>{item.seller.nickname || '未知用户'}</td>
            <td>{item.category || '未分类'}</td>
            <td><strong>{money(item.price)}</strong></td>
            <td><StatusPill>{PRODUCT_BUSINESS_LABELS[item.status] || item.status}</StatusPill></td>
            <td><StatusPill>{PRODUCT_STATUS_LABELS[item.reviewStatus] || item.reviewStatus}</StatusPill></td>
            <td>{displayTime(item.createdAt)}</td>
            <td onClick={event => event.stopPropagation()}>{item.reviewStatus === 'approved'
              ? <Button size="sm" variant="ghost" onClick={() => openTakedown(item)}>下架</Button>
              : item.reviewStatus === 'taken_down'
                ? <span className="preview-muted-action">已下架</span>
                : <span className="preview-muted-action">当前不可下架</span>}</td>
          </tr>)}
        </tbody></table>{visible.length === 0 && !list.error && <EmptyState title="当前页没有匹配商品" description="调整筛选条件，或翻到其他页查找。" />}</div>}
      <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="商品列表分页" />
    </Panel>

    <Drawer
      open={Boolean(detail)}
      onClose={() => setDetailId(null)}
      title={detail?.title || '商品详情'}
      description={detail ? `${detail.seller.nickname || '未知用户'} · ${detail.category || '未分类'} · ${displayTime(detail.createdAt)}` : undefined}
      footer={detail && <>
        <Button onClick={() => setDetailId(null)}>关闭</Button>
        {detail.reviewStatus === 'approved' && <Button variant="danger" onClick={() => openTakedown(detail)}>下架商品</Button>}
      </>}
    >
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前审核状态</span><strong>{PRODUCT_STATUS_LABELS[detail.reviewStatus] || detail.reviewStatus}</strong></div>
          <StatusPill>{PRODUCT_STATUS_LABELS[detail.reviewStatus] || detail.reviewStatus}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>卖家</span><strong>{detail.seller.nickname || '未知用户'}</strong></div>
          <div><span>分类</span><strong>{detail.category || '未分类'}</strong></div>
          <div><span>业务状态</span><strong>{PRODUCT_BUSINESS_LABELS[detail.status] || detail.status}</strong></div>
          <div><span>价格</span><strong>{money(detail.price)}</strong></div>
          <div><span>原价</span><strong>{money(detail.originalPrice)}</strong></div>
          <div><span>成色</span><strong>{detail.condition || '—'}</strong></div>
          <div><span>校区</span><strong>{detail.campus || '—'}</strong></div>
          <div><span>图片</span><strong>{detail.images?.length ?? 0} 张</strong></div>
          <div><span>收藏</span><strong>{detail.favoriteCount ?? 0}</strong></div>
          <div><span>发布时间</span><strong>{displayTime(detail.createdAt)}</strong></div>
        </div>
        <div className="preview-note-block"><span>图片（{detail.images?.length ?? 0}）</span><RealImageStrip urls={(detail.images ?? []).map(image => image.url)} label={detail.title} /></div>
        <div className="preview-note-block"><span>商品描述</span><p>{detail.description || '（无描述）'}</p></div>
      </div>}
    </Drawer>

    <TakedownModal
      kind="商品"
      target={takedownTarget}
      reason={takedownReason}
      error={takedownError}
      busy={busy}
      onReason={value => { setTakedownReason(value); setTakedownError('') }}
      onClose={() => setTakedownTarget(null)}
      onConfirm={() => void confirmTakedown()}
    />
  </div>
}

/* ===== 举报处理 ===== */

type ReportFilter = '全部' | ReviewTask['status']

function ReportOperations() {
  const list = useRemotePage<ReportRow>((page, pageSize) => adminAPI.reports({ page, pageSize }))
  const [filter, setFilter] = useState<ReportFilter>('全部')
  const [detailId, setDetailId] = useState<number | null>(null)
  const [detail, setDetail] = useState<ReportDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')

  useEffect(() => {
    if (detailId === null) return
    let active = true
    setDetail(null)
    setDetailLoading(true)
    setDetailError('')
    adminAPI.reportDetail(detailId)
      .then(value => { if (active) setDetail(value) })
      .catch(error => { if (active) setDetailError(requestErrorText(error)) })
      .finally(() => { if (active) setDetailLoading(false) })
    return () => { active = false }
  }, [detailId])

  const pageItems = list.data?.items ?? []
  const visible = pageItems.filter(item => filter === '全部' || item.reviewStatus === filter)

  return <div>
    <PageHeader eyebrow={metaBySection.reports.eyebrow} title={metaBySection.reports.title} description={metaBySection.reports.description} actions={<Button variant="primary" onClick={() => { setFilter('submitted'); list.setPage(1) }}>处理待办举报</Button>} />
    <section className="preview-stat-grid">
      <StatCard label="举报总数" value={list.data?.total ?? '—'} meta="全部举报记录" tone="accent" />
      <StatCard label="本页待审核" value={pageItems.filter(item => item.reviewStatus === 'submitted').length} meta="当前页统计" tone="rose" />
      <StatCard label="本页审核中" value={pageItems.filter(item => item.reviewStatus === 'reviewing').length} meta="当前页统计" tone="amber" />
      <StatCard label="本页已结案" value={pageItems.filter(item => item.reviewStatus === 'approved' || item.reviewStatus === 'rejected').length} meta="含通过或拒绝结论" tone="mint" />
    </section>
    <Panel eyebrow="SAFETY QUEUE" title="举报处理" description="举报处置沿用统一审核流；本页只读查看举报内容与最终结论。">
      {list.error && <div className="preview-form-error" role="alert">{list.error}</div>}
      <div className="preview-toolbar">
        <Segmented value={filter} onChange={setFilter} options={[
          { value: '全部', label: '全部' },
          { value: 'submitted', label: '待审核' },
          { value: 'reviewing', label: '审核中' },
          { value: 'approved', label: '已通过' },
          { value: 'rejected', label: '已拒绝' }
        ]} />
        <span className="preview-toolbar__hint">本页展示 {visible.length} 条 · 全库共 {list.data?.total ?? 0} 条</span>
      </div>
      {!list.data && list.loading
        ? <EmptyState title="正在读取举报" description="举报数据加载完成后会自动显示。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="举报列表"><thead><tr><th>举报目标</th><th>原因</th><th>举报人</th><th>审核状态</th><th>时间</th><th /></tr></thead><tbody>
          {visible.map(item => <tr key={item.id} className="is-clickable" onClick={() => setDetailId(item.id)}>
            <td><button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); setDetailId(item.id) }}>{reportTargetLabel(item.targetType)} #{item.targetId}<small>举报 #{item.id}</small></button></td>
            <td className="preview-cell-wrap">{item.reason || '未填写原因'}</td>
            <td>{item.reporterId ? `用户 #${item.reporterId}` : '匿名'}</td>
            <td><StatusPill tone={reviewStatusTone(item.reviewStatus)}>{reviewStatusLabel(item.reviewStatus)}</StatusPill></td>
            <td>{displayTime(item.createdAt)}</td>
            <td onClick={event => event.stopPropagation()}><Button size="sm" variant="primary" onClick={() => setDetailId(item.id)}>查看详情</Button></td>
          </tr>)}
        </tbody></table>{visible.length === 0 && !list.error && <EmptyState title="当前页没有举报" description="切换状态后再试。" />}</div>}
      <Pagination page={list.data?.page ?? list.page} pageSize={list.data?.pageSize ?? PAGE_SIZE} total={list.data?.total ?? 0} onChange={list.setPage} label="举报列表分页" />
    </Panel>

    <Drawer
      open={detailId !== null}
      onClose={() => setDetailId(null)}
      title={detailId !== null ? `举报 #${detailId}` : '举报详情'}
      description={detail ? `${reportTargetLabel(detail.report.targetType)} #${detail.report.targetId} · ${displayTime(detail.report.createdAt)}` : undefined}
      footer={<Button onClick={() => setDetailId(null)}>关闭</Button>}
    >
      {detailLoading && !detail && <EmptyState title="正在读取举报详情" description="举报内容与统一审核结论会一并展示。" />}
      {detailError && <div className="preview-form-error" role="alert">{detailError}</div>}
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前审核状态</span><strong>{reviewStatusLabel(detail.review?.status || detail.report.reviewStatus)}</strong></div>
          <StatusPill tone={reviewStatusTone(detail.review?.status || detail.report.reviewStatus)}>{reviewStatusLabel(detail.review?.status || detail.report.reviewStatus)}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>举报目标</span><strong>{reportTargetLabel(detail.report.targetType)} #{detail.report.targetId}</strong></div>
          <div><span>举报人</span><strong>{detail.report.reporterId ? `用户 #${detail.report.reporterId}` : '匿名'}</strong></div>
          <div><span>提交时间</span><strong>{displayTime(detail.report.createdAt)}</strong></div>
          <div><span>最后更新</span><strong>{displayTime(detail.report.updatedAt)}</strong></div>
        </div>
        <div className="preview-note-block"><span>举报原因</span><p>{detail.report.reason || '未填写原因'}</p></div>
        {detail.report.detail && <div className="preview-note-block"><span>补充说明</span><p>{detail.report.detail}</p></div>}
        <div className="preview-note-block">
          <span>统一审核结论</span>
          {detail.review
            ? <>
              <p>状态：{reviewStatusLabel(detail.review.status)} · 决策来源：{decisionSourceLabel(detail.review.decisionSource)}</p>
              <p>处理依据：{detail.review.reason || '未填写'}</p>
              <p>处理时间：{displayTime(detail.review.updatedAt || detail.review.createdAt)}</p>
            </>
            : <p>该举报尚未进入统一审核流。</p>}
        </div>
      </div>}
    </Drawer>
  </div>
}
