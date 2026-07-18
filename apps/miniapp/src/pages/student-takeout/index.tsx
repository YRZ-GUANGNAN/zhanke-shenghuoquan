import { useCallback, useEffect, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Text, View } from '@tarojs/components'
import StudentBottomNav from '../../components/student-bottom-nav'
import {
  listExternalTakeoutMerchants,
  type ExternalTakeoutMerchantLink,
} from '../../services/d-navigation'
import Taro from '../../services/taro'
import './index.scss'

type DirectoryStatus = 'loading' | 'empty' | 'ready' | 'error'

export default function StudentTakeoutPage() {
  const [merchants, setMerchants] = useState<ExternalTakeoutMerchantLink[]>([])
  const [status, setStatus] = useState<DirectoryStatus>('loading')
  const [jumpingMerchantId, setJumpingMerchantId] = useState<string | null>(null)
  const requestVersion = useRef(0)
  const jumpingRef = useRef<string | null>(null)

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

  return (
    <View className='student-takeout-page'>
      <View className='student-takeout-header'>
        <View className='student-takeout-header__row'>
          <Text className='student-takeout-header__title'>外卖</Text>
          <View
            className='student-takeout-search'
            onClick={() => void Taro.navigateTo({ url: '/pages/search/index?scope=takeout' })}
          >
            <View className='student-takeout-search__icon' />
            <Text>搜索商家</Text>
          </View>
        </View>
        <Text className='student-takeout-header__subtitle'>校内合作商家 · 点击进入对应商家小程序</Text>
      </View>

      <View className='student-takeout-scroll'>
        <View className='student-takeout-tip'>
          <Text className='student-takeout-tip__icon'>i</Text>
          <Text className='student-takeout-tip__text'>
            点击进入商家小程序，商品、价格、下单与售后由商家小程序提供。
          </Text>
        </View>

        {status === 'loading' && (
          <View className='student-takeout-state'><Text>正在读取合作商家</Text></View>
        )}
        {status === 'empty' && (
          <View className='student-takeout-state'><Text>暂无合作商家</Text></View>
        )}
        {status === 'error' && (
          <View className='student-takeout-state student-takeout-state--retry' onClick={() => void load()}>
            <Text>商家加载失败，点击重试</Text>
          </View>
        )}
        {status === 'ready' && (
          <View className='student-takeout-list'>
            {merchants.map((merchant) => {
              const resting = merchant.operatingStatus === 'resting'
              const jumping = jumpingMerchantId === merchant.id
              return (
                <View
                  key={merchant.id}
                  className={`student-takeout-merchant${resting ? ' student-takeout-merchant--resting' : ''}`}
                  onClick={() => void openMerchant(merchant)}
                >
                  <View className='student-takeout-merchant__avatar' style={{ backgroundColor: merchant.color }}>
                    <Text>{merchant.iconText}</Text>
                  </View>
                  <View className='student-takeout-merchant__main'>
                    <View className='student-takeout-merchant__title-row'>
                      <Text className='student-takeout-merchant__name'>{merchant.title}</Text>
                      <View className={`student-takeout-status ${resting ? 'student-takeout-status--closed' : 'student-takeout-status--open'}`}>
                        <View className='student-takeout-status__dot' />
                        <Text>{resting ? '休息中' : '营业中'}</Text>
                      </View>
                    </View>
                    <Text className='student-takeout-merchant__desc'>{merchant.subtitle}</Text>
                  </View>
                  <View className={`student-takeout-merchant__action${resting ? ' student-takeout-merchant__action--disabled' : ''}`}>
                    <Text>{jumping ? '打开中' : resting ? '暂休' : '小程序'}</Text>
                  </View>
                  <Text className='student-takeout-merchant__arrow'>›</Text>
                </View>
              )
            })}
          </View>
        )}

        <Text className='student-takeout-footer'>合作商家陆续入驻中</Text>
      </View>

      <StudentBottomNav active='takeout' />
    </View>
  )
}
