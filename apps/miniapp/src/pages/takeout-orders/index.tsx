import React, { useEffect, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { centsToYuan, listBuyerOrders, type BuyerOrder, type OrderStatus } from '../../services/g-takeout-buyer'
import Taro from '../../services/taro'
import './index.scss'

const statusText: Record<OrderStatus, string> = {
  pending_payment: '待支付',
  paid: '待接单',
  accepted: '已接单',
  preparing: '备餐中',
  ready_for_pickup: '待取餐',
  delivering: '配送中',
  completed: '已完成',
  cancelled: '已取消',
  refunded: '已退款'
}

export default function TakeoutOrdersPage() {
  const [status, setStatus] = useState<OrderStatus | ''>('')
  const [orders, setOrders] = useState<BuyerOrder[]>([])

  useEffect(() => {
    void loadOrders()
  }, [])

  const loadOrders = async (nextStatus = status) => {
    try {
      const page = await listBuyerOrders(nextStatus || undefined)
      setOrders(page.items)
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '订单加载失败', icon: 'none' })
    }
  }

  return (
    <View className='page takeout-orders-page'>
      <View className='orders-filter'>
        {(['', 'pending_payment', 'paid', 'accepted', 'preparing', 'ready_for_pickup', 'delivering', 'completed'] as Array<OrderStatus | ''>).map((item) => (
          <Text key={item || 'all'} className={status === item ? 'orders-filter__active' : ''} onClick={() => { setStatus(item); void loadOrders(item) }}>
            {item ? statusText[item] : '全部'}
          </Text>
        ))}
      </View>
      {orders.length === 0 && <View className='card order-card'>暂无订单</View>}
      {orders.map((order) => (
        <View key={order.orderPublicId} className='card order-card'>
          <View className='between'>
            <Text className='order-title'>订单 {shortId(order.orderPublicId)}</Text>
            <Text className={`order-status order-status--${order.status}`}>{statusText[order.status]}</Text>
          </View>
          <Text className='order-tip'>{order.addressBuilding} {order.addressRoom} · ¥{centsToYuan(order.totalAmountCents)}</Text>
          <Text className='order-tip'>创建时间：{formatTime(order.createdAt)}</Text>
          <Button className='cc-button cc-button--small' onClick={() => Taro.navigateTo({ url: `/pages/takeout-order-detail/index?id=${order.orderPublicId}` })}>查看详情</Button>
        </View>
      ))}
    </View>
  )
}

function shortId(value: string): string {
  return value.length > 10 ? `${value.slice(0, 6)}...${value.slice(-4)}` : value
}

function formatTime(value: string): string {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}
