import { withFeaturePage } from '../../components/feature-page-gate'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import StudentBottomNav from '../../components/student-bottom-nav'
import {
  listExternalTakeoutMerchants,
  listNavigationItems,
  type ExternalTakeoutMerchantLink,
} from '../../services/d-navigation'
import Taro from '../../services/taro'
import type { ToolLink } from '../../types'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import './index.scss'

type DirectoryStatus = 'loading' | 'empty' | 'ready' | 'error'
type MerchantFilter = 'all' | 'open' | 'resting'

const filters: Array<{ key: MerchantFilter; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'open', label: '营业中' },
  { key: 'resting', label: '休息中' },
]

function StudentTakeoutPage() {
  const [tools, setTools] = useState<ToolLink[]>([])
  const [toolStatus, setToolStatus] = useState<DirectoryStatus>('loading')
  const [merchants, setMerchants] = useState<ExternalTakeoutMerchantLink[]>([])
  const [status, setStatus] = useState<DirectoryStatus>('loading')
  const [filter, setFilter] = useState<MerchantFilter>('all')
  const [jumpingTarget, setJumpingTarget] = useState<string | null>(null)
  const visibility = useFeatureVisibility()
  const toolRequestVersion = useRef(0)
  const merchantRequestVersion = useRef(0)
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

  const loadTools = useCallback(async () => {
    const version = toolRequestVersion.current + 1
    toolRequestVersion.current = version
    setToolStatus('loading')
    try {
      const values = await listNavigationItems()
      if (version !== toolRequestVersion.current) return
      setTools(values)
      setToolStatus(values.length > 0 ? 'ready' : 'empty')
    } catch (error) {
      console.error('load campus service tools failed', error)
      if (version !== toolRequestVersion.current) return
      setTools([])
      setToolStatus('error')
    }
  }, [])

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
    if (visibility.campus_tools) void loadTools()
    if (visibility.takeout) void load()
  })

  useEffect(() => {
    if (visibility.campus_tools) void loadTools()
    if (visibility.takeout) void load()
  }, [visibility.campus_tools, visibility.takeout, loadTools, load])

  useEffect(() => () => {
    toolRequestVersion.current += 1
    merchantRequestVersion.current += 1
  }, [])

  const openMerchant = async (merchant: ExternalTakeoutMerchantLink) => {
    if (merchant.operatingStatus === 'resting') {
      await Taro.showToast({ title: '商家休息中', icon: 'none' })
      return
    }
    await openMiniProgram(
      `merchant:${merchant.id}`,
      merchant.appId,
      merchant.path,
      '商家小程序暂时无法打开',
    )
  }

  const openTool = async (tool: ToolLink) => {
    if (!tool.appId || !tool.path) return
    await openMiniProgram(`tool:${tool.id}`, tool.appId, tool.path, '服务小程序暂时无法打开')
  }

  async function openMiniProgram(target: string, appId: string, path: string, errorTitle: string) {
    if (jumpingRef.current) return
    jumpingRef.current = target
    setJumpingTarget(target)
    try {
      await Taro.navigateToMiniProgram({ appId, path })
    } catch (error) {
      console.error('open campus service mini program failed', error)
      await Taro.showToast({ title: errorTitle, icon: 'none' })
    } finally {
      jumpingRef.current = null
      setJumpingTarget(null)
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
        {visibility?.campus_tools && toolStatus === 'ready' && (
          <View className='student-takeout-services'>
            <View className='student-takeout-section-heading'>
              <View>
                <Text className='student-takeout-section-heading__title'>常用服务</Text>
                <Text className='student-takeout-section-heading__desc'>直接进入对应服务小程序</Text>
              </View>
            </View>
            <View className='student-takeout-services__grid'>
              {tools.map((tool) => {
                const target = `tool:${tool.id}`
                return (
                  <View
                    key={tool.id}
                    className='student-takeout-service'
                    hoverClass='student-takeout-service--pressed'
                    onClick={() => void openTool(tool)}
                  >
                    <View className='student-takeout-service__icon' style={{ color: tool.color }}>
                      <Text>{tool.iconText}</Text>
                    </View>
                    <Text className='student-takeout-service__name'>
                      {jumpingTarget === target ? '打开中' : tool.title}
                    </Text>
                  </View>
                )
              })}
            </View>
          </View>
        )}

        {visibility?.campus_tools && toolStatus === 'error' && (
          <View className='student-takeout-tools-error' onClick={() => void loadTools()}>
            <Text>常用服务暂时无法加载</Text>
            <Text className='student-takeout-tools-error__action'>重新加载</Text>
          </View>
        )}

        {visibility?.takeout && <View className='student-takeout-directory'>
          <View className='student-takeout-directory__copy'>
            <Text className='student-takeout-directory__title'>合作商家</Text>
            <Text className='student-takeout-directory__desc'>商品、订单与售后由商家小程序提供</Text>
          </View>
          {status === 'ready' && (
            <Text className='student-takeout-directory__status'>{openCount} 家营业中</Text>
          )}
        </View>}

        {visibility?.takeout && status === 'ready' && (
          <View className='student-takeout-filters'>
            {filters.map((item) => (
              <View
                key={item.key}
                className={`student-takeout-filter${filter === item.key ? ' student-takeout-filter--active' : ''}`}
                hoverClass='student-takeout-filter--pressed'
                onClick={() => setFilter(item.key)}
              >
                <Text>{item.label}</Text>
                <Text className='student-takeout-filter__count'>{filterCount(item.key)}</Text>
              </View>
            ))}
          </View>
        )}

        {visibility?.takeout && <View className='student-takeout-tip'>
          <AppIcon name='info' className='student-takeout-tip__icon' />
          <Text className='student-takeout-tip__text'>点击后将前往商家小程序，下单与售后由商家提供。</Text>
        </View>}

        {visibility?.takeout && status === 'loading' && (
          <View className='student-takeout-state'>
            <View className='student-takeout-state__visual student-takeout-state__visual--loading'>
              <View /><View /><View />
            </View>
            <Text className='student-takeout-state__title'>正在读取合作商家</Text>
            <Text className='student-takeout-state__desc'>正在同步最新营业状态</Text>
          </View>
        )}
        {visibility?.takeout && status === 'empty' && (
          <View className='student-takeout-state'>
            <View className='student-takeout-state__visual'>
              <View /><View /><View />
            </View>
            <Text className='student-takeout-state__title'>暂无可用合作商家</Text>
            <Text className='student-takeout-state__desc'>接入并启用后会显示在这里</Text>
          </View>
        )}
        {visibility?.takeout && status === 'error' && (
          <View className='student-takeout-state student-takeout-state--retry' onClick={() => void load()}>
            <View className='student-takeout-state__visual student-takeout-state__visual--error'>
              <View /><View /><View />
            </View>
            <Text className='student-takeout-state__title'>商家加载失败，点击重试</Text>
            <Text className='student-takeout-state__desc'>请检查网络连接后重试</Text>
          </View>
        )}
        {visibility?.takeout && status === 'ready' && filteredMerchants.length === 0 && (
          <View className='student-takeout-state'>
            <View className='student-takeout-state__visual'>
              <View /><View /><View />
            </View>
            <Text className='student-takeout-state__title'>当前分类暂无商家</Text>
            <Text className='student-takeout-state__desc'>可以切换其他营业状态查看</Text>
          </View>
        )}
        {visibility?.takeout && status === 'ready' && filteredMerchants.length > 0 && (
          <View className='student-takeout-list'>
            {filteredMerchants.map((merchant) => {
              const resting = merchant.operatingStatus === 'resting'
              const jumping = jumpingTarget === `merchant:${merchant.id}`
              return (
                <View
                  key={merchant.id}
                  className={`student-takeout-merchant${resting ? ' student-takeout-merchant--resting' : ''}`}
                  hoverClass={resting ? 'none' : 'student-takeout-merchant--pressed'}
                  onClick={() => void openMerchant(merchant)}
                >
                  <View className='student-takeout-merchant__avatar' style={{ color: merchant.color }}>
                    <Text>{merchant.iconText}</Text>
                  </View>
                  <View className='student-takeout-merchant__main'>
                    <View className='student-takeout-merchant__headline'>
                      <Text className='student-takeout-merchant__name'>{merchant.title}</Text>
                      <View className={`student-takeout-status ${resting ? 'student-takeout-status--closed' : 'student-takeout-status--open'}`}>
                        <View className='student-takeout-status__dot' />
                        <Text>{resting ? '休息中' : '营业中'}</Text>
                      </View>
                    </View>
                    <Text className='student-takeout-merchant__desc'>{merchant.subtitle}</Text>
                    <Text className={`student-takeout-merchant__action${resting ? ' student-takeout-merchant__action--disabled' : ''}`}>
                      {jumping ? '打开中' : resting ? '暂未营业' : '进入商家'}
                    </Text>
                  </View>
                  {!resting && (
                    <AppIcon name='chevron-right' className='student-takeout-merchant__arrow app-icon--chevron' />
                  )}
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
