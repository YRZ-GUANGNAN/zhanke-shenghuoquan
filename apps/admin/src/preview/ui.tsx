import {
  Button as AntButton,
  Card,
  Drawer as AntDrawer,
  Empty,
  Input,
  Modal as AntModal,
  Progress,
  Segmented as AntSegmented,
  Statistic,
  Switch,
  Tag,
  message
} from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import {
  createContext,
  useContext,
  useMemo,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes
} from 'react'
import type { Tone } from './data'

export function cx(...values: Array<string | false | null | undefined>) {
  return values.filter(Boolean).join(' ')
}

const toneByLabel: Record<string, Tone> = {
  正常: 'success',
  已通过: 'success',
  已完成: 'success',
  已送达: 'success',
  营业中: 'success',
  测试成功: 'success',
  已解决: 'success',
  已公开: 'success',
  出售中: 'success',
  待审核: 'warning',
  待认证: 'warning',
  待处理: 'warning',
  待接单: 'warning',
  待确认: 'accent',
  处理中: 'info',
  审核中: 'info',
  配送中: 'info',
  发送中: 'info',
  已预留: 'accent',
  测试失败: 'danger',
  异常: 'danger',
  失败: 'danger',
  高风险: 'danger',
  已拒绝: 'danger',
  已下架: 'danger',
  停用: 'danger',
  不处理: 'neutral',
  禁言: 'danger',
  已取消: 'neutral',
  已跳过: 'neutral',
  休息中: 'neutral',
  未测试: 'neutral'
}

const toneToColor: Record<Tone, string> = {
  success: 'green',
  warning: 'gold',
  danger: 'red',
  info: 'blue',
  accent: 'geekblue',
  neutral: 'default'
}

export function StatusPill({ children, tone }: { children: ReactNode; tone?: Tone }) {
  const label = String(children)
  const resolved = tone || toneByLabel[label] || 'neutral'
  return <Tag color={toneToColor[resolved]} className="preview-tag">{children}</Tag>
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'quiet'
  size?: 'sm' | 'md'
}

export function Button({ variant = 'secondary', size = 'md', className, children, ...props }: ButtonProps) {
  const type = variant === 'primary'
    ? 'primary'
    : variant === 'danger'
      ? 'danger'
      : variant === 'quiet'
        ? 'text'
        : 'default'
  // antd 的 ghost 是给深色背景用的（白字白边），浅色页面会看不见；
  // 这里把 ghost 当作浅色描边按钮处理，不再透传 ghost 属性。
  return <AntButton
    {...props}
    type={type as never}
    size={size === 'sm' ? 'small' : 'middle'}
    className={cx('preview-button', variant === 'ghost' && 'preview-button--ghost', className)}
  >
    {children}
  </AntButton>
}

export function Panel({
  title,
  actions,
  className,
  children
}: {
  title?: string
  eyebrow?: string
  description?: string
  actions?: ReactNode
  className?: string
  children: ReactNode
}) {
  return <Card
    className={cx('preview-panel', className)}
    size="small"
    title={title}
    extra={actions}
    bordered
  >
    {children}
  </Card>
}

export function PageHeader({
  title,
  actions
}: {
  eyebrow?: string
  title: string
  description?: string
  actions?: ReactNode
}) {
  return <header className="preview-page-header">
    <h1>{title}</h1>
    {actions && <div className="preview-page-header__actions">{actions}</div>}
  </header>
}

export function StatCard({
  label,
  value,
  meta,
  trend,
  tone = 'accent',
  unit
}: {
  label: string
  value: ReactNode
  meta: ReactNode
  trend?: ReactNode
  tone?: 'accent' | 'mint' | 'amber' | 'rose' | 'ink'
  unit?: string
}) {
  return <Card className={`preview-stat preview-stat--${tone}`} size="small" bordered>
    <Statistic
      title={label}
      value={value as never}
      suffix={unit ? <span className="preview-stat__unit">{unit}</span> : undefined}
    />
    <div className="preview-stat__bottom">
      <span>{meta}</span>
      {trend && <em>{trend}</em>}
    </div>
  </Card>
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
  className
}: {
  value: T
  options: Array<{ value: T; label: string; badge?: number }>
  onChange: (value: T) => void
  className?: string
}) {
  return <AntSegmented
    className={cx('preview-segmented', className)}
    value={value}
    options={options.map(option => ({
      value: option.value,
      label: option.badge !== undefined ? `${option.label} ${option.badge}` : option.label
    }))}
    onChange={value => onChange(value as T)}
  />
}

export function SearchBox({
  value,
  onChange,
  placeholder = '搜索',
  label = '搜索'
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  label?: string
}) {
  return <Input
    className="preview-search"
    allowClear
    prefix={<SearchOutlined />}
    value={value}
    placeholder={placeholder}
    aria-label={label}
    onChange={event => onChange(event.target.value)}
  />
}

export function Field({
  label,
  hint,
  error,
  required,
  className,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  label: string
  hint?: string
  error?: string
  required?: boolean
}) {
  return <label className={cx('preview-field', className)}>
    <span>{label}{required && <b>*</b>}</span>
    <input {...props} aria-invalid={Boolean(error)} />
    {error ? <small className="preview-field__error">{error}</small> : hint ? <small>{hint}</small> : null}
  </label>
}

export function SelectField({
  label,
  children,
  className,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement> & { label: string }) {
  return <label className={cx('preview-field', className)}>
    <span>{label}</span>
    <select {...props}>{children}</select>
  </label>
}

export function TextAreaField({
  label,
  hint,
  required,
  className,
  ...props
}: TextareaHTMLAttributes<HTMLTextAreaElement> & {
  label: string
  hint?: string
  required?: boolean
}) {
  return <label className={cx('preview-field', className)}>
    <span>{label}{required && <b>*</b>}</span>
    <textarea {...props} />
    {hint && <small>{hint}</small>}
  </label>
}

export function Toggle({
  checked,
  onChange,
  label
}: {
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
}) {
  return <Switch checked={checked} onChange={onChange} aria-label={label} />
}

export function EmptyState({ title, action }: { title: string; description?: string; action?: ReactNode }) {
  return <Empty className="preview-empty" description={title}>
    {action}
  </Empty>
}

export function Modal({
  open,
  title,
  children,
  footer,
  onClose,
  width = 'md'
}: {
  open: boolean
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
  width?: 'sm' | 'md' | 'lg'
}) {
  const widths = { sm: 480, md: 640, lg: 900 }
  return <AntModal
    visible={open}
    title={title}
    footer={footer}
    width={widths[width]}
    centered
    destroyOnClose
    onCancel={onClose}
  >
    {children}
  </AntModal>
}

export function Drawer({
  open,
  title,
  children,
  footer,
  onClose
}: {
  open: boolean
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
}) {
  return <AntDrawer
    visible={open}
    title={title}
    width={520}
    footer={footer}
    destroyOnClose
    onClose={onClose}
  >
    {children}
  </AntDrawer>
}

type ToastTone = 'success' | 'danger' | 'info'
interface ToastContextValue {
  notify: (messageText: string, tone?: ToastTone) => void
}
const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const notify = useMemo(() => (messageText: string, tone: ToastTone = 'success') => {
    if (tone === 'danger') message.error(messageText)
    else if (tone === 'info') message.info(messageText)
    else message.success(messageText)
  }, [])
  return <ToastContext.Provider value={{ notify }}>{children}</ToastContext.Provider>
}

export function usePreviewToast() {
  const context = useContext(ToastContext)
  if (!context) throw new Error('usePreviewToast must be used inside ToastProvider')
  return context
}


// 积压时长统一口径：总览与审核列表共用，避免两处各算一套。
export function formatWait(createdAt: string): string {
  const started = new Date(createdAt).getTime()
  if (Number.isNaN(started)) return ''
  const minutes = Math.floor((Date.now() - started) / 60000)
  if (minutes < 1) return '不到 1 分钟'
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时`
  return `${Math.floor(hours / 24)} 天`
}

export function ProgressBar({ value, label }: { value: number; label: string }) {
  // 阈值配色：低于 90% 需要排查，90–95% 观察，95% 以上视为健康。
  const tone = value >= 95 ? 'healthy' : value >= 90 ? 'watch' : 'risk'
  return <Progress
    className={`preview-progress preview-progress--${tone}`}
    percent={value}
    showInfo={false}
    strokeWidth={6}
    aria-label={label}
  />
}
