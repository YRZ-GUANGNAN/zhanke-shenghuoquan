import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import StudentBottomNav from '../../components/student-bottom-nav'
import {
  listExternalTakeoutMerchants,
  type ExternalTakeoutMerchantLink,
} from '../../services/d-navigation'
import Taro from '../../services/taro'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

type DirectoryStatus = 'loading' | 'empty' | 'ready' | 'error'
type MerchantFilter = 'all' | 'open' | 'resting'

const filters: Array<{ key: MerchantFilter; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'open', label: '营业中' },
  { key: 'resting', label: '休息中' },
]

export default function StudentTakeoutPage() {
  const [merchants, setMerchants] = useState<ExternalTakeoutMerchantLink[]>([])
  const [status, setStatus] = useState<DirectoryStatus>('loading')
  const [filter, setFilter] = useState<MerchantFilter>('all')
  const [jumpingMerchantId, setJumpingMerchantId] = useState<string | null>(null)
  const requestVersion = useRef(0)
  const jumpingRef = useRef<string | null>(null)

  const openCount = useMemo(
    () => merchants.filter((merchant) => merchant.operatingStatus === 'open').length,
    [merchants],
  )
  const restingCount = merchants.length - openCount
  const filteredMerchants = useMemo(() => {
    if (filter === 'all') return merchants
    return merchants.filter((merchant) => merchant.operatingStatus === filter)
  }, [filter, merchants])

  const load = useCallback(async () => {
    const version = requestVersion.current + 1
    requestVersion.current = version
    setStatus('loading')
    try {
      const values = await listExternalTakeoutMerchants()
      if (version !== requestVersion.current) return
      setMerchants(values)
      setStatus(values.length > 0 ? 'ready' : 'empty')
    } catch (error) {
      console.error('load external takeout directory failed', error)
      if (version !== requestVersion.current) return
      setMerchants([])
      setStatus('error')
    }
  }, [])

  useDidShow(() => {
    void load()
  })

  useEffect(() => () => {
    requestVersion.current += 1
  }, [])

  const openMerchant = async (merchant: ExternalTakeoutMerchantLink) => {
    if (merchant.operatingStatus === 'resting') {
      await Taro.showToast({ title: '商家休息中', icon: 'none' })
      return
    }
    if (jumpingRef.current) return
    jumpingRef.current = merchant.id
    setJumpingMerchantId(merchant.id)
    try {
      await Taro.navigateToMiniProgram({ appId: merchant.appId, path: merchant.path })
    } catch (error) {
      console.error('open external merchant mini program failed', error)
      await Taro.showToast({ title: '商家小程序暂时无法打开', icon: 'none' })
    } finally {
      jumpingRef.current = null
      setJumpingMerchantId(null)
    }
  }

  const filterCount = (key: MerchantFilter) => {
    if (key === 'open') return openCount
    if (key === 'resting') return restingCount
    return merchants.length
  }

  return (
    <View className='student-takeout-page' style={getCapsuleSafeStyle()}>
      <View className='student-takeout-header'>
        <CapsulePageHeader
          variant='search'
          className='student-takeout-header__row'
          left={<Text className='student-takeout-header__title'>外卖</Text>}
          center={
            <View
              className='student-takeout-search'
              hoverClass='student-takeout-search--pressed'
              onClick={() => void Taro.navigateTo({ url: '/pages/search/index?scope=takeout' })}
            >
              <View className='student-takeout-search__icon' />
              <Text>搜索合作商家</Text>
            </View>
          }
          divider={false}
        />
        <View className='student-takeout-header__summary'>
          <Text>校外合作商家目录</Text>
          <Text>{status === 'ready' ? `共 ${merchants.length} 家 · 营业中 ${openCount} 家` : '商品与交易由商家小程序提供'}</Text>
        </View>
      </View>

      <View className='student-takeout-scroll'>
        {status === 'ready' && (
          <View className='student-takeout-filters'>
            {filters.map((item) => (
              <View
                key={item.key}
                className={`student-takeout-filter${filter === item.key ? ' student-takeout-filter--active' : ''}`}
                onClick={() => setFilter(item.key)}
              >
                <Text>{item.label}</Text>
                <Text className='student-takeout-filter__count'>{filterCount(item.key)}</Text>
              </View>
            ))}
          </View>
        )}

        <View className='student-takeout-tip'>
          <Text className='student-takeout-tip__icon'>i</Text>
          <Text className='student-takeout-tip__text'>点击后将前往商家小程序，下单与售后由商家提供。</Text>
        </View>

        {status === 'loading' && (
          <View className='student-takeout-state'>
            <Text className='student-takeout-state__title'>正在读取合作商家</Text>
            <Text className='student-takeout-state__desc'>请稍候</Text>
          </View>
        )}
        {status === 'empty' && (
          <View className='student-takeout-state'>
            <Text className='student-takeout-state__title'>暂无合作商家</Text>
            <Text className='student-takeout-state__desc'>新的合作商家会显示在这里</Text>
          </View>
        )}
        {status === 'error' && (
          <View className='student-takeout-state student-takeout-state--retry' onClick={() => void load()}>
            <Text className='student-takeout-state__title'>商家加载失败，点击重试</Text>
            <Text className='student-takeout-state__desc'>请检查网络连接后重试</Text>
          </View>
        )}
        {status === 'ready' && filteredMerchants.length === 0 && (
          <View className='student-takeout-state'>
            <Text className='student-takeout-state__title'>当前分类暂无商家</Text>
            <Text className='student-takeout-state__desc'>可以切换其他营业状态查看</Text>
          </View>
        )}
        {status === 'ready' && filteredMerchants.length > 0 && (
          <View className='student-takeout-list'>
            {filteredMerchants.map((merchant) => {
              const resting = merchant.operatingStatus === 'resting'
              const jumping = jumpingMerchantId === merchant.id
              return (
                <View
                  key={merchant.id}
                  className={`student-takeout-merchant${resting ? ' student-takeout-merchant--resting' : ''}`}
                  hoverClass={resting ? 'none' : 'student-takeout-merchant--pressed'}
                  onClick={() => void openMerchant(merchant)}
                >
                  <View className='student-takeout-merchant__top'>
                    <View className='student-takeout-merchant__avatar' style={{ backgroundColor: merchant.color }}>
                      <Text>{merchant.iconText}</Text>
                    </View>
                    <View className='student-takeout-merchant__main'>
                      <Text className='student-takeout-merchant__name'>{merchant.title}</Text>
                      <Text className='student-takeout-merchant__desc'>{merchant.subtitle}</Text>
                    </View>
                    <View className={`student-takeout-status ${resting ? 'student-takeout-status--closed' : 'student-takeout-status--open'}`}>
                      <View className='student-takeout-status__dot' />
                      <Text>{resting ? '休息中' : '营业中'}</Text>
                    </View>
                  </View>
                  <View className='student-takeout-merchant__bottom'>
                    <Text className='student-takeout-merchant__hint'>微信小程序商家</Text>
                    <View className={`student-takeout-merchant__action${resting ? ' student-takeout-merchant__action--disabled' : ''}`}>
                      <Text>{jumping ? '打开中' : resting ? '暂未营业' : '进入商家'}</Text>
                      {!resting && <AppIcon name='chevron-right' className='student-takeout-merchant__arrow app-icon--chevron' />}
                    </View>
                  </View>
                </View>
              )
            })}
          </View>
        )}

        {status === 'ready' && <Text className='student-takeout-footer'>— 已显示全部合作商家 —</Text>}
      </View>

      <StudentBottomNav active='takeout' />
    </View>
  )
}
