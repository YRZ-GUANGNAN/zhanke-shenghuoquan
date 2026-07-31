import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'

export type AdminButtonTone = 'primary' | 'secondary' | 'ghost' | 'danger' | 'text'
interface AdminButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> { tone?: AdminButtonTone; loading?: boolean; children: ReactNode }

export const AdminButton = forwardRef<HTMLButtonElement, AdminButtonProps>(function AdminButton({ tone = 'secondary', loading = false, disabled, children, className = '', type = 'button', ...props }, ref) {
  return <button {...props} ref={ref} type={type} className={`admin-button admin-button--${tone} ${className}`.trim()} disabled={disabled || loading} aria-busy={loading || undefined}>
    {loading && <span className='admin-button__spinner' aria-hidden='true' />}
    <span>{loading ? '处理中…' : children}</span>
  </button>
})
