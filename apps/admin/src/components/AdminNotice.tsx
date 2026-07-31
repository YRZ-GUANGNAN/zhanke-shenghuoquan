import type { ReactNode } from 'react'
export function AdminNotice({ tone = 'info', title, children }: { tone?: 'info' | 'success' | 'warning' | 'danger'; title?: string; children: ReactNode }) {
  return <div className={`admin-notice admin-notice--${tone}`} role={tone === 'danger' ? 'alert' : 'status'}>{title && <strong>{title}</strong>}<div>{children}</div></div>
}
