import NativeTaro from '@tarojs/taro'
import type Facade from '../services/taro'
import type { RequestOptions } from '../services/taro'
import { demoRequest, DemoRequestError } from './api'
import { demoStorageKey, ensureDemo } from './runtime'

const native = NativeTaro as unknown as typeof Facade
const safePages = new Set(['home','profile','user-profile','post-detail','secondhand-detail','secondhand-chat','student-secondhand','search','my-content'])
async function navigation(kind: 'navigateTo' | 'redirectTo' | 'reLaunch', options: { url: string }) {
  ensureDemo()
  const page = options.url.match(/^\/pages\/([^/]+)\/index(?:\?|$)/)?.[1]
  if (!page || !safePages.has(page)) return native.showModal({ title: '模拟体验', content: '此版本演示帖子、二手、主页和私信。该操作没有连接真实账户或交易。', showCancel: false })
  return native[kind](options)
}
const demoTaro: typeof Facade = {
  ...native,
  login: async () => { ensureDemo(); return { code: 'visual-demo-local-code' } },
  getStorageSync: <T,>(key: string): T => { ensureDemo(); return native.getStorageSync<T>(demoStorageKey(key)) },
  setStorageSync: <T,>(key: string, value: T) => { ensureDemo(); native.setStorageSync(demoStorageKey(key), value) },
  removeStorageSync: (key: string) => { ensureDemo(); native.removeStorageSync(demoStorageKey(key)) },
  request: async <T,>(options: RequestOptions) => {
    ensureDemo()
    await new Promise(resolve => setTimeout(resolve, 100))
    const match = options.url.match(/^https:\/\/visual-demo\.invalid\/api\/v1(\/.*)$/)
    if (!match) throw new Error('模拟体验已阻止外部业务请求')
    try { return { statusCode: 200, data: { code: 0, message: '模拟数据', data: demoRequest(match[1], options.method || 'GET', options.data) } as T } }
    catch (error) {
      const status = error instanceof DemoRequestError ? error.status : 404
      return { statusCode: status, data: { code: status, message: error instanceof Error ? error.message : '模拟数据不可用' } as T }
    }
  },
  navigateTo: options => navigation('navigateTo', options),
  redirectTo: options => navigation('redirectTo', options),
  reLaunch: options => navigation('reLaunch', options),
  requestPayment: async () => { throw new Error('模拟体验不支持付款') },
  navigateToMiniProgram: async () => { throw new Error('模拟体验不打开外部小程序') },
  makePhoneCall: async () => { throw new Error('模拟体验不拨打电话') },
  chooseMedia: async () => { throw new Error('模拟体验使用预置摄影样本，不上传个人照片') },
}
export default demoTaro
export type { RequestOptions, MenuButtonRect, WindowInfo } from '../services/taro'
