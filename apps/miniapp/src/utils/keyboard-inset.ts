/**
 * 键盘避让的纯计算层。
 *
 * Input.onKeyboardHeightChange 给出的高度和小程序逻辑窗口高度的变化
 * 可能同时包含同一段键盘遮挡。这里把两种来源合并成一个剩余 inset，
 * 页面只消费这个值，避免在部分机型上重复扣除键盘高度。
 */
export interface KeyboardInsetViewport {
  /** 未被键盘缩短时的逻辑窗口高度。 */
  baselineHeight: number
  /** 当前逻辑窗口高度。 */
  windowHeight: number
}

function finiteNonNegative(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0
}

export function normalizeKeyboardHeight(value: unknown, maxHeight = 0): number {
  const height = finiteNonNegative(Number(value))
  const max = finiteNonNegative(maxHeight)
  return max > 0 ? Math.min(height, max) : height
}

/**
 * 计算需要由页面自己补偿的键盘高度。
 *
 * 当平台已经把 windowHeight 缩短时，先扣掉 viewportReduction；
 * 当平台没有缩短窗口时，完整使用键盘事件高度。无效/负数尺寸统一回零。
 */
export function resolveKeyboardInset(reportedHeight: unknown, viewport: KeyboardInsetViewport): number {
  const baseline = finiteNonNegative(viewport.baselineHeight)
  const current = finiteNonNegative(viewport.windowHeight)
  const maxHeight = baseline > 0 ? baseline : current
  const reported = normalizeKeyboardHeight(reportedHeight, maxHeight)
  const reduction = baseline > 0 && current > 0 ? Math.max(0, baseline - current) : 0
  return Math.max(0, Math.round(reported - reduction))
}
