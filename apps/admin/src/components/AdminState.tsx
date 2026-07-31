import { AdminButton } from './AdminButton'
interface AdminStateProps { kind: 'loading' | 'empty' | 'error' | 'forbidden'; title: string; description?: string; actionLabel?: string; onAction?: () => void }
export function AdminState({ kind, title, description, actionLabel, onAction }: AdminStateProps) {
  return <div className={`admin-state admin-state--${kind}`} role={kind === 'error' ? 'alert' : 'status'}><span className='admin-state__mark' aria-hidden='true' /><strong>{title}</strong>{description && <p>{description}</p>}{actionLabel && onAction && <AdminButton tone='secondary' onClick={onAction}>{actionLabel}</AdminButton>}</div>
}
