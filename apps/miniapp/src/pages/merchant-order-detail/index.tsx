import React from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type StepState = 'done' | 'current' | 'todo'

interface ProgressStep {
  key: string
  title: string
  note?: string
  time?: string
  state: StepState
}

const progressSteps: ProgressStep[] = [
  { key: 'created', title: '创建订单', time: '12:31', state: 'done' },
  { key: 'paid', title: '支付配送费', note: '¥1.50 · 钱包支付', time: '12:31', state: 'done' },
  { key: 'pool', title: '进入抢单池', note: '等待骑手抢单', time: '12:32', state: 'done' },
  { key: 'claimed', title: '骑手接单', note: '李伟 · 137 5560 8814', time: '12:35', state: 'done' },
  { key: 'picked', title: '已取货', note: '已上传取货照', time: '12:41', state: 'done' },
  { key: 'delivering', title: '配送中', note: '前往 3栋 4楼 412室', state: 'current' },
  { key: 'delivered', title: '送达', note: '骑手上传送达照后完成', state: 'todo' }
]

function showStaticToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function handleBack() {
  void Taro.navigateBack()
}

function openMerchantConversation() {
  void Taro.navigateTo({
    url: '/pages/delivery-conversation/index?orderId=B1079&role=merchant'
  })
}

export default function MerchantOrderDetailPage() {
  return (
    <View className='merchant-order-detail-page'>
      <View className='merchant-detail-nav'>
        <View className='merchant-detail-nav__back' onClick={handleBack}>
          <Text>‹</Text>
        </View>
        <Text className='merchant-detail-nav__title'>配送单详情</Text>
        <View className='merchant-detail-nav__spacer' />
      </View>

      <ScrollView scrollY className='merchant-detail-scroll' enhanced showScrollbar={false}>
        <View className='merchant-detail-content'>
          <View className='merchant-detail-status-card'>
            <View className='merchant-detail-status-card__main'>
              <View className='merchant-detail-status-card__state-row'>
                <Text className='merchant-detail-status-card__state'>配送中</Text>
                <View className='merchant-detail-status-card__dot' />
              </View>
              <Text className='merchant-detail-status-card__sub'>骑手已取货，正在前往送达点</Text>
            </View>
            <View className='merchant-detail-status-card__fee'>
              <Text className='merchant-detail-status-card__amount'>¥1.50</Text>
              <Text className='merchant-detail-status-card__pay'>钱包支付</Text>
            </View>
            <View className='merchant-detail-status-card__divider' />
            <View className='merchant-detail-status-card__meta'>
              <Text>#B1079 · 12:31 下单</Text>
              <Text>送达点: 3栋 4楼 412室</Text>
            </View>
          </View>

          <View className='merchant-detail-rider-card'>
            <View className='merchant-detail-rider-card__avatar'>
              <Text>李</Text>
            </View>
            <View className='merchant-detail-rider-card__main'>
              <Text className='merchant-detail-rider-card__name'>李伟 · 骑手 · 12:35 接单</Text>
              <Text className='merchant-detail-rider-card__phone'>137 5560 8814</Text>
            </View>
            <View className='merchant-detail-rider-card__actions'>
              <View className='merchant-detail-rider-card__circle' onClick={openMerchantConversation}>
                <Text>□</Text>
              </View>
              <View className='merchant-detail-rider-card__circle' onClick={() => showStaticToast('电话联系稍后开放')}>
                <Text>⌕</Text>
              </View>
            </View>
          </View>

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
                <View className='merchant-detail-photo-item__thumb merchant-detail-photo-item__thumb--ready'>
                  <Text>取货照 · 骑手拍摄</Text>
                </View>
                <Text className='merchant-detail-photo-item__caption merchant-detail-photo-item__caption--ready'>
                  ✓ 取货照 · 12:41 已上传
                </Text>
              </View>
              <View className='merchant-detail-photo-item' onClick={() => showStaticToast('送达照待上传')}>
                <View className='merchant-detail-photo-item__thumb merchant-detail-photo-item__thumb--pending'>
                  <Text className='merchant-detail-photo-item__camera'>□</Text>
                  <Text>送达后由骑手上传</Text>
                </View>
                <Text className='merchant-detail-photo-item__caption merchant-detail-photo-item__caption--pending'>
                  · 送达照 · 待上传
                </Text>
              </View>
            </View>
          </View>
        </View>
      </ScrollView>
    </View>
  )
}
