export function isMiniProgramJumpCancelled(error: unknown): boolean {
  const message = typeof error === 'object' && error !== null && 'errMsg' in error
    ? String(error.errMsg) : error instanceof Error ? error.message : String(error || '')
  return /cancel|用户取消/i.test(message)
}
