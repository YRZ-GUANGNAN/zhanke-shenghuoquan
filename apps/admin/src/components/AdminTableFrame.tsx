import type { ReactNode } from 'react'

interface AdminTableFrameProps {
  label: string
  children: ReactNode
}

export function AdminTableFrame({ label, children }: AdminTableFrameProps) {
  return <div className="admin-table-frame" role="region" aria-label={label} tabIndex={0}>{children}</div>
}

