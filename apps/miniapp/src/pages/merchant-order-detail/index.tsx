import React from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import Taro from '../../services/taro'
import { resolveStaticDeliveryOrder, type StaticDeliveryOrder } from '../delivery-static-data'
import './index.scss'

type StepState = 'done' | 'current' | 'todo'

interface ProgressStep {
  key: string
  title: string
  note?: string
  time?: string
  state: StepState
}

function buildProgressSteps(order: StaticDeliveryOrder): ProgressStep[] {
  const currentIndex = {
    waiting: 2,
    pickup: 3,
    delivering: 5,
    completed: 6
  }[order.stage]
  const templates = [
    { key: 'created', title: '创建订单', time: order.orderTime },
    { key: 'paid', title: '支付配送费', note: `¥${order.fee} · ${order.paymentLabel}`, time: order.orderTime },
    { key: 'pool', title: '进入抢单池', note: '等待骑手抢单', time: order.orderTime },
    { key: 'claimed', title: '骑手接单', note: order.rider ? `${order.rider.name} · ${order.rider.phone}` : '等待骑手接单' },
    { key: 'picked', title: '已取货', note: '骑手上传取货照后进入配送' },
    { key: 'delivering', title: '配送中', note: `前往 ${order.destination}` },
    { key: 'delivered', title: '送达', note: '骑手上传送达照后完成' }
  ]

  return templates.map((step, index) => ({
    ...step,
    state: order.stage === 'completed' || index < currentIndex
      ? 'done'
      : index === currentIndex
        ? 'current'
        : 'todo'
  }))
}

function showStaticToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function handleBack() {
  void Taro.navigateBack()
}

function openConversation(orderId: string, role: 'merchant' | 'rider') {
  void Taro.navigateTo({
    url: `/pages/delivery-conversation/index?orderId=${encodeURIComponent(orderId)}&role=${role}`
  })
}

export default function MerchantOrderDetailPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const role = params.role === 'rider' ? 'rider' : 'merchant'
  const { order } = resolveStaticDeliveryOrder(params.orderId)
  const progressSteps = buildProgressSteps(order)
  const contact = role === 'rider'
    ? order.merchant
    : order.rider
  const pickupUploaded = order.stage === 'delivering' || order.stage === 'completed'
  const deliveredUploaded = order.stage === 'completed'

  return (
    <View className='merchant-order-detail-page'>
      <CapsulePageHeader
        className='merchant-detail-nav'
        title='配送单详情'
        left={<View className='merchant-detail-nav__back' onClick={handleBack}><Text>‹</Text></View>}
      />

      <ScrollView scrollY className='merchant-detail-scroll' enhanced showScrollbar={false}>
        <View className='merchant-detail-content'>
          <View className='merchant-detail-status-card'>
            <View className='merchant-detail-status-card__main'>
              <View className='merchant-detail-status-card__state-row'>
                <Text className='merchant-detail-status-card__state'>{order.statusLabel}</Text>
                <View className='merchant-detail-status-card__dot' />
              </View>
              <Text className='merchant-detail-status-card__sub'>{order.statusSub}</Text>
            </View>
            <View className='merchant-detail-status-card__fee'>
              <Text className='merchant-detail-status-card__amount'>¥{order.fee}</Text>
              <Text className='merchant-detail-status-card__pay'>{order.paymentLabel}</Text>
            </View>
            <View className='merchant-detail-status-card__divider' />
            <View className='merchant-detail-status-card__meta'>
              <Text>#{order.id} · {order.orderTime} 下单</Text>
              <Text>送达点: {order.destination}</Text>
            </View>
          </View>

          {contact ? (
            <View className='merchant-detail-rider-card'>
              <View className='merchant-detail-rider-card__avatar'>
                <Text>{contact.avatar}</Text>
              </View>
              <View className='merchant-detail-rider-card__main'>
                <Text className='merchant-detail-rider-card__name'>{contact.name}</Text>
                <Text className='merchant-detail-rider-card__phone'>{contact.phone}</Text>
              </View>
              <View className='merchant-detail-rider-card__actions'>
                <View className='merchant-detail-rider-card__circle' onClick={() => openConversation(order.id, role)}>
                  <Text>□</Text>
                </View>
                <View className='merchant-detail-rider-card__circle' onClick={() => showStaticToast('电话联系稍后开放')}>
                  <Text>⌕</Text>
                </View>
              </View>
            </View>
          ) : (
            <View className='merchant-detail-rider-card'>
              <View className='merchant-detail-rider-card__main'>
                <Text className='merchant-detail-rider-card__name'>等待骑手接单</Text>
                <Text className='merchant-detail-rider-card__phone'>接单后开放本单私信与电话入口</Text>
              </View>
            </View>
          )}

          <View className='merchant-detail-progress-card'>
            <Text className='merchant-detail-section-title'>配送进度</Text>
            <View className='merchant-detail-timeline'>
              {progressSteps.map((step, index) => (
                <View key={step.key} className='merchant-detail-timeline__row'>
                  <View className='merchant-detail-timeline__rail'>
                    <View className={`merchant-detail-timeline__marker merchant-detail-timeline__marker--${step.state}`}>
                      {step.state === 'done' ? <Text>✓</Text> : null}
                    </View>
                    {index < progressSteps.length - 1 ? (
                      <View className={`merchant-detail-timeline__line merchant-detail-timeline__line--${step.state}`} />
                    ) : null}
                  </View>
                  <View className='merchant-detail-timeline__body'>
                    <View className='merchant-detail-timeline__head'>
                      <Text className={`merchant-detail-timeline__title merchant-detail-timeline__title--${step.state}`}>
                        {step.title}
                      </Text>
                      {step.time ? <Text className='merchant-detail-timeline__time'>{step.time}</Text> : null}
                    </View>
                    {step.note ? <Text className='merchant-detail-timeline__note'>{step.note}</Text> : null}
                  </View>
                </View>
              ))}
            </View>
          </View>

          <View className='merchant-detail-photo-card'>
            <View className='merchant-detail-photo-card__header'>
              <Text className='merchant-detail-section-title'>配送照片</Text>
              <View className='merchant-detail-photo-card__privacy'>
                <Text>锁 仅本单商家与骑手可见</Text>
              </View>
            </View>
            <View className='merchant-detail-photo-grid'>
              <View className='merchant-detail-photo-item' onClick={() => showStaticToast('取货照静态展示')}>
                <View className={`merchant-detail-photo-item__thumb ${pickupUploaded ? 'merchant-detail-photo-item__thumb--ready' : 'merchant-detail-photo-item__thumb--pending'}`}>
                  <Text>{pickupUploaded ? '取货照 · 骑手拍摄' : '骑手取货后上传'}</Text>
                </View>
                <Text className={`merchant-detail-photo-item__caption ${pickupUploaded ? 'merchant-detail-photo-item__caption--ready' : 'merchant-detail-photo-item__caption--pending'}`}>
                  {pickupUploaded ? '✓ 取货照 · 已上传' : '· 取货照 · 待上传'}
                </Text>
              </View>
              <View className='merchant-detail-photo-item' onClick={() => showStaticToast('送达照待上传')}>
                <View className={`merchant-detail-photo-item__thumb ${deliveredUploaded ? 'merchant-detail-photo-item__thumb--ready' : 'merchant-detail-photo-item__thumb--pending'}`}>
                  <Text className='merchant-detail-photo-item__camera'>□</Text>
                  <Text>{deliveredUploaded ? '送达照 · 骑手拍摄' : '送达后由骑手上传'}</Text>
                </View>
                <Text className={`merchant-detail-photo-item__caption ${deliveredUploaded ? 'merchant-detail-photo-item__caption--ready' : 'merchant-detail-photo-item__caption--pending'}`}>
                  {deliveredUploaded ? '✓ 送达照 · 已上传' : '· 送达照 · 待上传'}
                </Text>
              </View>
            </View>
          </View>
        </View>
      </ScrollView>
    </View>
  )
}
