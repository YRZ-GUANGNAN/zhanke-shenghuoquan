import Taro from '../services/taro'

export async function navigateBackOrHome(): Promise<void> {
  try {
    await Taro.navigateBack({ delta: 1 })
  } catch {
    // A shared detail page can be the only page in the stack.
    try {
      await Taro.reLaunch({ url: '/pages/home/index' })
    } catch {
      await Taro.showToast({ title: '暂时无法返回，请重试', icon: 'none' }).catch(() => undefined)
    }
  }
}
