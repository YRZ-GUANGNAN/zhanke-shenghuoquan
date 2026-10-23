import { request } from './api'
import { DEFAULT_SHARE_SETTINGS, parseShareSettings, type ShareSettings } from './share-model'

let snapshot: ShareSettings = DEFAULT_SHARE_SETTINGS
let readAt = 0
let pending: Promise<ShareSettings> | null = null
const listeners = new Set<(value: ShareSettings) => void>()
export function shareSettingsSnapshot() { return snapshot }
export function subscribeShareSettings(listener: (value: ShareSettings) => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}
export async function getShareSettings(force = false): Promise<ShareSettings> {
  if (pending) return pending
  if (!force && Date.now() - readAt < 30000) return snapshot
  const task = (async () => {
    try {
      const next = parseShareSettings(await request('/share-settings'))
      if (next && JSON.stringify(next) !== JSON.stringify(snapshot)) {
        snapshot = next
        listeners.forEach(listener => listener(snapshot))
      }
    } catch { /* Cosmetic settings never block login or access to real content. */ }
    readAt = Date.now()
    return snapshot
  })()
  pending = task
  try { return await task } finally { if (pending === task) pending = null }
}
