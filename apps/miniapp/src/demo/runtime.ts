import Taro from '@tarojs/taro'
import type Facade from '../services/taro'

export function assertDemoEnvironment(version: string) {
  if (version !== 'develop' && version !== 'trial') throw new Error('模拟包仅供开发与体验使用，不能用于正式版本')
}

export function ensureDemo() {
  assertDemoEnvironment((Taro as unknown as typeof Facade).getAccountInfoSync().miniProgram.envVersion)
}

export const DEMO_STORAGE_PREFIX = 'visual-demo-20261017.'

export function demoStorageKey(key: string) {
  return DEMO_STORAGE_PREFIX + key
}
