import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type RiderTab = 'grab' | 'pickup' | 'delivery'
type ProofKind = 'pickup' | 'delivery'

interface GrabOrder {
  id: string
  time: string
  shop: string
  floor: string
  remark?: string
  phone: string
}

interface AssignedOrder {
  id: string
  time: string
  shop: string
  address: string
  phone: string
  remark?: string
  actionText: string
  proofKind: ProofKind
}

interface ProofCopy {
  title: string
  desc: string
  placeholder: string
  confirmText: string
  successText: string
}

const tabs: Array<{ key: RiderTab; label: string; count: number }> = [
  { key: 'grab', label: '待抢单', count: 5 },
  { key: 'pickup', label: '待取货', count: 1 },
  { key: 'delivery', label: '待送达', count: 1 }
]

const grabOrders: GrabOrder[] = [
  {
    id: '#A2048',
    time: '12:32下单',
    shop: '蜜雪冰城·大学城店',
    floor: '3栋 · 4楼',
    remark: '冰淇淋打包好别洒了，放门口就行',
    phone: '138****2048'
  },
  {
    id: '#A2051',
    time: '12:35下单',
    shop: '华莱士·西门店',
    floor: '7栋 · 2楼',
    phone: '176****0093'
  },
  {
    id: '#A2053',
    time: '12:38下单',
    shop: '塔斯汀·大学城店',
    floor: '5栋 · 4楼',
    remark: '麻烦快一点，到了打电话',
    phone: '150****3390'
  }
]

const pickupOrders: AssignedOrder[] = [
  {
    id: '#A2044',
    time: '12:25下单',
    shop: '瑞幸咖啡·图书馆店',
    address: '11栋 3楼 302室',
    phone: '137 5560 8814',
    actionText: '确认取货',
    proofKind: 'pickup'
  }
]

const deliveryOrders: AssignedOrder[] = [
  {
    id: '#A2039',
    time: '12:18下单',
    shop: '塔斯汀·大学城店',
    address: '5栋 4楼 408室',
    remark: '到楼下打个电话',
    phone: '150 7712 3390',
    actionText: '确认送达',
    proofKind: 'delivery'
  }
]

const proofCopy: Record<ProofKind, ProofCopy> = {
  pickup: {
    title: '上传取货照',
    desc: '请拍摄商品完整外观，便于商家核对订单',
    placeholder: '照片 · 商品实拍',
    confirmText: '确认取货',
    successText: '已确认取货（静态示意）'
  },
  delivery: {
    title: '上传送达照',
    desc: '请拍摄放置位置（含门牌号），作为送达凭证',
    placeholder: '照片 · 送达位置实拍',
    confirmText: '确认送达',
    successText: '已确认送达（静态示意）'
  }
}

function showStaticToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function DeliveryRiderPage() {
  const [activeTab, setActiveTab] = useState<RiderTab>('grab')
  const [proofKind, setProofKind] = useState<ProofKind | null>(null)
  const [photoCaptured, setPhotoCaptured] = useState(false)

  const openProofSheet = (kind: ProofKind) => {
    setProofKind(kind)
    setPhotoCaptured(false)
  }

  const closeProofSheet = () => {
    setProofKind(null)
    setPhotoCaptured(false)
  }

  const confirmProof = () => {
    if (!proofKind) return
    if (!photoCaptured) {
      showStaticToast('请先拍照（静态示意）')
      return
    }
    showStaticToast(proofCopy[proofKind].successText)
    closeProofSheet()
  }

  const renderAssignedOrders = (orders: AssignedOrder[]) => (
    <View className='rider-grab-list rider-grab-list--assigned'>
      {orders.map((order) => (
        <View key={order.id} className='rider-order-card rider-order-card--assigned'>
          <View className='rider-order-card__top'>
            <View className='rider-order-card__serial'>
              <Text>{order.id}</Text>
              <Text className='rider-order-card__dot'>·</Text>
              <Text>{order.time}</Text>
            </View>
            <View className='rider-order-card__fee'>
              <Text>配送费 </Text>
              <Text className='rider-order-card__fee-price'>¥1.5</Text>
            </View>
          </View>

          <View className='rider-order-route rider-order-route--assigned'>
            <View className='rider-order-route__line' />
            <View className='rider-order-route__row'>
              <View className='rider-order-route__marker rider-order-route__marker--pickup' />
              <View className='rider-order-route__info'>
                <Text className='rider-order-route__label'>取货点</Text>
                <Text className='rider-order-route__place'>{order.shop}</Text>
              </View>
            </View>
            <View className='rider-order-route__row rider-order-route__row--drop'>
              <View className='rider-order-route__marker rider-order-route__marker--drop' />
              <View className='rider-order-route__info'>
                <Text className='rider-order-route__label'>送达点</Text>
                <View className='rider-order-route__drop-row'>
                  <Text className='rider-order-route__place'>{order.address}</Text>
                  <Text className='rider-order-route__complete'>✓ 完整门牌</Text>
                </View>
              </View>
            </View>
          </View>

          {order.remark && (
            <View className='rider-order-card__remark rider-order-card__remark--assigned'>
              <Text className='rider-order-card__remark-label'>备注</Text>
              <Text>{order.remark}</Text>
            </View>
          )}

          <View className='rider-order-card__bottom rider-order-card__bottom--assigned'>
            <View className='rider-order-card__phone'>
              <Text className='rider-order-card__phone-icon'>⌕</Text>
              <Text>{order.phone}</Text>
            </View>
            <View className='rider-order-card__actions'>
              <View className='rider-order-card__message' onClick={() => showStaticToast('消息入口静态占位')}>
                <Text>□</Text>
              </View>
              <View className='rider-order-card__grab' onClick={() => openProofSheet(order.proofKind)}>
                <Text>{order.actionText}</Text>
              </View>
            </View>
          </View>
        </View>
      ))}
    </View>
  )

  const copy = proofKind ? proofCopy[proofKind] : null

  return (
    <View className='rider-grab-page'>
      <View className='rider-grab-hero'>
        <View className='rider-grab-hero__orb rider-grab-hero__orb--one' />
        <View className='rider-grab-hero__orb rider-grab-hero__orb--two' />

        <View className='rider-grab-user'>
          <View className='rider-grab-user__avatar'>
            <Text>李</Text>
          </View>
          <View className='rider-grab-user__main'>
            <View className='rider-grab-user__name-row'>
              <Text className='rider-grab-user__name'>李伟</Text>
              <Text className='rider-grab-user__badge'>骑手</Text>
            </View>
            <Text className='rider-grab-user__meta'>已认证 · 湛江科技学院</Text>
          </View>
          <View
            className='rider-grab-user__switch'
            onClick={() => showStaticToast('身份切换阶段二接入')}
          >
            <Text className='rider-grab-user__switch-icon'>⇄</Text>
            <Text>切换身份</Text>
          </View>
        </View>

        <View className='rider-grab-wallet-summary'>
          <View className='rider-grab-wallet-summary__main'>
            <Text className='rider-grab-wallet-summary__label'>钱包余额（元）</Text>
            <View className='rider-grab-wallet-summary__amount-row'>
              <Text className='rider-grab-wallet-summary__currency'>¥</Text>
              <Text className='rider-grab-wallet-summary__amount'>128.50</Text>
            </View>
            <Text className='rider-grab-wallet-summary__sub'>今日收入 ¥18.00　｜　已完成 3 单</Text>
          </View>
          <View
            className='rider-grab-wallet-summary__button'
            onClick={() => void Taro.navigateTo({ url: '/pages/rider-wallet/index' })}
          >
            <View className='rider-grab-wallet-summary__wallet-icon' />
            <Text>钱包</Text>
          </View>
        </View>
      </View>

      <View className='rider-grab-tabs'>
        {tabs.map((tab) => (
          <View
            key={tab.key}
            className={`rider-grab-tabs__item ${activeTab === tab.key ? 'rider-grab-tabs__item--active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            <Text>{tab.label}</Text>
            <Text className={`rider-grab-tabs__count rider-grab-tabs__count--${tab.key}`}>{tab.count}</Text>
          </View>
        ))}
      </View>

      <View className='rider-grab-content'>
        {activeTab === 'grab' && (
          <>
            <View className='rider-grab-list-head'>
              <Text>手快有 · 共 5 单可抢</Text>
              <View className='rider-grab-list-head__refresh' onClick={() => showStaticToast('刷新静态占位')}>
                <Text>↻</Text>
                <Text>刷新</Text>
              </View>
            </View>
            <View className='rider-grab-list'>
              {grabOrders.map((order) => (
                <View key={order.id} className='rider-order-card'>
                  <View className='rider-order-card__top'>
                    <View className='rider-order-card__serial'>
                      <Text>{order.id}</Text>
                      <Text className='rider-order-card__dot'>·</Text>
                      <Text>{order.time}</Text>
                    </View>
                    <View className='rider-order-card__fee'>
                      <Text>配送费 </Text>
                      <Text className='rider-order-card__fee-price'>¥1.5</Text>
                    </View>
                  </View>

                  <View className='rider-order-route'>
                    <View className='rider-order-route__line' />
                    <View className='rider-order-route__row'>
                      <View className='rider-order-route__marker rider-order-route__marker--pickup' />
                      <View className='rider-order-route__info'>
                        <Text className='rider-order-route__label'>取货点</Text>
                        <Text className='rider-order-route__place'>{order.shop}</Text>
                      </View>
                    </View>
                    <View className='rider-order-route__row rider-order-route__row--drop'>
                      <View className='rider-order-route__marker rider-order-route__marker--drop' />
                      <View className='rider-order-route__info'>
                        <Text className='rider-order-route__label'>送达点</Text>
                        <View className='rider-order-route__drop-row'>
                          <Text className='rider-order-route__place'>{order.floor}</Text>
                          <Text className='rider-order-route__lock'>锁 抢单后显示门牌</Text>
                        </View>
                      </View>
                    </View>
                  </View>

                  {order.remark && (
                    <View className='rider-order-card__remark'>
                      <Text className='rider-order-card__remark-label'>备注</Text>
                      <Text>{order.remark}</Text>
                    </View>
                  )}

                  <View className='rider-order-card__bottom'>
                    <View className='rider-order-card__phone'>
                      <Text className='rider-order-card__phone-icon'>⌕</Text>
                      <Text>{order.phone}</Text>
                    </View>
                    <View
                      className='rider-order-card__grab'
                      onClick={() => {
                        showStaticToast('抢单成功（静态示意）')
                        setActiveTab('pickup')
                      }}
                    >
                      <Text>抢单</Text>
                    </View>
                  </View>
                </View>
              ))}
            </View>
          </>
        )}

        {activeTab === 'pickup' && renderAssignedOrders(pickupOrders)}
        {activeTab === 'delivery' && renderAssignedOrders(deliveryOrders)}
      </View>

      {copy && (
        <View className='rider-proof-mask'>
          <View className='rider-proof-sheet'>
            <View className='rider-proof-sheet__top'>
              <Text className='rider-proof-sheet__title'>{copy.title}</Text>
              <View className='rider-proof-sheet__close' onClick={closeProofSheet}>
                <Text>×</Text>
              </View>
            </View>
            <Text className='rider-proof-sheet__desc'>{copy.desc}</Text>

            <View
              className={`rider-proof-uploader ${photoCaptured ? 'rider-proof-uploader--captured' : ''}`}
              onClick={() => setPhotoCaptured(true)}
            >
              {photoCaptured ? (
                <>
                  <View className='rider-proof-uploader__retake'>
                    <Text>↻ 重拍</Text>
                  </View>
                  <Text className='rider-proof-uploader__photo-text'>{copy.placeholder}</Text>
                  <View className='rider-proof-uploader__done'>
                    <Text>✓ 已拍摄</Text>
                  </View>
                </>
              ) : (
                <>
                  <View className='rider-proof-uploader__camera'>
                    <Text>📷</Text>
                  </View>
                  <Text className='rider-proof-uploader__main'>点击拍照</Text>
                  <Text className='rider-proof-uploader__hint'>示意交互，开发时调用微信相机</Text>
                </>
              )}
            </View>

            <Text className='rider-proof-sheet__privacy'>锁 照片仅本单商家与你可见</Text>
            <View
              className={`rider-proof-confirm ${photoCaptured ? 'rider-proof-confirm--active' : 'rider-proof-confirm--disabled'}`}
              onClick={confirmProof}
            >
              <Text>{copy.confirmText}</Text>
            </View>
          </View>
        </View>
      )}
    </View>
  )
}
