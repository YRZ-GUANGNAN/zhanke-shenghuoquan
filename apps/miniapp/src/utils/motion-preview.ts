// List previews always leave the rest of the clip for the detail page.
export function previewSeconds(durationMs: number): number {
  if (!Number.isFinite(durationMs) || durationMs < 500 || durationMs > 60000) return 0
  return Math.min(3, durationMs / 2000, (durationMs - 300) / 1000)
}

export function createMotionPreviewCoordinator(onChange: (id: string) => void) {
  const candidates = new Map<string, { order: number; stop: () => void }>()
  const played = new Set<string>()
  let active = '', enabled = false
  const select = () => {
    const currentStillVisible = enabled && candidates.has(active) && !played.has(active)
    const next = currentStillVisible ? active : enabled ? [...candidates.entries()].filter(([id]) => !played.has(id)).sort((a, b) => a[1].order - b[1].order)[0]?.[0] ?? '' : ''
    if (next === active) return
    const previous = active
    active = next
    candidates.get(previous)?.stop()
    onChange(next)
  }
  return {
    setEnabled(value: boolean) { enabled = value; select() },
    visible(id: string, order: number, stop: () => void) { candidates.set(id, { order, stop }); select() },
    hidden(id: string) { candidates.get(id)?.stop(); candidates.delete(id); select() },
    finish(id: string) { played.add(id); select() },
    dispose() { enabled = false; select(); candidates.clear(); played.clear() },
  }
}
export type MotionPreviewCoordinator = ReturnType<typeof createMotionPreviewCoordinator>
