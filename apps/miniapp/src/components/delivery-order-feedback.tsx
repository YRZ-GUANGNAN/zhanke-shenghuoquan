import React from 'react'
import { Text, View } from '@tarojs/components'
import './delivery-order-feedback.scss'

/** Feedback used only in merchant/rider workspaces. No page-wide loading layer. */
export function DeliveryLoadDots() {
  return <View className='delivery-load-dots'>
    <View className='delivery-load-dots__dot' />
    <View className='delivery-load-dots__dot' />
    <View className='delivery-load-dots__dot' />
  </View>
}

export function DeliveryOrderLoading({ label = '正在加载订单', compact = false, columns = 1 }: {
  label?: string
  compact?: boolean
  columns?: 1 | 2
}) {
  return <View className={`delivery-order-loading${compact ? ' delivery-order-loading--compact' : ''}`} ariaLabel={label}>
    <View className='delivery-order-loading__label'><DeliveryLoadDots /><Text>{label}</Text></View>
    {!compact && <View className={`delivery-order-skeletons delivery-order-skeletons--${columns}`}>
      {Array.from({ length: columns === 2 ? 4 : 2 }, (_, index) => <View key={index} className='delivery-order-skeleton'>
        <View className='delivery-order-skeleton__source' />
        <View className='delivery-order-skeleton__number' />
        <View className='delivery-order-skeleton__route' />
        <View className='delivery-order-skeleton__route delivery-order-skeleton__route--short' />
        <View className='delivery-order-skeleton__action' />
      </View>)}
    </View>}
  </View>
}

export function DeliveryOrderState({ kind, title, description, actionLabel = '重新加载', onAction, compact = false }: {
  kind: 'empty' | 'error' | 'access'
  title: string
  description: string
  actionLabel?: string
  onAction?: () => void
  compact?: boolean
}) {
  return <View className={`delivery-order-state delivery-order-state--${kind}${compact ? ' delivery-order-state--compact' : ''}`}>
    <View className='delivery-order-state__copy'>
      <Text className='delivery-order-state__eyebrow'>{kind === 'empty' ? '订单列表' : kind === 'error' ? '暂未更新' : '身份核验'}</Text>
      <Text className='delivery-order-state__title'>{title}</Text>
      <Text className='delivery-order-state__description'>{description}</Text>
    </View>
    {onAction && <View className='delivery-order-state__action' ariaRole='button' ariaLabel={actionLabel}
      hoverClass='delivery-order-control--pressed' onClick={onAction}><Text>{actionLabel}</Text></View>}
  </View>
}
