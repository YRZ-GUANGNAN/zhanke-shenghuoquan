import type { ReactNode } from 'react'

export function AdminToolbar({ summary, actions, children }: { summary?: ReactNode; actions?: ReactNode; children?: ReactNode }) {
  return <div className="admin-toolbar">
    <div className="admin-toolbar__filters">{children}</div>
    <div className="admin-toolbar__meta">
      {summary && <div className="admin-toolbar__summary">{summary}</div>}
      {actions && <div className="admin-toolbar__actions">{actions}</div>}
    </div>
  </div>
}
