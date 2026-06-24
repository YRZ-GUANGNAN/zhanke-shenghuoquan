import React, { useEffect, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { centsToYuan, getBuyerOrderDetail, type BuyerOrderDetail, type OrderStatus } from '../../services/g-takeout-buyer'
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

export default function TakeoutOrderDetailPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const [detail, setDetail] = useState<BuyerOrderDetail | null>(null)

  useEffect(() => {
    if (!params.id) return
    void getBuyerOrderDetail(params.id)
      .then(setDetail)
      .catch((error) => Taro.showToast({ title: error instanceof Error ? error.message : '订单详情加载失败', icon: 'none' }))
  }, [params.id])

  if (!detail) {
    return <View className='page takeout-order-detail-page'><View className='card detail-card'>订单加载中...</View></View>
  }

  const order = detail.order

  return (
    <View className='page takeout-order-detail-page'>
      <View className='card detail-card'>
        <View className='between'>
          <Text className='detail-title'>{statusText[order.status]}</Text>
          <Text className={`detail-status detail-status--${order.status}`}>{order.status}</Text>
        </View>
        {order.status === 'pending_payment' && <Text className='detail-warning'>待支付（支付功能开发中）</Text>}
        <Text className='detail-tip'>订单号：{order.orderPublicId}</Text>
        <Text className='detail-tip'>创建时间：{formatTime(order.createdAt)}</Text>
      </View>

      <View className='card detail-card'>
        <Text className='detail-section-title'>菜品明细</Text>
        {detail.items.map((item) => (
          <View key={`${item.dishPublicId}-${item.quantity}`} className='detail-row'>
            <View>
              <Text className='detail-item-name'>{item.dishNameSnapshot}</Text>
              <Text className='detail-tip'>¥{centsToYuan(item.unitPriceCentsSnapshot)} × {item.quantity}</Text>
            </View>
            <Text className='detail-price'>¥{centsToYuan(item.lineAmountCents)}</Text>
          </View>
        ))}
        <View className='detail-total'>
          <Text>商品 ¥{centsToYuan(order.itemsAmountCents)}</Text>
          <Text>配送 ¥{centsToYuan(order.deliveryFeeCents)}</Text>
          <Text className='detail-price'>合计 ¥{centsToYuan(order.totalAmountCents)}</Text>
        </View>
      </View>

      <View className='card detail-card'>
        <Text className='detail-section-title'>配送信息</Text>
        <Text className='detail-tip'>{order.contactName} · {order.contactPhone}</Text>
        <Text className='detail-tip'>{order.addressBuilding} {order.addressRoom}</Text>
        {order.remark && <Text className='detail-tip'>备注：{order.remark}</Text>}
      </View>

      <Button className='cc-button cc-button--primary cc-button--block' onClick={() => Taro.redirectTo({ url: '/pages/takeout-orders/index' })}>返回订单列表</Button>
    </View>
  )
}

function formatTime(value: string): string {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}
