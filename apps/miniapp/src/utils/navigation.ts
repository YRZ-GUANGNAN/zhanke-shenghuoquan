import Taro from '../services/taro'

// Native success can precede the visible transition; absorb that transition's extra taps.
export const RETURN_TRANSITION_GUARD_MS = 450
let activeReturn: Promise<void> | null = null
let guardedUntil = 0

async function performReturn(): Promise<void> {
  try {
    await Taro.navigateBack({ delta: 1 })
    guardedUntil = Date.now() + RETURN_TRANSITION_GUARD_MS
  } catch {
    // A shared detail page can be the only page in the stack.
    try {
      await Taro.reLaunch({ url: '/pages/home/index' })
      guardedUntil = Date.now() + RETURN_TRANSITION_GUARD_MS
    } catch {
      await Taro.showToast({ title: '暂时无法返回，请重试', icon: 'none' }).catch(() => undefined)
    }
  }
}

export function navigateBackOrHome(): Promise<void> {
  if (activeReturn) return activeReturn
  if (Date.now() < guardedUntil) return Promise.resolve()
  const request = performReturn()
  activeReturn = request
  const release = () => { if (activeReturn === request) activeReturn = null }
  void request.then(release, release)
  return request
}
