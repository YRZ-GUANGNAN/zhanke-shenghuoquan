import type { CommunityPost, Product } from '../types'
import type { CampusGuide } from './campus-info-model'

export interface ShareSettings {
  homeTitle: string
  secondhandTitle: string
  servicesTitle: string
  defaultImageUrl: string
  detailTitleSuffix: string
  preferContentImage: boolean
}
// A package asset (never a data URL), also used by the admin's default-cover preview.
export const DEFAULT_SHARE_IMAGE = '/assets/share/default-share.png'
export const DEFAULT_SHARE_SETTINGS: ShareSettings = {
  homeTitle: '湛科生活圈', secondhandTitle: '校园二手 · 湛科生活圈', servicesTitle: '校园服务 · 湛科生活圈',
  defaultImageUrl: '', detailTitleSuffix: '湛科生活圈', preferContentImage: true
}
export interface ShareCard { title: string; path: string; imageUrl?: string }
export type ShareTarget =
  | { kind: 'guide'; enabled: boolean; id: string; content: CampusGuide | null }
  | { kind: 'home' | 'secondhand' | 'services'; enabled: boolean }
  | { kind: 'post'; enabled: boolean; id: string; content: CommunityPost | null }
  | { kind: 'product'; enabled: boolean; id: string; content: Product | null }
  | { kind: 'profile'; enabled: boolean; id: string; nickname: string; loadedId: string }

export function publicShareId(value: unknown): string {
  // IDs are opaque decimal strings; never coerce through Number or double-decode URLs.
  return typeof value === 'string' && /^[1-9]\d{0,19}$/.test(value) ? value : ''
}

export function shareImage(value: unknown): string {
  // Taro's Mini Program runtime need not supply the browser URL constructor.
  if (typeof value !== 'string' || value.length > 2048 || /[\s\\]/.test(value)) return ''
  return /^https:\/\/[^/?#@:]+(?:\.[^/?#@:]+)+\/[^#]*\.(?:png|jpe?g)(?:\?[^#]*)?$/i.test(value) ? value : ''
}
function title(value: string): string { return [...value.replace(/[\u0000-\u001f\u007f]/g, ' ').replace(/\s+/g, ' ').trim()].slice(0, 100).join('') }

export function parseShareSettings(value: unknown): ShareSettings | null {
  if (!value || typeof value !== 'object') return null
  const data = value as Record<string, unknown>
  if (['homeTitle', 'secondhandTitle', 'servicesTitle'].some(key => typeof data[key] !== 'string' || !(data[key] as string).trim()) ||
    typeof data.detailTitleSuffix !== 'string' || typeof data.defaultImageUrl !== 'string' || typeof data.preferContentImage !== 'boolean') return null
  return {
    homeTitle: title(data.homeTitle as string), secondhandTitle: title(data.secondhandTitle as string), servicesTitle: title(data.servicesTitle as string),
    detailTitleSuffix: title(data.detailTitleSuffix), defaultImageUrl: shareImage(data.defaultImageUrl), preferContentImage: data.preferContentImage
  }
}

export function buildShareCard(target: ShareTarget, settings: ShareSettings): ShareCard | null {
  if (!target.enabled) return null
  const fallbackImage = shareImage(settings.defaultImageUrl) || DEFAULT_SHARE_IMAGE
  const card = (text: string, path: string, media = ''): ShareCard => {
    const imageUrl = (settings.preferContentImage && shareImage(media)) || fallbackImage
    return { title: title(text), path, ...(imageUrl ? { imageUrl } : {}) }
  }
  const detailTitle = (text: string) => title(`${title(text)}${settings.detailTitleSuffix ? ` · ${settings.detailTitleSuffix}` : ''}`)
  switch (target.kind) {
    case 'guide':
      if (!target.content || target.content.id !== target.id || !/^[a-z][a-z0-9-]{0,63}$/.test(target.id)) return null
      return card(detailTitle(target.content.title), `/pages/campus-guide/index?id=${target.id}`)
    case 'home': return card(settings.homeTitle, '/pages/home/index')
    case 'secondhand': return card(settings.secondhandTitle, '/pages/student-secondhand/index')
    case 'services': return card(settings.servicesTitle, '/pages/student-takeout/index')
    case 'post': {
      const post = target.content
      if (!post || !publicShareId(target.id) || post.id !== target.id || post.reviewStatus !== 'approved') return null
      return card(detailTitle(post.title || post.content || '校园动态'), `/pages/post-detail/index?id=${target.id}`, post.images.find(image => shareImage(image)) || '')
    }
    case 'product': {
      const product = target.content
      if (!product || !publicShareId(target.id) || product.id !== target.id || product.reviewStatus !== 'approved') return null
      return card(detailTitle(product.title || '校园二手'), `/pages/secondhand-detail/index?id=${target.id}`, product.images.find(image => shareImage(image)) || '')
    }
    case 'profile':
      if (!publicShareId(target.id) || target.loadedId !== target.id) return null
      return card(detailTitle(`${target.nickname || '校园用户'}的主页`), `/pages/user-profile/index?userId=${target.id}`)
  }
}
