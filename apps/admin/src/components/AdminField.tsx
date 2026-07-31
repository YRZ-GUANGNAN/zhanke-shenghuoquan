import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react'

type CommonProps = { label: string; description?: string; error?: string; required?: boolean; className?: string }
type InputProps = CommonProps & InputHTMLAttributes<HTMLInputElement> & { multiline?: false }
type TextareaProps = CommonProps & TextareaHTMLAttributes<HTMLTextAreaElement> & { multiline: true }
export type AdminFieldProps = InputProps | TextareaProps

export function AdminField(props: AdminFieldProps) {
  const { label, description, error, required, className = '', multiline, ...controlProps } = props
  const id = controlProps.id || `admin-field-${controlProps.name || label.replace(/\s+/g, '-')}`
  const describedBy = [description ? `${id}-description` : '', error ? `${id}-error` : ''].filter(Boolean).join(' ') || undefined
  return <label className={`admin-field ${error ? 'admin-field--error' : ''} ${className}`.trim()} htmlFor={id}>
    <span className='admin-field__label'>{label}{required && <span aria-hidden='true'> *</span>}</span>
    {multiline
      ? <textarea {...(controlProps as TextareaHTMLAttributes<HTMLTextAreaElement>)} id={id} className='admin-control admin-control--textarea' required={required} aria-invalid={Boolean(error)} aria-describedby={describedBy} />
      : <input {...(controlProps as InputHTMLAttributes<HTMLInputElement>)} id={id} className='admin-control' required={required} aria-invalid={Boolean(error)} aria-describedby={describedBy} />}
    {description && <span id={`${id}-description`} className='admin-field__description'>{description}</span>}
    {error && <span id={`${id}-error`} className='admin-field__error' role='alert'>{error}</span>}
  </label>
}
