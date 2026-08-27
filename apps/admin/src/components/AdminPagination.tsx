import { AdminButton } from './AdminButton'

interface AdminPaginationProps {
  page: number
  pageSize: number
  total: number
  loading?: boolean
  onPageChange: (page: number) => void
}

export function AdminPagination({ page, pageSize, total, loading = false, onPageChange }: AdminPaginationProps) {
  const pages = Math.max(1, Math.ceil(total / pageSize))
  const safePage = Math.min(Math.max(page, 1), pages)
  return <nav className="admin-pagination" aria-label="列表分页">
    <AdminButton tone="ghost" disabled={loading || safePage <= 1} onClick={() => onPageChange(safePage - 1)}>上一页</AdminButton>
    <span><strong>{safePage}</strong> / {pages} 页</span>
    <small>共 {total} 条</small>
    <AdminButton tone="ghost" disabled={loading || safePage >= pages || total === 0} onClick={() => onPageChange(safePage + 1)}>下一页</AdminButton>
  </nav>
}
