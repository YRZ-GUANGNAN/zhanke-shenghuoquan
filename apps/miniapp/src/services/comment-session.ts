import type { CommunityThreadComment, UserProfile } from '../types'
import type { LikeState, PageResult } from './b-community'

export interface CommentPageState {
  items: CommunityThreadComment[]
  page: number
  total: number
  hasMore: boolean
  loading: boolean
  error: string
  expanded: boolean
}

export interface CommentSessionState {
  publicCount?: number
  roots: CommentPageState
  replies: Record<string, CommentPageState>
  draft: string
  target: CommunityThreadComment | null
  submitting: boolean
  likeBusy: Record<string, boolean>
  error: string
  notice: string
}

export interface CommentSessionAPI {
  roots(postId: string, page: number): Promise<PageResult<CommunityThreadComment>>
  replies(postId: string, rootId: string, page: number): Promise<PageResult<CommunityThreadComment>>
  submit(postId: string, content: string, key: string, targetId?: string): Promise<CommunityThreadComment>
  like(postId: string, id: string, liked: boolean): Promise<LikeState>
  key(): string
}

const emptyPage = (): CommentPageState => ({ items: [], page: 0, total: 0, hasMore: false, loading: false, error: '', expanded: false })
const merge = (current: CommunityThreadComment[], incoming: CommunityThreadComment[]) => {
  const values = new Map(current.map(value => [value.id, value]))
  incoming.forEach(value => values.set(value.id, value))
  return [...values.values()].sort((a, b) => Number(a.id) - Number(b.id))
}

// A page/account owns one session. Disposing it invalidates every in-flight reply.
export class CommentSession {
  private alive = true
  private listeners = new Set<() => void>()
  private submission: { signature: string; key: string } | null = null
  private likeOverrides = new Map<string, LikeState>()
  private local = new Map<string, CommunityThreadComment>()
  private state: CommentSessionState = { roots: emptyPage(), replies: {}, draft: '', target: null, submitting: false, likeBusy: {}, error: '', notice: '' }

  constructor(private postId: string, private profile: Pick<UserProfile, 'id' | 'nickname' | 'avatar'>, private api: CommentSessionAPI) {}

  snapshot = () => this.state
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener) } }
  dispose = () => { this.alive = false; this.listeners.clear() }
  private update(patch: Partial<CommentSessionState>) {
    if (!this.alive) return
    this.state = { ...this.state, ...patch }
    this.listeners.forEach(listener => listener())
  }
  private page(rootId?: string) { return rootId ? this.state.replies[rootId] ?? emptyPage() : this.state.roots }
  private updatePage(rootId: string | undefined, patch: Partial<CommentPageState>) {
    const value = { ...this.page(rootId), ...patch }
    this.update(rootId ? { replies: { ...this.state.replies, [rootId]: value } } : { roots: value })
  }
  setDraft = (draft: string) => {
    if (this.state.submitting) return
    if (draft !== this.state.draft) this.submission = null
    this.update({ draft, notice: '', error: '' })
  }
  replyTo = (target: CommunityThreadComment | null) => {
    if (this.state.submitting || (target && target.reviewStatus !== 'approved')) return
    if (target?.id !== this.state.target?.id) this.submission = null
    this.update({ target, notice: '', error: '' })
  }
  collapse = (rootId: string) => this.updatePage(rootId, { expanded: false })
  expand = async (rootId: string) => {
    this.updatePage(rootId, { expanded: true })
    if (this.page(rootId).page === 0) await this.load(rootId)
  }
  load = async (rootId?: string, refresh = false) => {
    const current = this.page(rootId)
    if (!this.alive || current.loading || this.state.submitting) return
    if (!refresh && current.page > 0 && !current.hasMore) return
    const page = refresh ? 1 : current.page + 1
    this.updatePage(rootId, { loading: true, error: '' })
    try {
      const result = rootId ? await this.api.replies(this.postId, rootId, page) : await this.api.roots(this.postId, page)
      if (!this.alive) return
      if (result.publicCount !== undefined) this.update({ publicCount: result.publicCount })
      result.items.forEach(value => this.local.delete(value.id))
      // An older list response cannot overwrite a confirmed like operation.
      const incoming = result.items.map(value => {
        const override = this.likeOverrides.get(value.id)
        return override ? { ...value, ...override } : value
      })
      const local = [...this.local.values()].filter(value => value.parentCommentId === (rootId ?? '0'))
      const items = merge(refresh ? [] : this.page(rootId).items, merge(incoming, local))
      this.updatePage(rootId, { items, page: result.page, total: result.total, hasMore: result.page * result.pageSize < result.total })
      if (rootId) this.patchComment(rootId, { replyCount: result.total })
    } catch {
      if (this.alive) this.updatePage(rootId, { error: '暂时无法读取，点击重试' })
    } finally {
      if (this.alive) this.updatePage(rootId, { loading: false })
    }
  }
  private patchComment(id: string, patch: Partial<CommunityThreadComment>) {
    const patchPage = (page: CommentPageState) => ({ ...page, items: page.items.map(value => value.id === id ? { ...value, ...patch } : value) })
    const replies: Record<string, CommentPageState> = {}
    Object.keys(this.state.replies).forEach(key => { replies[key] = patchPage(this.state.replies[key]) })
    this.update({ roots: patchPage(this.state.roots), replies })
  }
  private find(id: string) {
    const root = this.state.roots.items.find(value => value.id === id)
    if (root) return root
    for (const page of Object.values(this.state.replies)) {
      const reply = page.items.find(value => value.id === id)
      if (reply) return reply
    }
    return undefined
  }
  like = async (id: string) => {
    const comment = this.find(id)
    if (!this.alive || !comment || comment.reviewStatus !== 'approved' || comment.liked === undefined || this.state.likeBusy[id]) return
    this.update({ likeBusy: { ...this.state.likeBusy, [id]: true }, error: '', notice: '' })
    try {
      const state = await this.api.like(this.postId, id, !comment.liked)
      if (!this.alive) return
      this.likeOverrides.set(id, state)
      this.patchComment(id, state)
    } catch {
      this.update({ error: '点赞未确认，请重试；已确认的状态会保留' })
    } finally {
      if (this.alive) this.update({ likeBusy: { ...this.state.likeBusy, [id]: false } })
    }
  }
  submit = async () => {
    const content = this.state.draft.trim()
    if (!this.alive || this.state.submitting) return
    if (!content || Array.from(content).length > 1000) { this.update({ error: '评论需填写 1–1000 字' }); return }
    const target = this.state.target
    const signature = JSON.stringify([this.postId, target?.id ?? '', content])
    if (this.submission?.signature !== signature) this.submission = { signature, key: this.api.key() }
    this.update({ submitting: true, error: '', notice: '' })
    try {
      const raw = await this.api.submit(this.postId, content, this.submission.key, target?.id)
      if (!this.alive) return
      if (!raw.id || raw.parentCommentId !== (target ? (target.parentCommentId === '0' ? target.id : target.parentCommentId) : '0')) {
        throw new Error('评论回执归属不一致')
      }
      const value = { ...raw, author: this.profile.nickname || '我', avatar: this.profile.avatar, replyToAuthor: target?.author ?? raw.replyToAuthor }
      const rootId = value.parentCommentId !== '0' ? value.parentCommentId : undefined
      const existed = Boolean(this.find(value.id))
      this.local.set(value.id, value)
      this.updatePage(rootId, { items: merge(this.page(rootId).items, [value]), expanded: true })
      if (rootId && !existed) {
        const root = this.find(rootId)
        if (root) this.patchComment(rootId, { replyCount: root.replyCount + 1 })
      }
      this.submission = null
      this.update({ draft: '', target: null, notice: value.reviewStatus === 'approved' ? '已发送' : '已提交审核，仅你可见' })
      // A reply to a collapsed thread must still load its earlier replies.
      if (rootId && this.page(rootId).page === 0) {
        this.update({ submitting: false })
        void this.load(rootId)
      }
    } catch {
      // Keep text, target and the same idempotency key after uncertain outcomes.
      this.update({ error: '发送未完成，文字已保留，可重试' })
    } finally {
      this.update({ submitting: false })
    }
  }
}
