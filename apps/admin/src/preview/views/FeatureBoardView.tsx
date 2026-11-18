import { useCallback, useEffect, useRef, useState } from 'react'
import type { AdminFeatureSuggestion, AuthorNoteRow, FeatureCommentRow, FeatureSuggestionStatus } from '../../types'
import { adminAPI } from '../../services/f-admin'
import {
  Button,
  EmptyState,
  ErrorNote,
  PageHeader,
  Pagination,
  Panel,
  Segmented,
  StatusPill,
  TextAreaField,
  usePreviewToast
} from '../ui'

const PAGE_SIZE = 8

const statusLabel: Record<FeatureSuggestionStatus, string> = { open: '待处理', accepted: '已采纳', closed: '已关闭' }
type StatusFilter = 'all' | FeatureSuggestionStatus
const filterOptions: Array<{ value: StatusFilter; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'open', label: '待处理' },
  { value: 'accepted', label: '已采纳' },
  { value: 'closed', label: '已关闭' }
]

function formatDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function FeatureBoardView() {
  const { notify } = usePreviewToast()
  const [filter, setFilter] = useState<StatusFilter>('all')
  const [page, setPage] = useState(1)
  const [items, setItems] = useState<AdminFeatureSuggestion[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')
  const [notes, setNotes] = useState<AuthorNoteRow[] | null>(null)
  const [noteDraft, setNoteDraft] = useState('')
  const [noteBusy, setNoteBusy] = useState(false)
  const [threadId, setThreadId] = useState('')
  const [threadComments, setThreadComments] = useState<FeatureCommentRow[]>([])
  const [threadLoading, setThreadLoading] = useState(false)
  const [replyDraft, setReplyDraft] = useState('')
  const [replyBusy, setReplyBusy] = useState(false)
  const listSeq = useRef(0)
  const threadSeq = useRef(0)
  const threadIdRef = useRef('')

  const load = useCallback(async (nextFilter: StatusFilter, nextPage: number) => {
    const seq = ++listSeq.current
    setLoading(true)
    try {
      const result = await adminAPI.featureSuggestions({ status: nextFilter === 'all' ? '' : nextFilter, page: nextPage, pageSize: PAGE_SIZE })
      if (listSeq.current !== seq) return
      if (result.items.length === 0 && nextPage > 1 && result.total > 0) {
        // 当前页已空（数据被处理或状态迁移），回到第一页重新加载。
        setPage(1)
        return
      }
      setItems(result.items)
      setTotal(result.total)
      setError('')
    } catch (reason) {
      if (listSeq.current !== seq) return
      setError(reason instanceof Error ? reason.message : '功能建议加载失败')
    } finally {
      if (listSeq.current === seq) setLoading(false)
    }
  }, [])

  const loadNotes = useCallback(async () => {
    try {
      setNotes(await adminAPI.authorNotes())
    } catch {
      setNotes([])
    }
  }, [])

  useEffect(() => { void load(filter, page) }, [load, filter, page])
  useEffect(() => { void loadNotes() }, [loadNotes])

  const changeStatus = async (item: AdminFeatureSuggestion, status: FeatureSuggestionStatus) => {
    if (busy) return
    setBusy(item.id)
    try {
      await adminAPI.setFeatureSuggestionStatus(item.id, status)
      notify(`建议已标记为「${statusLabel[status]}」`)
      await load(filter, page)
    } catch (reason) {
      notify(reason instanceof Error ? reason.message : '操作失败', 'danger')
    } finally {
      setBusy('')
    }
  }

  const toggleThread = async (item: AdminFeatureSuggestion) => {
    if (threadId === item.id) {
      threadSeq.current += 1
      threadIdRef.current = ''
      setThreadId('')
      setThreadComments([])
      setReplyDraft('')
      return
    }
    threadIdRef.current = item.id
    setThreadId(item.id)
    setThreadComments([])
    setReplyDraft('')
    const seq = ++threadSeq.current
    setThreadLoading(true)
    try {
      const page = await adminAPI.featureSuggestionComments(item.id)
      if (threadSeq.current !== seq) return
      setThreadComments(page.items)
    } catch (reason) {
      if (threadSeq.current === seq) notify(reason instanceof Error ? reason.message : '对话加载失败', 'danger')
    } finally {
      if (threadSeq.current === seq) setThreadLoading(false)
    }
  }

  const sendReply = async (item: AdminFeatureSuggestion) => {
    const content = replyDraft.trim()
    if (!content || replyBusy) return
    setReplyBusy(true)
    try {
      const comment = await adminAPI.replyFeatureSuggestion(item.id, content)
      setItems((current) => current.map((value) => value.id === item.id ? { ...value, commentCount: value.commentCount + 1 } : value))
      if (threadIdRef.current === item.id) {
        setThreadComments((current) => [...current, comment])
        setReplyDraft('')
      }
      notify('回复已发送')
    } catch (reason) {
      notify(reason instanceof Error ? reason.message : '回复失败', 'danger')
    } finally {
      setReplyBusy(false)
    }
  }

  const publishNote = async () => {
    const content = noteDraft.trim()
    if (noteBusy) return
    if (content.length < 2) {
      notify('请先写下内容', 'danger')
      return
    }
    setNoteBusy(true)
    try {
      await adminAPI.createAuthorNote(content)
      setNoteDraft('')
      notify('更新日记已发布')
      await loadNotes()
    } catch (reason) {
      notify(reason instanceof Error ? reason.message : '发布失败', 'danger')
    } finally {
      setNoteBusy(false)
    }
  }

  const removeNote = async (note: AuthorNoteRow) => {
    if (noteBusy) return
    setNoteBusy(true)
    try {
      await adminAPI.deleteAuthorNote(note.id)
      notify('已删除')
      await loadNotes()
    } catch (reason) {
      notify(reason instanceof Error ? reason.message : '删除失败', 'danger')
    } finally {
      setNoteBusy(false)
    }
  }

  return <>
    <PageHeader eyebrow="FEATURE BOARD" title="功能建议" description="小程序「功能意见与需求」的建议墙：用户提交建议并按支持数排行，这里负责采纳与关闭。" />
    <Panel
      title="建议列表"
      description="状态变化会立即反映到小程序建议墙：已采纳的建议带「已采纳」标记，已关闭的不再展示。"
      actions={<Segmented value={filter} options={filterOptions} onChange={(value) => { setFilter(value); setPage(1) }} />}
    >
      {error && <ErrorNote message={error} onRetry={() => void load(filter, page)} />}
      {loading ? <EmptyState title="正在读取建议" description="建议列表加载完成后会自动显示。" />
        : items.length === 0 ? <EmptyState title="暂无建议" description={filter === 'all' ? '用户提交功能建议后会出现在这里。' : '当前筛选下没有建议。'} />
        : <div className="preview-feedback-list">
          {items.map((item) => <article key={item.id}>
            <div className="preview-feedback-list__meta">
              <span><strong>支持 {item.supportCount} 人</strong><small>{formatDate(item.createdAt)} · 用户 #{item.userId}</small></span>
              <StatusPill>{statusLabel[item.status]}</StatusPill>
            </div>
            <p>{item.content}</p>
            <div className="preview-feedback-list__footer" style={{ justifyContent: 'flex-end', gap: 8 }}>
              <Button size="sm" variant="quiet" onClick={() => void toggleThread(item)}>{threadId === item.id ? '收起对话' : `对话 ${item.commentCount}`}</Button>
              {item.status !== 'accepted' && <Button size="sm" disabled={Boolean(busy)} onClick={() => void changeStatus(item, 'accepted')}>采纳</Button>}
              {item.status !== 'closed' && <Button size="sm" variant="quiet" disabled={Boolean(busy)} onClick={() => void changeStatus(item, 'closed')}>关闭</Button>}
              {item.status !== 'open' && <Button size="sm" variant="quiet" disabled={Boolean(busy)} onClick={() => void changeStatus(item, 'open')}>重新打开</Button>}
            </div>
            {threadId === item.id && <div className="preview-form-stack" style={{ marginTop: 12 }}>
              {threadLoading ? <p>正在加载对话…</p>
                : threadComments.length === 0 ? <p>还没有对话。</p>
                : threadComments.map((comment) => <div key={comment.id} style={{ padding: '8px 10px', borderRadius: 8, background: comment.isMaintainer ? '#e6f1fb' : '#f7f8fa' }}>
                  <div><strong style={{ fontSize: 12 }}>{comment.isMaintainer ? '作者' : '用户'}</strong><span style={{ fontSize: 12, color: '#93a2b4', marginLeft: 8 }}>{formatDate(comment.createdAt)}</span></div>
                  <p style={{ margin: '4px 0 0' }}>{comment.content}</p>
                </div>)}
              <TextAreaField label="作者回复" rows={2} maxLength={300} value={replyDraft} placeholder="以作者身份回复这条建议…" onChange={(event) => setReplyDraft(event.target.value)} />
              <div><Button size="sm" variant="primary" disabled={replyBusy} onClick={() => void sendReply(item)}>{replyBusy ? '发送中…' : '发送回复'}</Button></div>
            </div>}
          </article>)}
        </div>}
      <Pagination page={page} pageSize={PAGE_SIZE} total={total} onChange={setPage} label="建议分页" />
    </Panel>
    <Panel title="更新日记" description="发布后展示在小程序「关于 → 更新日记」页（以及建议墙底部），最新一条在最上面。">
      <div className="preview-form-stack">
        <TextAreaField
          label="内容"
          value={noteDraft}
          maxLength={1000}
          rows={4}
          placeholder="例如：这周修复了筛选问题；下一步计划做夜间模式…"
          onChange={(event) => setNoteDraft(event.target.value)}
        />
        <div><Button variant="primary" disabled={noteBusy} onClick={() => void publishNote()}>{noteBusy ? '发布中…' : '发布'}</Button></div>
      </div>
      {notes === null ? <EmptyState title="正在读取更新日记" description="加载完成后会自动显示。" />
        : notes.length === 0 ? <EmptyState title="还没有发布过更新日记" description="发布后会出现在小程序「关于 → 更新日记」页。" />
        : <div className="preview-feedback-list">
          {notes.map((note) => <article key={note.id}>
            <div className="preview-feedback-list__meta">
              <span><strong>{formatDate(note.createdAt)}</strong></span>
            </div>
            <p>{note.content}</p>
            <div className="preview-feedback-list__footer" style={{ justifyContent: 'flex-end' }}>
              <Button size="sm" variant="quiet" disabled={noteBusy} onClick={() => void removeNote(note)}>删除</Button>
            </div>
          </article>)}
        </div>}
    </Panel>
  </>
}
