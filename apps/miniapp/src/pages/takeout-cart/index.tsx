import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useState } from 'react'
import { Button, Input, Text, Textarea, View } from '@tarojs/components'
import { ResolvedMediaImage } from '../../media-image'
import { centsToYuan, createBuyerOrder } from '../../services/g-takeout-buyer'
import Taro from '../../services/taro'
import { useTakeoutCartStore } from '../../stores/cart'
import './index.scss'

function TakeoutCartPage() {
  const merchantPublicId = useTakeoutCartStore((state) => state.merchantPublicId)
  const merchantName = useTakeoutCartStore((state) => state.merchantName)
  const items = useTakeoutCartStore((state) => state.items)
  const increment = useTakeoutCartStore((state) => state.increment)
  const decrement = useTakeoutCartStore((state) => state.decrement)
  const clear = useTakeoutCartStore((state) => state.clear)
  const totalCents = useTakeoutCartStore((state) => state.totalCents())
  const [contactName, setContactName] = useState('')
  const [contactPhone, setContactPhone] = useState('')
  const [addressBuilding, setAddressBuilding] = useState('')
  const [addressRoom, setAddressRoom] = useState('')
  const [remark, setRemark] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const submit = async () => {
    if (items.length === 0 || !merchantPublicId) return void Taro.showToast({ title: '购物车为空', icon: 'none' })
    if (!contactName.trim() || !contactPhone.trim() || !addressBuilding.trim()) {
      return void Taro.showToast({ title: '请填写联系人、电话和楼栋', icon: 'none' })
    }
    setSubmitting(true)
    try {
      const detail = await createBuyerOrder({
        merchantPublicId,
        items: items.map((item) => ({ dishPublicId: item.dishPublicId, quantity: item.quantity })),
        address: {
          contactName: contactName.trim(),
          contactPhone: contactPhone.trim(),
          addressBuilding: addressBuilding.trim(),
          addressRoom: addressRoom.trim()
        },
        remark: remark.trim()
      })
      clear()
      await Taro.showToast({ title: '订单已提交', icon: 'success' })
      await Taro.redirectTo({ url: `/pages/takeout-order-detail/index?id=${detail.order.orderPublicId}` })
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '下单失败', icon: 'none' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <View className='page takeout-cart-page'>
      <View className='card cart-card'>
        <Text className='cart-title'>{merchantName || '外卖购物车'}</Text>
        <Text className='cart-tip'>订单将停留在 pending_payment，支付能力留待 G-④。</Text>
      </View>

      {items.length === 0 && (
        <View className='card cart-card'>
          <Text>购物车为空</Text>
          <Button className='cc-button cc-button--primary cc-button--block' onClick={() => Taro.redirectTo({ url: '/pages/takeout/index' })}>去点餐</Button>
        </View>
      )}

      {items.map((item) => (
        <View key={item.dishPublicId} className='card cart-item'>
          <ResolvedMediaImage className='cart-item__image' contentUrl={item.imageContentUrl} placeholder='菜品图' />
          <View className='cart-item__body'>
            <View className='between'>
              <Text className='cart-item__name'>{item.name}</Text>
              <Text className='cart-price'>¥{centsToYuan(item.priceCents * item.quantity)}</Text>
            </View>
            <Text className='cart-tip'>单价 ¥{centsToYuan(item.priceCents)}</Text>
            <View className='cart-quantity'>
              <Button className='cc-button cc-button--small' onClick={() => decrement(item.dishPublicId)}>-</Button>
              <Text>{item.quantity}</Text>
              <Button className='cc-button cc-button--small' onClick={() => increment(item.dishPublicId)}>+</Button>
            </View>
          </View>
        </View>
      ))}

      {items.length > 0 && (
        <View className='card cart-card'>
          <Text className='cart-title'>配送信息</Text>
          <Input className='cc-field' value={contactName} placeholder='联系人' onInput={(event) => setContactName(String(event.detail.value))} />
          <Input className='cc-field' value={contactPhone} placeholder='手机号' onInput={(event) => setContactPhone(String(event.detail.value))} />
          <Input className='cc-field' value={addressBuilding} placeholder='楼栋/地点' onInput={(event) => setAddressBuilding(String(event.detail.value))} />
          <Input className='cc-field' value={addressRoom} placeholder='房间号/备注位置' onInput={(event) => setAddressRoom(String(event.detail.value))} />
          <Textarea className='cc-textarea' value={remark} placeholder='备注，如少辣、放门口' maxlength={200} onInput={(event) => setRemark(String(event.detail.value))} />
          <View className='between cart-total'>
            <Text>合计 ¥{centsToYuan(totalCents)}</Text>
            <Button className='cc-button cc-button--primary' disabled={submitting} onClick={() => void submit()}>
              {submitting ? '提交中...' : '提交订单'}
            </Button>
          </View>
        </View>
      )}
    </View>
  )
}

export default withFeaturePage(TakeoutCartPage)
