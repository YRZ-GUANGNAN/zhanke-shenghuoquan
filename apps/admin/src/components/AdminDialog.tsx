import { useEffect, useRef } from 'react'
import { AdminButton } from './AdminButton'
import { AdminIcon } from './AdminIcon'

interface AdminDialogProps {
  open: boolean; title: string; description: string; confirmLabel: string; cancelLabel?: string; tone?: 'default' | 'danger'; inputLabel?: string; placeholder?: string; value?: string; error?: string; busy?: boolean; notifyOnly?: boolean
  onValueChange?: (value: string) => void; onConfirm: () => void; onCancel: () => void
}
export function AdminDialog({ open, title, description, confirmLabel, cancelLabel = '取消', tone = 'default', inputLabel, placeholder, value = '', error, busy = false, notifyOnly = false, onValueChange, onConfirm, onCancel }: AdminDialogProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const confirmRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!open) return
    const timer = window.setTimeout(() => (inputRef.current || confirmRef.current)?.focus(), 0)
    const handleKey = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) onCancel() }
    document.addEventListener('keydown', handleKey)
    return () => { window.clearTimeout(timer); document.removeEventListener('keydown', handleKey) }
  }, [open, busy, onCancel])
  if (!open) return null
  return <div className='admin-dialog-layer'>
    <button className='admin-dialog-backdrop' aria-label='关闭对话框' disabled={busy} onClick={onCancel} />
    <section className='admin-dialog' role='dialog' aria-modal='true' aria-labelledby='admin-dialog-title'>
      <header className='admin-dialog__header'><div><span className={`admin-dialog__tone admin-dialog__tone--${tone}`} aria-hidden='true' /><h2 id='admin-dialog-title'>{title}</h2></div>{!busy && <button className='admin-dialog__close' aria-label='关闭' onClick={onCancel}><AdminIcon name='close' /></button>}</header>
      <p className='admin-dialog__description'>{description}</p>
      {inputLabel && <label className={`admin-field ${error ? 'admin-field--error' : ''}`}><span className='admin-field__label'>{inputLabel}</span><input ref={inputRef} className='admin-control' value={value} placeholder={placeholder} onChange={(event) => onValueChange?.(event.target.value)} aria-invalid={Boolean(error)} />{error && <span className='admin-field__error' role='alert'>{error}</span>}</label>}
      <footer className='admin-dialog__actions'>{!notifyOnly && <AdminButton tone='secondary' disabled={busy} onClick={onCancel}>{cancelLabel}</AdminButton>}<AdminButton ref={confirmRef} tone={tone === 'danger' ? 'danger' : 'primary'} loading={busy} onClick={onConfirm}>{confirmLabel}</AdminButton></footer>
    </section>
  </div>
}
