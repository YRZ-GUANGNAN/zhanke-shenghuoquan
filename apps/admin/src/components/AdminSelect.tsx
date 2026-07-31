import type { SelectHTMLAttributes } from 'react'

interface AdminSelectProps extends SelectHTMLAttributes<HTMLSelectElement> { label: string; description?: string; error?: string; className?: string }
export function AdminSelect({ label, description, error, className = '', children, id, name, ...props }: AdminSelectProps) {
  const controlId = id || `admin-select-${name || label.replace(/\s+/g, '-')}`
  return <label className={`admin-field admin-select ${error ? 'admin-field--error' : ''} ${className}`.trim()} htmlFor={controlId}>
    <span className='admin-field__label'>{label}</span>
    <span className='admin-select__control'><select {...props} id={controlId} name={name} className='admin-control' aria-invalid={Boolean(error)}>{children}</select></span>
    {description && <span className='admin-field__description'>{description}</span>}
    {error && <span className='admin-field__error' role='alert'>{error}</span>}
  </label>
}
