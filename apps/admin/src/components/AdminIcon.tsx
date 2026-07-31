export type AdminIconName = 'media' | 'delivery' | 'dashboard' | 'users' | 'identities' | 'moderation' | 'community' | 'trade' | 'reports' | 'navigation' | 'search' | 'close' | 'refresh' | 'chevron-right'
interface AdminIconProps { name: AdminIconName; size?: number; className?: string }

const paths: Record<AdminIconName, JSX.Element> = {
  media: <><rect x='4' y='5' width='16' height='14' rx='2'/><path d='m7 15 3.5-3.5 3 3 2-2L21 18'/><circle cx='17.5' cy='9' r='1.5'/></>,
  delivery: <><path d='M3 7h11v9H3zM14 10h4l3 3v3h-7z'/><circle cx='7' cy='18' r='2'/><circle cx='18' cy='18' r='2'/></>,
  dashboard: <path d='M4 13h6V4H4zM14 20h6v-9h-6zM4 20h6v-3H4zM14 7h6V4h-6z'/>,
  users: <><circle cx='9' cy='8' r='3'/><circle cx='17' cy='9' r='2.5'/><path d='M3 20c.5-4 2.5-6 6-6s5.5 2 6 6M14 15c3-.5 5 1.2 6 4'/></>,
  identities: <><rect x='3' y='5' width='18' height='14' rx='2'/><circle cx='8' cy='11' r='2.5'/><path d='M5 17c.5-2 1.5-3 3-3s2.5 1 3 3M14 9h4M14 13h4'/></>,
  moderation: <><path d='M12 3 4 6v5c0 5 3 8 8 10 5-2 8-5 8-10V6z'/><path d='m8.5 12 2.2 2.2 4.8-5'/></>,
  community: <><path d='M4 5h16v12H9l-5 4z'/><path d='M8 9h8M8 13h5'/></>,
  trade: <><path d='M4 8h16l-1 12H5zM8 8a4 4 0 0 1 8 0'/></>,
  reports: <><path d='M12 3 3 20h18z'/><path d='M12 9v5M12 17h.01'/></>,
  navigation: <><circle cx='12' cy='12' r='9'/><path d='m15.5 8.5-2.2 5-4.8 2 2.2-5z'/></>,
  search: <><circle cx='10.5' cy='10.5' r='6.5'/><path d='m15.5 15.5 5 5'/></>,
  close: <path d='m6 6 12 12M18 6 6 18'/>,
  refresh: <><path d='M20 7v5h-5'/><path d='M19 12a7 7 0 1 0-1.5 5'/></>,
  'chevron-right': <path d='m9 5 7 7-7 7'/>,
}
export function AdminIcon({ name, size = 20, className = '' }: AdminIconProps) {
  return <svg className={`admin-icon ${className}`.trim()} width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='1.8' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>{paths[name]}</svg>
}
