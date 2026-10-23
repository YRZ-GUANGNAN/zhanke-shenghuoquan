import React, { useEffect, useRef, useState } from 'react'
import { Text, View } from '@tarojs/components'
import { AppState, AppSkeleton } from './ui'
import AppIcon from './app-icon'
import { ResolvedMediaImage } from '../media-image'
import { request } from '../services/api'
import type { BuyerDish, BuyerMerchant, Page } from '../services/g-takeout-buyer'
import { centsToYuan } from '../services/g-takeout-buyer'
import Taro from '../services/taro'
import { useSessionStore } from '../stores/session'
import './service-merchant-catalog.scss'

// Catalog reads are deliberately separate from the legacy ordering workflow.
export default function ServiceMerchantCatalog({ merchantId = '' }: { merchantId?: string }) {
  const account = useSessionStore(state => state.profile.id)
  const [shops, setShops] = useState<BuyerMerchant[]>([])
  const [shop, setShop] = useState<BuyerMerchant | null>(null)
  const [dishes, setDishes] = useState<BuyerDish[]>([])
  const [page, setPage] = useState(0)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const version = useRef(0)
  const busy = useRef(false)
  const key = `${account}:${merchantId}`
  const activeKey = useRef(key)
  activeKey.current = key
  const load = async (append = false) => {
    if (busy.current) return
    busy.current = true
    const sequence = ++version.current
    const current = () => sequence === version.current && activeKey.current === key
    setLoading(true); setError('')
    const nextPage = append ? page + 1 : 1
    try {
      const base = '/takeout/catalog/merchants'
      if (merchantId) {
        const [merchant, result] = await Promise.all([
          request<BuyerMerchant>(`${base}/${encodeURIComponent(merchantId)}`, { url: '', timeout: 10000 }),
          request<Page<BuyerDish>>(`${base}/${encodeURIComponent(merchantId)}/dishes?page=${nextPage}&pageSize=20`, { url: '', timeout: 10000 })
        ])
        if (!current()) return
        setShop(merchant)
        setDishes(old => append ? Array.from(new Map([...old, ...result.items].map(item => [item.dishPublicId, item])).values()) : result.items)
        setTotal(result.total)
      } else {
        const result = await request<Page<BuyerMerchant>>(`${base}?page=${nextPage}&pageSize=20`, { url: '', timeout: 10000 })
        if (!current()) return
        setShops(old => append ? Array.from(new Map([...old, ...result.items].map(item => [item.merchantPublicId, item])).values()) : result.items)
        setTotal(result.total)
      }
      setPage(nextPage)
    } catch {
      if (current()) setError(merchantId ? '商品暂时无法加载，请稍后重试' : '校内商家暂时无法加载，请稍后重试')
    } finally {
      if (current()) { busy.current = false; setLoading(false) }
    }
  }
  useEffect(() => {
    version.current += 1; busy.current = false; setShops([]); setShop(null); setDishes([]); setPage(0); setTotal(0)
    void load()
    return () => { version.current += 1; busy.current = false }
  }, [key])
  const count = merchantId ? dishes.length : shops.length
  return <View className={`service-catalog${merchantId ? ' service-catalog--merchant' : ''}`}>
    <View className='service-catalog__heading'><Text>{merchantId ? shop?.name || '商家商品' : '校内商家'}</Text><Text>{merchantId ? '在售商品展示' : '查看商家与商品'}</Text></View>
    {shop ? <Text className='service-catalog__description'>{[shop.location, shop.businessHours].filter(Boolean).join(' · ')}</Text> : null}
    {loading && page === 0 ? <AppSkeleton rows={2} /> : null}
    {!loading && !error && count === 0 ? <AppState kind='empty' title={merchantId ? '商家暂未上架商品' : '暂无展示中的校内商家'} description={merchantId ? '商家上架后，商品会显示在这里。' : '可继续查看下方合作商家小程序。'} /> : null}
    {error ? <AppState kind='error' title='加载未完成' description={error} actionText='重试' onAction={() => void load(page > 0)} /> : null}
    {merchantId ? dishes.map(item => <View key={item.dishPublicId} className='service-catalog__dish'>
      <ResolvedMediaImage contentUrl={item.imageContentUrl} className='service-catalog__dish-image' placeholder='商品' />
      <View className='service-catalog__dish-main'><Text className='service-catalog__name'>{item.name}</Text><Text className='service-catalog__description'>{item.description}</Text><Text className='service-catalog__price'><Text className='service-catalog__currency'>¥</Text>{centsToYuan(item.priceCents)}</Text></View>
    </View>) : shops.map(item => <View key={item.merchantPublicId} className='service-catalog__shop' role='button' aria-label={`查看${item.name}商品`} hoverClass='service-catalog__shop--pressed'
      onClick={() => void Taro.navigateTo({ url: `/pages/student-takeout/index?merchantId=${encodeURIComponent(item.merchantPublicId)}` }).catch(() => Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' }))}>
      <ResolvedMediaImage contentUrl={item.logoContentUrl} className='service-catalog__logo' placeholder={item.name.slice(0, 1)} />
      <View className='service-catalog__shop-main'><Text className='service-catalog__name'>{item.name}</Text><Text className='service-catalog__description'>{item.location}</Text><Text className='service-catalog__hours'>{item.businessHours || '营业时间以商家为准'}</Text></View><AppIcon name='chevron-right' />
    </View>)}
    {count < total ? <View className='service-catalog__more' role='button' aria-label='加载更多' onClick={() => void load(true)}>{loading ? '加载中…' : '加载更多'}</View> : null}
  </View>
}
