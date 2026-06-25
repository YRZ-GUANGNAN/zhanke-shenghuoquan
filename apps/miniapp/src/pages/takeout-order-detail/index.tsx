import React, { useEffect, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { centsToYuan, getBuyerOrderDetail, payBuyerOrder, type BuyerOrderDetail, type OrderStatus } from '../../services/g-takeout-buyer'
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
  const [paying, setPaying] = useState(false)

  useEffect(() => {
    if (!params.id) return
    void loadDetail(params.id)
  }, [params.id])

  const loadDetail = async (id = params.id) => {
    if (!id) return
    try {
      setDetail(await getBuyerOrderDetail(id))
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '订单详情加载失败', icon: 'none' })
    }
  }

  const handlePay = async () => {
    const currentOrder = detail?.order
    if (!currentOrder || paying) return
    setPaying(true)
    try {
      const result = await payBuyerOrder(currentOrder.orderPublicId)
      if (result.mode === 'wechat') {
        if (!result.wechat) throw new Error('缺少微信支付参数')
        await Taro.requestPayment(result.wechat)
      }
      await loadDetail(currentOrder.orderPublicId)
      void Taro.showToast({ title: result.mode === 'mock' ? 'mock 支付成功' : '支付处理中', icon: 'success' })
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '支付失败', icon: 'none' })
    } finally {
      setPaying(false)
    }
  }

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
        {order.status === 'pending_payment' && (
          <View className='detail-payment'>
            <Text className='detail-warning'>待支付：支付成功后由服务端确认订单状态</Text>
            <Button className='cc-button cc-button--primary cc-button--block' disabled={paying} onClick={handlePay}>
              {paying ? '支付处理中...' : '去支付'}
            </Button>
          </View>
        )}
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
