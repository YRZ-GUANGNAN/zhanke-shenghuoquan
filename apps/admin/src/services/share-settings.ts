import { adminRequest } from './http'

export interface ShareCards {
  homeTitle: string
  secondhandTitle: string
  servicesTitle: string
  defaultImageUrl: string
  detailTitleSuffix: string
  preferContentImage: boolean
}
export interface ShareSettings extends ShareCards { rowVersion: number; updatedAt: string }

export const shareSettingsAPI = {
  read: () => adminRequest<ShareSettings>('/admin/share-settings'),
  save: (value: ShareCards, expectedRowVersion: number) => adminRequest<ShareSettings>('/admin/share-settings', {
    method: 'PUT', body: JSON.stringify({ ...value, expectedRowVersion })
  })
}

export function cardFields(value: ShareCards): ShareCards {
  const { homeTitle, secondhandTitle, servicesTitle, defaultImageUrl, detailTitleSuffix, preferContentImage } = value
  return { homeTitle, secondhandTitle, servicesTitle, defaultImageUrl, detailTitleSuffix, preferContentImage }
}

export function coverError(value: string): string {
  if (!value.trim()) return ''
  try {
    const url = new URL(value.trim())
    if (url.protocol !== 'https:' || url.username || url.password || url.port || url.search || url.hash ||
      (url.hostname !== 'zhanduiwang.top' && !url.hostname.endsWith('.myqcloud.com')) || !/\.(png|jpe?g)$/i.test(url.pathname)) throw new Error()
    return ''
  } catch { return '请输入本站或腾讯云 COS 的公开 HTTPS JPG/PNG 地址，不使用临时签名链接。' }
}
