export const FEATURE_KEYS = ['forum', 'post', 'comment', 'secondhand', 'secondhand_publish', 'secondhand_chat', 'takeout', 'campus_tools', 'delivery', 'merchant', 'rider', 'messages', 'following', 'search', 'likes', 'favorites', 'notifications', 'identity_application', 'publish', 'secondhand_comments'] as const
export type FeaturePolicyKey = typeof FEATURE_KEYS[number]
export type FeatureVisibility = Record<FeaturePolicyKey, boolean> & {
  postingMode: 'admin_only' | 'all_users'
  enabled: boolean
  labEnabled: boolean
  rowVersion: number
  available: boolean
}

export const CLOSED_VISIBILITY: FeatureVisibility = {
  ...FEATURE_KEYS.reduce((flags, key) => { flags[key] = false; return flags }, {} as Record<FeaturePolicyKey, boolean>),
  postingMode: 'admin_only', enabled: true, labEnabled: false, rowVersion: 0, available: false,
}

export function parseFeatureVisibility(value: unknown): FeatureVisibility {
  if (!value || typeof value !== 'object') return CLOSED_VISIBILITY
  const data = value as { flags?: Record<string, unknown>; postingMode?: string; enabled?: boolean; rowVersion?: number }
  if (data.enabled !== true || !data.flags || !Number.isSafeInteger(data.rowVersion) || (data.rowVersion || 0) < 1 || !['admin_only', 'all_users'].includes(data.postingMode || '') || FEATURE_KEYS.some(key => typeof data.flags?.[key] !== 'boolean')) return CLOSED_VISIBILITY
  const flags = FEATURE_KEYS.reduce((flags, key) => { flags[key] = data.flags?.[key] === true; return flags }, {} as Record<FeaturePolicyKey, boolean>)
  flags.post = flags.post && flags.forum && flags.publish
  flags.comment = flags.comment && flags.forum
  flags.following = flags.following && flags.forum
  flags.notifications = flags.notifications && flags.forum
  flags.secondhand_publish = flags.secondhand_publish && flags.secondhand && flags.publish
  flags.secondhand_chat = flags.secondhand_chat && flags.secondhand && flags.messages
  flags.secondhand_comments = flags.secondhand_comments && flags.secondhand
  return { ...CLOSED_VISIBILITY, ...flags, postingMode: data.postingMode as FeatureVisibility['postingMode'], rowVersion: data.rowVersion!, available: true }
}

export function featurePageKeys(path: string, params: Record<string, string | undefined> = {}): FeaturePolicyKey[] {
  const page = path.replace(/^\//, '').split('?')[0].replace(/^pages\//, '').replace(/\/index$/, '')
  if (page === 'publish-entry' || page === 'publish') return ['publish']
  if (page === 'publish-post') return ['post']
  if (page === 'post-detail' || page === 'user-profile') return ['forum']
  if (['student-secondhand', 'market', 'secondhand-detail', 'product-detail'].includes(page)) return ['secondhand']
  if (page === 'publish-secondhand') return ['secondhand_publish']
  if (page === 'secondhand-chat' || page === 'chat' || page === 'conversations') return ['secondhand_chat']
  if (page === 'notifications') return ['notifications']
  if (page === 'search') return ['search', params.scope === 'takeout' ? 'takeout' : params.scope === 'secondhand' ? 'secondhand' : 'forum']
  if (page === 'identity-application') return ['identity_application', params.role === 'rider' ? 'rider' : 'merchant']
  if (page === 'merchant-publish') return ['merchant', 'delivery']
  if (page === 'takeout' || page === 'takeout-cart') return ['takeout']
  // This page can switch between community and products; gate each visible tab
  // inside the page instead of tying the whole page to its initial URL.
  if (page === 'my-content') return []
  return []
}

export function featureRequestKeys(method: string, url: string): FeaturePolicyKey[] {
  const path = url.split('?')[0]
  if (path.startsWith('/community/')) {
    const keys: FeaturePolicyKey[] = ['forum']
    if (path.includes('/search')) keys.push('search')
    if (path.includes('/notifications')) keys.push('notifications')
    if (path.includes('/following-feed') || path.endsWith('/follow')) keys.push('following')
    if (path.endsWith('/like')) keys.push('likes')
    if (path.includes('/favorite')) keys.push('favorites')
    if (path.includes('/comments')) keys.push('comment')
    if (method === 'POST' && path === '/community/posts') keys.push('post')
    return keys
  }
  if (path.startsWith('/trade/')) {
    const keys: FeaturePolicyKey[] = ['secondhand']
    if (path.includes('/search')) keys.push('search')
    if (path.includes('/favorite')) keys.push('favorites')
    if (path.includes('/comments')) keys.push('secondhand_comments')
    if (path.endsWith('/contact')) keys.push('secondhand_chat')
    if (method === 'POST' && path === '/trade/products' || method === 'PUT' && path.startsWith('/trade/products/') && !path.endsWith('/favorite') && !path.endsWith('/contact')) keys.push('secondhand_publish')
    return keys
  }
  if (path.startsWith('/im/')) return ['messages', 'secondhand_chat']
  if (path.startsWith('/navigation/external-takeout')) return path.includes('/search') ? ['takeout', 'search'] : ['takeout']
  if (path.startsWith('/navigation')) return ['campus_tools']
  if (path.startsWith('/auth/identity-applications') && (method === 'POST' || method === 'PUT')) return ['identity_application', path.endsWith('/rider') ? 'rider' : 'merchant']
  if (path === '/delivery/merchant/orders' && method === 'POST') return ['delivery', 'merchant']
  if (path === '/delivery/rider/orders/claimable' || path.startsWith('/delivery/rider/orders/') && path.endsWith('/claim') && method === 'POST') return ['delivery', 'rider']
  if (path.startsWith('/takeout/buyer/merchants') || path === '/takeout/buyer/orders' && method === 'POST') return ['takeout']
  if (path.startsWith('/takeout/merchant/') && (path.includes('/categories') || path.includes('/dishes') || path.endsWith('/onboarding'))) return ['takeout', 'merchant']
  if (path === '/takeout/rider/claimable-orders' || path === '/takeout/rider/register' || path.startsWith('/takeout/rider/orders/') && path.endsWith('/claim')) return ['takeout', 'rider']
  return []
}
