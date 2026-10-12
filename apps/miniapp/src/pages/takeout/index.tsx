import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useEffect, useState } from 'react'
import { Button, ScrollView, Text, View } from '@tarojs/components'
import { BottomNav } from '../../components'
import { ResolvedMediaImage } from '../../media-image'
import {
  centsToYuan,
  getBuyerMerchant,
  listBuyerDishes,
  listBuyerMerchants,
  type BuyerDish,
  type BuyerMerchant
} from '../../services/g-takeout-buyer'
import Taro from '../../services/taro'
import { useTakeoutCartStore } from '../../stores/cart'
import './index.scss'

function TakeoutPage() {
  const [loading, setLoading] = useState(false)
  const [merchants, setMerchants] = useState<BuyerMerchant[]>([])
  const [selectedMerchant, setSelectedMerchant] = useState<BuyerMerchant | null>(null)
  const [dishes, setDishes] = useState<BuyerDish[]>([])
  const addDish = useTakeoutCartStore((state) => state.addDish)
  const cartQuantity = useTakeoutCartStore((state) => state.totalQuantity())
  const cartTotal = useTakeoutCartStore((state) => state.totalCents())
  const cartMerchantId = useTakeoutCartStore((state) => state.merchantPublicId)

  useEffect(() => {
    void loadMerchants()
  }, [])

  const loadMerchants = async () => {
    setLoading(true)
    try {
      const page = await listBuyerMerchants()
      setMerchants(page.items)
      if (!selectedMerchant && page.items[0]) {
        await openMerchant(page.items[0])
      }
    } catch (error) {
      showError(error, '外卖商家加载失败')
    } finally {
      setLoading(false)
    }
  }

  const openMerchant = async (merchant: BuyerMerchant) => {
    setLoading(true)
    try {
      const [detail, dishPage] = await Promise.all([
        getBuyerMerchant(merchant.merchantPublicId),
        listBuyerDishes(merchant.merchantPublicId)
      ])
      setSelectedMerchant(detail)
      setDishes(dishPage.items)
    } catch (error) {
      showError(error, '商家菜品加载失败')
    } finally {
      setLoading(false)
    }
  }

  const addToCart = async (dish: BuyerDish) => {
    if (!selectedMerchant) return
    const switchingMerchant = cartMerchantId && cartMerchantId !== selectedMerchant.merchantPublicId
    addDish(selectedMerchant, dish)
    await Taro.showToast({ title: switchingMerchant ? '已切换店铺购物车' : '已加入购物车', icon: 'success' })
  }

  return (
    <View className='page takeout-page'>
      <View className='takeout-hero'>
        <View>
          <Text className='takeout-hero__title'>校园外卖</Text>
          <Text className='takeout-hero__desc'>只看营业中的商家与已上架菜品，价格以下单后端重算为准。</Text>
        </View>
        <Button className='cc-button cc-button--small' onClick={() => Taro.navigateTo({ url: '/pages/takeout-orders/index' })}>
          我的订单
        </Button>
      </View>

      <ScrollView className='takeout-merchant-strip' scrollX enhanced showScrollbar={false}>
        <View className='takeout-merchant-strip__inner'>
          {merchants.map((merchant) => (
            <View
              key={merchant.merchantPublicId}
              className={`takeout-merchant-chip ${selectedMerchant?.merchantPublicId === merchant.merchantPublicId ? 'takeout-merchant-chip--active' : ''}`}
              onClick={() => void openMerchant(merchant)}
            >
              <ResolvedMediaImage className='takeout-merchant-chip__logo' contentUrl={merchant.logoContentUrl} placeholder='店' />
              <Text>{merchant.name}</Text>
            </View>
          ))}
        </View>
      </ScrollView>

      {loading && <View className='card takeout-card'>正在加载...</View>}
      {!loading && merchants.length === 0 && <View className='card takeout-card'>暂无营业商家</View>}

      {selectedMerchant && (
        <View className='card takeout-shop-card'>
          <ResolvedMediaImage className='takeout-shop-card__logo' contentUrl={selectedMerchant.logoContentUrl} placeholder='店铺' />
          <View className='takeout-shop-card__body'>
            <Text className='takeout-shop-card__name'>{selectedMerchant.name}</Text>
            <Text className='takeout-muted'>{selectedMerchant.location || '校园内'} · {selectedMerchant.businessHours || '营业时间未填写'}</Text>
            <Text className='takeout-muted'>联系方式：{selectedMerchant.contactPhone || '-'}</Text>
          </View>
        </View>
      )}

      <View className='takeout-section-title'>已上架菜品</View>
      <View className='takeout-dish-list'>
        {dishes.map((dish) => (
          <View key={dish.dishPublicId} className='card takeout-dish-card'>
            <ResolvedMediaImage className='takeout-dish-card__image' contentUrl={dish.imageContentUrl} placeholder='菜品图' />
            <View className='takeout-dish-card__body'>
              <View className='between'>
                <Text className='takeout-dish-card__name'>{dish.name}</Text>
                <Text className='takeout-price'>¥{centsToYuan(dish.priceCents)}</Text>
              </View>
              <Text className='takeout-muted'>{dish.description || '暂无描述'}</Text>
              <View className='between takeout-dish-card__actions'>
                <Text className='takeout-muted'>图源：resolve → COS 短签 URL</Text>
                <Button className='cc-button cc-button--small cc-button--primary' onClick={() => void addToCart(dish)}>加购</Button>
              </View>
            </View>
          </View>
        ))}
      </View>

      {cartQuantity > 0 && (
        <View className='takeout-cart-bar card' onClick={() => Taro.navigateTo({ url: '/pages/takeout-cart/index' })}>
          <Text>{cartQuantity} 件 · ¥{centsToYuan(cartTotal)}</Text>
          <Button className='cc-button cc-button--small cc-button--primary'>去结算</Button>
        </View>
      )}

      <BottomNav active='takeout' />
    </View>
  )
}

function showError(error: unknown, fallback: string) {
  void Taro.showToast({ title: error instanceof Error ? error.message : fallback, icon: 'none' })
}

export default withFeaturePage(TakeoutPage)
