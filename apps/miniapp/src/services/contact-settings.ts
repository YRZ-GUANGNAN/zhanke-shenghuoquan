// 联系设置：众创申请弹窗的微信号与文案（后台可维护）。
import { request } from './api'

export interface ContactSettings {
  makerWechat: string
  makerIntro: string
}

export async function getContactSettings(): Promise<ContactSettings> {
  return request<ContactSettings>('/contact-settings', { url: '', timeout: 10000 })
}
