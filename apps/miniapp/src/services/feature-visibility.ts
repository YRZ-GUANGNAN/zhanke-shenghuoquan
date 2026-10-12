import { request } from './api'
import { CLOSED_VISIBILITY, parseFeatureVisibility, type FeatureVisibility } from './feature-policy-model'
export type { FeaturePolicyKey, FeatureVisibility } from './feature-policy-model'

let snapshot = CLOSED_VISIBILITY
let readAt = 0
let pending: Promise<FeatureVisibility> | null = null
const listeners = new Set<(value: FeatureVisibility) => void>()

export function featureVisibilitySnapshot(): FeatureVisibility { return snapshot }
export function subscribeFeatureVisibility(listener: (value: FeatureVisibility) => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

// Read failures close optional surfaces. Concurrent reads share one request.
export async function getFeatureVisibility(force = false): Promise<FeatureVisibility> {
  if (pending) return pending
  if (!force && Date.now() - readAt < 5000) return snapshot
  const task = (async () => {
    let next: FeatureVisibility
    try { next = parseFeatureVisibility(await request('/features')) }
    catch { next = CLOSED_VISIBILITY }
    readAt = Date.now()
    if (JSON.stringify(next) !== JSON.stringify(snapshot)) {
      snapshot = next
      listeners.forEach(listener => listener(snapshot))
    }
    return snapshot
  })()
  pending = task
  try { return await task } finally { if (pending === task) pending = null }
}
