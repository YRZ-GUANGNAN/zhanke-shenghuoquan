import { useCallback, useEffect, useRef, useState } from 'react'

interface CacheEntry {
  scope: string
  data: unknown
}

// 模块级缓存：页面销毁后依然保留，用于「先旧后新」的切页恢复。
const resourceCache = new Map<string, CacheEntry>()

export interface CachedResourceOptions<T> {
  /** 缓存键：同一份数据用同一个 key。 */
  key: string
  /** 作用域（通常是登录会话），不一致时旧缓存自动作废。 */
  scope: string
  /** false 时不请求并清空展示。 */
  enabled?: boolean
  /** 实际拉取数据的函数。 */
  read: () => Promise<T>
}

export interface CachedResource<T> {
  data: T | null
  loading: boolean
  error: boolean
  reload: () => Promise<void>
}

// 通用「先旧后新」资源 hook：有缓存先用缓存渲染并后台静默刷新，
// 无缓存才显示加载态。缓存按 (key, scope) 隔离，防止跨账号串数据。
export function useCachedResource<T>(options: CachedResourceOptions<T>): CachedResource<T> {
  const { key, scope } = options
  const enabled = options.enabled ?? true
  const readRef = useRef(options.read)
  readRef.current = options.read
  const enabledRef = useRef(enabled)
  enabledRef.current = enabled

  const [state, setState] = useState<{ data: T | null; loading: boolean; error: boolean }>(() => {
    if (!enabled) return { data: null, loading: false, error: false }
    const entry = resourceCache.get(key)
    const cached = entry && entry.scope === scope ? (entry.data as T) : null
    // 有缓存时 loading 为 false：界面直接显示旧数据，而不是"正在读取"。
    return { data: cached, loading: cached === null, error: false }
  })
  const versionRef = useRef(0)
  const busyRef = useRef(false)

  const reload = useCallback(async () => {
    if (!enabledRef.current || busyRef.current) return
    busyRef.current = true
    const version = ++versionRef.current
    setState((current) => ({ ...current, loading: current.data === null, error: false }))
    try {
      const data = await readRef.current()
      if (version === versionRef.current && enabledRef.current) {
        resourceCache.set(key, { scope, data })
        setState({ data, loading: false, error: false })
      }
    } catch {
      if (version === versionRef.current && enabledRef.current) {
        setState((current) => ({ ...current, loading: false, error: true }))
      }
    } finally {
      if (version === versionRef.current) busyRef.current = false
    }
  }, [key, scope])

  useEffect(() => {
    if (enabled) void reload()
    else setState({ data: null, loading: false, error: false })
    return () => {
      // 组件卸载后让在途请求作废，避免回写已销毁页面的状态。
      versionRef.current += 1
      busyRef.current = false
    }
  }, [enabled, reload])

  return { ...state, reload }
}
