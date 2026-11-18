// 取昵称/名称的首个字符（按 Unicode 码点，避免把 emoji 的代理对切成半个字符）。
export function userInitial(value: string | null | undefined, fallback: string): string {
  const trimmed = (value || '').trim()
  if (!trimmed) return fallback
  return [...trimmed][0] || fallback
}
