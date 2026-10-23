export type SourcePlatform = 'local' | 'taobao' | 'jd' | 'meituan'

const labels: Record<SourcePlatform, string> = {
  local: '校内配送',
  taobao: '淘宝闪购',
  jd: '京东外卖',
  meituan: '美团外卖',
}

export function deliverySourceLabel(platform?: string): string {
  return platform && Object.prototype.hasOwnProperty.call(labels, platform)
    ? labels[platform as SourcePlatform]
    : '校内配送'
}

export function deliveryPickupLabel(order: { sourcePlatform?: string; pickupCode?: string }): string {
  const code = order.pickupCode?.trim()
  return code ? (code.endsWith('号') ? code : `${code}号`) : '配送单'
}
