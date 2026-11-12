import { withFeaturePage } from '../../components/feature-page-gate'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppSkeleton, AppState } from '../../components/ui'
import { isMiniProgramJumpCancelled } from '../../utils/mini-program-jump'
import StudentBottomNav from '../../components/student-bottom-nav'
import {
  listExternalTakeoutMerchants,
  type ExternalTakeoutMerchantLink,
} from '../../services/d-navigation'
import Taro from '../../services/taro'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { useViewportRefresh } from '../../hooks/use-viewport-refresh'
import { usePageShare } from '../../hooks/use-page-share'
import './index.scss'

type DirectoryStatus = 'loading' | 'empty' | 'ready' | 'error'

function StudentTakeoutPage() {
  const capsuleStyle = useViewportRefresh()
  const [merchants, setMerchants] = useState<ExternalTakeoutMerchantLink[]>([])
  const [status, setStatus] = useState<DirectoryStatus>('loading')
  const [jumpingTarget, setJumpingTarget] = useState<string | null>(null)
  const visibility = useFeatureVisibility()
  usePageShare({ kind: 'services', enabled: visibility.takeout })
  const merchantRequestVersion = useRef(0)
  const jumpingRef = useRef<string | null>(null)

  const load = useCallback(async () => {
    const version = merchantRequestVersion.current + 1
    merchantRequestVersion.current = version
    setStatus('loading')
    try {
      const values = await listExternalTakeoutMerchants()
      if (version !== merchantRequestVersion.current) return
      setMerchants(values)
      setStatus(values.length > 0 ? 'ready' : 'empty')
    } catch (error) {
      console.error('load external takeout directory failed', error)
      if (version !== merchantRequestVersion.current) return
      setMerchants([])
      setStatus('error')
    }
  }, [])

  useDidShow(() => {
    if (visibility.takeout) void load()
  })

  useEffect(() => {
    if (visibility.takeout) void load()
    else { merchantRequestVersion.current += 1; setMerchants([]) }
  }, [visibility.takeout, load])

  useEffect(() => () => {
    merchantRequestVersion.current += 1
  }, [])

  const openMerchant = async (merchant: ExternalTakeoutMerchantLink) => {
    await openMiniProgram(
      `merchant:${merchant.id}`,
      merchant.appId,
      merchant.path,
      '商家小程序暂时无法打开',
    )
  }

  async function openMiniProgram(target: string, appId: string, path: string, errorTitle: string) {
    if (jumpingRef.current) return
    jumpingRef.current = target
    setJumpingTarget(target)
    try {
      await Taro.navigateToMiniProgram({ appId, path })
    } catch (error) {
      if (isMiniProgramJumpCancelled(error)) return
      console.error('open campus service mini program failed', error)
      await Taro.showToast({ title: errorTitle, icon: 'none' })
    } finally {
      jumpingRef.current = null
      setJumpingTarget(null)
    }
  }

  return (
    <View className='student-takeout-page' style={capsuleStyle}>
      <View className='student-takeout-header'>
        <CapsulePageHeader
          variant='standard'
          className='student-takeout-header__row'
          left={<Text className='student-takeout-header__title'>校园服务</Text>}
          divider={false}
        />
        {visibility.search && visibility.takeout && <View
          className='student-takeout-search'
          hoverClass='student-takeout-search--pressed'
          onClick={() => void Taro.navigateTo({ url: '/pages/search/index?scope=takeout' })}
        >
          <AppIcon name='search' className='student-takeout-search__icon' />
          <Text>搜索合作商家</Text>
        </View>}
      </View>

      <View className='student-takeout-scroll'>
        {visibility?.takeout && <View className='student-takeout-directory'>
          <View className='student-takeout-directory__copy'>
            <Text className='student-takeout-directory__title'>合作商家</Text>
            <Text className='student-takeout-directory__desc'>商品、订单与售后由商家小程序提供</Text>
          </View>
        </View>}

        {visibility?.takeout && <View className='student-takeout-tip'>
          <AppIcon name='info' className='student-takeout-tip__icon' />
          <Text className='student-takeout-tip__text'>点击后将前往商家小程序，下单与售后由商家提供。</Text>
        </View>}

        {visibility?.takeout && status === 'loading' && (
          <AppSkeleton rows={3} />
        )}
        {visibility?.takeout && status === 'empty' && (
          <AppState kind='empty' title='暂无可用合作商家' description='新的合作商家加入后，会显示在这里。' />
        )}
        {visibility?.takeout && status === 'error' && (
          <AppState kind='error' title='商家加载失败' description='请检查网络连接后重试。' actionText='重新加载' onAction={() => void load()} />
        )}
        {visibility?.takeout && status === 'ready' && merchants.length > 0 && (
          <View className='student-takeout-list'>
            {merchants.map((merchant) => {
              const jumping = jumpingTarget === `merchant:${merchant.id}`
              return (
                <View
                  key={merchant.id}
                  className='student-takeout-merchant'
                  hoverClass='student-takeout-merchant--pressed'
                  onClick={() => void openMerchant(merchant)}
                >
                  <View className='student-takeout-merchant__avatar' style={{ color: merchant.color }}>
                    <Text>{merchant.iconText}</Text>
                  </View>
                  <View className='student-takeout-merchant__main'>
                    <View className='student-takeout-merchant__headline'>
                      <Text className='student-takeout-merchant__name'>{merchant.title}</Text>
                    </View>
                    <Text className='student-takeout-merchant__action'>
                      {jumping ? '打开中' : '进入商家'}
                    </Text>
                  </View>
                  <AppIcon name='chevron-right' className='student-takeout-merchant__arrow app-icon--chevron' />
                </View>
              )
            })}
          </View>
        )}

        {visibility?.takeout && status === 'ready' && <Text className='student-takeout-footer'>已显示全部合作商家</Text>}
      </View>

      <StudentBottomNav active='takeout' />
    </View>
  )
}

export default withFeaturePage(StudentTakeoutPage)
