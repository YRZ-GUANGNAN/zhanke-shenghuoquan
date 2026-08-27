import type { ReactNode } from 'react'

interface AdminPanelProps {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
  className?: string
}

export function AdminPanel({ title, description, actions, children, className = '' }: AdminPanelProps) {
  return <section className={`admin-panel ${className}`.trim()}>
    <header className="admin-panel__header">
      <div>
        <h2>{title}</h2>
        {description && <p>{description}</p>}
      </div>
      {actions && <div className="admin-panel__actions">{actions}</div>}
    </header>
    <div className="admin-panel__body">{children}</div>
  </section>
}
