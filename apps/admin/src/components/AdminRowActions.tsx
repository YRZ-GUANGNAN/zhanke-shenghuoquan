import type { ReactNode } from 'react'

export function AdminRowActions({ children }: { children: ReactNode }) {
  return <div className="admin-row-actions">{children}</div>
}
