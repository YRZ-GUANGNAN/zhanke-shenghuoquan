import Taro from '../services/taro'
import { removeLocalFile, saveLocalFile } from './local-file'

const HOME_HEADER_BACKDROP_KEY = 'ui.homeHeaderBackdrop'
const HOME_HEADER_IMAGE_KEY = 'ui.homeHeaderBackdropImage'

// 首页头图是纯本地展示偏好：默认关闭（纯白头部），在设置页开启。
export function readHomeHeaderBackdrop(): boolean {
  try {
    return Taro.getStorageSync(HOME_HEADER_BACKDROP_KEY) === '1'
  } catch {
    return false
  }
}

export function writeHomeHeaderBackdrop(enabled: boolean) {
  try {
    Taro.setStorageSync(HOME_HEADER_BACKDROP_KEY, enabled ? '1' : '0')
  } catch {
    // 存储失败不应阻断开关的即时生效（本次会话内仍按内存状态显示）。
  }
}

// 用户自选头图：保存到小程序本地用户目录（跨启动有效，仅本机可见）。
export function readHomeHeaderBackdropImage(): string {
  try {
    const value = Taro.getStorageSync(HOME_HEADER_IMAGE_KEY)
    return typeof value === 'string' && value ? value : ''
  } catch {
    return ''
  }
}

export async function saveHomeHeaderBackdropImage(tempFilePath: string): Promise<string> {
  const previous = readHomeHeaderBackdropImage()
  const savedFilePath = await saveLocalFile(tempFilePath)
  Taro.setStorageSync(HOME_HEADER_IMAGE_KEY, savedFilePath)
  if (previous && previous !== savedFilePath) {
    void removeLocalFile(previous).catch(() => undefined)
  }
  return savedFilePath
}

export async function resetHomeHeaderBackdropImage(): Promise<void> {
  const previous = readHomeHeaderBackdropImage()
  try {
    Taro.removeStorageSync(HOME_HEADER_IMAGE_KEY)
  } catch {
    // 清空存储失败时保留旧记录，下次进入编辑页仍可重试。
  }
  if (previous) {
    try {
      await removeLocalFile(previous)
    } catch {
      // 本地文件可能已被系统清理，忽略即可。
    }
  }
}
