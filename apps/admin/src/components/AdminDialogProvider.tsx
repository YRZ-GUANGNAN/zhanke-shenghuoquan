import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { AdminDialog } from './AdminDialog'

export interface ConfirmOptions { title: string; description: string; confirmLabel?: string; tone?: 'default' | 'danger' }
export interface PromptOptions extends ConfirmOptions { inputLabel: string; placeholder?: string; requiredMessage?: string; initialValue?: string }
export interface AdminDialogAPI { confirm(options: ConfirmOptions): Promise<boolean>; prompt(options: PromptOptions): Promise<string | null>; notify(options: Omit<ConfirmOptions, 'tone'>): Promise<void> }
type ActiveRequest = { kind: 'confirm'; options: ConfirmOptions; resolve: (value: boolean) => void } | { kind: 'prompt'; options: PromptOptions; resolve: (value: string | null) => void } | { kind: 'notify'; options: Omit<ConfirmOptions, 'tone'>; resolve: () => void }
const AdminDialogContext = createContext<AdminDialogAPI | null>(null)

export function AdminDialogProvider({ children }: { children: ReactNode }) {
  const [active, setActive] = useState<ActiveRequest | null>(null)
  const [value, setValue] = useState('')
  const [error, setError] = useState('')
  const settled = useRef(false)
  const open = useCallback((request: ActiveRequest) => { settled.current = false; setValue(request.kind === 'prompt' ? request.options.initialValue || '' : ''); setError(''); setActive(request) }, [])
  const close = useCallback(() => {
    if (!active || settled.current) return
    settled.current = true
    if (active.kind === 'confirm') active.resolve(false)
    else if (active.kind === 'prompt') active.resolve(null)
    else active.resolve()
    setActive(null)
  }, [active])
  const confirmCurrent = useCallback(() => {
    if (!active || settled.current) return
    if (active.kind === 'prompt') { const trimmed = value.trim(); if (!trimmed) { setError(active.options.requiredMessage || '请输入原因'); return }; settled.current = true; active.resolve(trimmed) }
    else if (active.kind === 'confirm') { settled.current = true; active.resolve(true) }
    else { settled.current = true; active.resolve() }
    setActive(null)
  }, [active, value])
  const api = useMemo<AdminDialogAPI>(() => ({
    confirm: (options) => new Promise((resolve) => open({ kind: 'confirm', options, resolve })),
    prompt: (options) => new Promise((resolve) => open({ kind: 'prompt', options, resolve })),
    notify: (options) => new Promise((resolve) => open({ kind: 'notify', options, resolve })),
  }), [open])
  const options = active?.options
  return <AdminDialogContext.Provider value={api}>
    {children}
    <AdminDialog
      open={Boolean(active)}
      title={options?.title || ''}
      description={options?.description || ''}
      confirmLabel={options?.confirmLabel || (active?.kind === 'notify' ? '知道了' : '确认')}
      tone={active && active.kind !== 'notify' ? active.options.tone : 'default'}
      inputLabel={active?.kind === 'prompt' ? active.options.inputLabel : undefined}
      placeholder={active?.kind === 'prompt' ? active.options.placeholder : undefined}
      value={value}
      error={error}
      notifyOnly={active?.kind === 'notify'}
      onValueChange={(next) => { setValue(next); setError('') }}
      onConfirm={confirmCurrent}
      onCancel={close}
    />
  </AdminDialogContext.Provider>
}
export function useAdminDialog(): AdminDialogAPI {
  const value = useContext(AdminDialogContext)
  if (!value) throw new Error('useAdminDialog must be used within AdminDialogProvider')
  return value
}
