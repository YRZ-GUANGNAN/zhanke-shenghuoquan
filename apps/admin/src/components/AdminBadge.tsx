import type { ReactNode } from 'react'
export type AdminBadgeTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'
export function AdminBadge({ tone = 'neutral', children }: { tone?: AdminBadgeTone; children: ReactNode }) { return <span className={`admin-badge admin-badge--${tone}`}>{children}</span> }
