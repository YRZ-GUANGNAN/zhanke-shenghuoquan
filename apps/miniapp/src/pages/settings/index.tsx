import React, { useEffect, useRef, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppDialog, AppIconButton } from '../../components/ui'
import {
  bindWechatPhone,
  closeAccount,
  getUserSettings,
  updateUserSettings,
  type UserSettingsView
} from '../../services/a-user'
import { isApiError } from '../../services/api'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import {
  loadNotificationSubscriptionTemplates,
  prepareNotificationSubscription,
  requestNotificationSubscription,
  type NotificationSubscriptionTemplate,
  type PreparedNotificationSubscription
} from '../../services/notification'
import './index.scss'

type ToggleKey = 'orderNotice' | 'socialNotice' | 'quietHours' | 'personalized'
type CloseStage = 'idle' | 'first' | 'final' | 'blocked'

const defaults: UserSettingsView = {
	wechatBound: true,
  orderNotice: true,
  socialNotice: true,
  quietHours: false,
  personalized: true,
  phoneBound: false,
  maskedPhone: '',
  blacklistCount: 0
}

const noticeRows: Array<{ key: ToggleKey; title: string; subtitle: string }> = [
  { key: 'orderNotice', title: '订单通知', subtitle: '接单、取货、送达等状态提醒' },
  { key: 'socialNotice', title: '互动通知', subtitle: '点赞、评论、私聊消息提醒' },
  { key: 'quietHours', title: '夜间免打扰', subtitle: '23:00–07:00 不推送普通通知' }
]

const privacyRows: Array<{ key: ToggleKey; title: string; subtitle: string }> = [
  { key: 'personalized', title: '个性化推荐', subtitle: '根据站内浏览和搜索行为优化内容排序' }
]

function currentVersion(): string {
  try {
    const info = Taro.getAccountInfoSync().miniProgram
    if (info.version) return info.version
    if (info.envVersion === 'release') return '正式版'
    if (info.envVersion === 'trial') return '体验版'
    if (info.envVersion === 'develop') return '开发预览'
    return '版本待确认'
  } catch {
    return '版本待确认'
  }
}

export default function SettingsPage() {
  const logout = useSessionStore((state) => state.logout)
  const token = useSessionStore((state) => state.token)
  const identity = useSessionStore((state) => state.identity)
  const visibility = useFeatureVisibility()
  const isCampusInfo = visibility.presentationMode === 'campus_info'
  const hasHistoricalWork = Boolean(identity?.identities.some(item => item.identity !== 'student' && item.available))
  const visibleNoticeRows = isCampusInfo ? noticeRows.filter(row => row.key === 'orderNotice' ? hasHistoricalWork : row.key === 'socialNotice' ? visibility.notifications || visibility.messages : hasHistoricalWork || visibility.messages) : noticeRows
  const visiblePrivacyRows = isCampusInfo ? privacyRows.filter(() => visibility.forum || visibility.secondhand) : privacyRows
  const [settings, setSettings] = useState<UserSettingsView>(defaults)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [closing, setClosing] = useState(false)
  const [closeStage, setCloseStage] = useState<CloseStage>('idle')
  const [closeError, setCloseError] = useState('')
  const [preparedNotifications, setPreparedNotifications] = useState<Record<string, PreparedNotificationSubscription | null>>({})
  const [subscriptionStatus, setSubscriptionStatus] = useState<Record<string, string>>({})
  const [subscriptionTemplates, setSubscriptionTemplates] = useState<Record<string, NotificationSubscriptionTemplate>>({})
  const subscriptionBusy = useRef(new Set<string>())
  const subscriptionEpoch = useRef(0)
  const subscriptionRefreshSequence = useRef(0)
  useEffect(() => {
    subscriptionEpoch.current += 1
    subscriptionRefreshSequence.current += 1
    subscriptionBusy.current.clear()
    setPreparedNotifications({}); setSubscriptionStatus({}); setSubscriptionTemplates({})
    return () => { subscriptionEpoch.current += 1 }
  }, [token, identity?.currentIdentity])

  const refreshSubscriptionState = async (clearStatusKey = '') => {
    const epoch = subscriptionEpoch.current
    const sequence = ++subscriptionRefreshSequence.current
    const current = () => epoch === subscriptionEpoch.current
      && sequence === subscriptionRefreshSequence.current
      && useSessionStore.getState().token === token
    try {
      const result = await loadNotificationSubscriptionTemplates()
      if (!current()) return
      const next: Record<string, NotificationSubscriptionTemplate> = {}
      result.items.forEach(item => { next[item.templateKey] = item })
      setSubscriptionTemplates(next)
      if (clearStatusKey) setSubscriptionStatus(previous => ({ ...previous, [clearStatusKey]: '' }))
    } catch {
      if (current() && clearStatusKey) setSubscriptionStatus(previous => ({ ...previous, [clearStatusKey]: '通知额度暂时无法确认，请稍后重试' }))
    }
  }

  const prepareSubscription = async (key: string) => {
    if (subscriptionBusy.current.has(key)) return
    subscriptionBusy.current.add(key)
    const epoch = subscriptionEpoch.current
    const current = () => epoch === subscriptionEpoch.current && useSessionStore.getState().token === token
    try {
      const prepared = await prepareNotificationSubscription(key)
      if (!current()) return
      setPreparedNotifications(current => ({ ...current, [key]: prepared }))
      setSubscriptionStatus(current => ({ ...current, [key]: '已准备，请点击授权' }))
    } catch (error) {
      if (current()) setSubscriptionStatus(previous => ({ ...previous, [key]: error instanceof Error ? error.message : '暂不可用' }))
    } finally {
      if (current()) subscriptionBusy.current.delete(key)
    }
  }

  const authorizeSubscription = async (key: string) => {
    const prepared = preparedNotifications[key]
    if (!prepared) return prepareSubscription(key)
    if (subscriptionBusy.current.has(key)) return
    subscriptionBusy.current.add(key)
    const epoch = subscriptionEpoch.current
    const result = await requestNotificationSubscription(prepared)
    if (epoch !== subscriptionEpoch.current || useSessionStore.getState().token !== token) return
    subscriptionBusy.current.delete(key)
    setPreparedNotifications(current => ({ ...current, [key]: null }))
    const accepted = result.status === 'accept' && result.reportStatus === 'recorded'
    setSubscriptionStatus(current => ({ ...current, [key]: accepted ? '授权已记录，正在确认可用额度' : result.message || '未完成授权' }))
    if (accepted) await refreshSubscriptionState(key)
  }

  const load = async () => {
    setLoading(true)
    try {
      setSettings(await getUserSettings())
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '设置加载失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  useDidShow(() => { void load(); void refreshSubscriptionState() })

  const toggle = async (key: ToggleKey) => {
    if (saving || loading) return
    const previous = settings
    const next = { ...settings, [key]: !settings[key] }
    setSettings(next)
    setSaving(true)
    try {
      setSettings(await updateUserSettings(next))
    } catch (error) {
      setSettings(previous)
      void Taro.showToast({ title: error instanceof Error ? error.message : '保存失败', icon: 'none' })
    } finally {
      setSaving(false)
    }
  }

  const bindPhone = async (event: { detail?: { code?: string; errMsg?: string } }) => {
    const code = event.detail?.code || ''
    if (!code) {
      void Taro.showToast({ title: '未获得手机号授权', icon: 'none' })
      return
    }
    try {
      setSettings(await bindWechatPhone(code))
      void Taro.showToast({ title: '手机号已绑定', icon: 'success' })
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '绑定失败', icon: 'none' })
    }
  }

  const checkUpdate = () => {
    try {
      const manager = Taro.getUpdateManager()
      manager.onCheckForUpdate(({ hasUpdate }) => {
        if (!hasUpdate) void Taro.showToast({ title: '当前已是最新版本', icon: 'none' })
      })
      manager.onUpdateReady(() => {
        void Taro.showModal({ title: '更新已准备好', content: '重启小程序以应用新版本', confirmText: '立即重启' })
          .then((result) => { if (result.confirm) manager.applyUpdate() })
      })
      manager.onUpdateFailed(() => { void Taro.showToast({ title: '更新包下载失败，请稍后重试', icon: 'none' }) })
    } catch {
      void Taro.showToast({ title: '当前环境无法检查更新', icon: 'none' })
    }
  }

  const closeCurrentAccount = async () => {
    if (closing) return
    setCloseStage('idle')
    setClosing(true)
    try {
      await closeAccount()
      logout()
      await Taro.reLaunch({ url: '/pages/home/index' })
    } catch (error) {
      let message = error instanceof Error ? error.message : '注销失败'
      if (isApiError(error)) {
        const data = error.data as { blockers?: Array<{ message: string }> } | undefined
        if (data?.blockers?.length) message = data.blockers.map((item) => item.message).join('；')
      }
      setCloseError(message)
      setCloseStage('blocked')
    } finally {
      setClosing(false)
    }
  }

  const identityName = identity?.currentIdentity === 'merchant' ? '商家' : identity?.currentIdentity === 'rider' ? '骑手' : '学生'

  return (
    <View className='student-settings-page'>
      <CapsulePageHeader title='设置' left={<AppIconButton name='back' label='返回' className='student-settings-nav__back' onClick={() => void Taro.navigateBack()} />} />

      <View className='student-settings-content'>
        <View className='student-settings-section'>
          <Text className='student-settings-section__label'>账号与绑定</Text>
          <View className='student-settings-card'>
            <View className='student-settings-row student-settings-account-row'>
              <View className='student-settings-row__main'>
                <Text className='student-settings-row__title'>微信登录</Text>
                <Text className='student-settings-row__subtitle'>当前账号已与微信稳定绑定，退出后可重新登录</Text>
              </View>
              <View className='student-settings-bind'>
                <View className='student-settings-bind__dot' />
                <Text>{settings.wechatBound ? '已绑定' : '待确认'}</Text>
              </View>
            </View>
            {(!isCampusInfo || settings.phoneBound || hasHistoricalWork) && <View className='student-settings-row'>
              <Text className='student-settings-row__title'>手机号</Text>
              <Button className='student-settings-phone' openType='getPhoneNumber' onGetPhoneNumber={bindPhone}>
                {settings.phoneBound ? settings.maskedPhone : '绑定手机号'}
              </Button>
            </View>}
            <View className='student-settings-row'>
              <Text className='student-settings-row__title'>当前身份</Text>
              <Text className='student-settings-row__value'>{identityName}</Text>
            </View>
            {!settings.phoneBound && <View className='student-settings-row student-settings-account-row'>
              <View className='student-settings-row__main'>
                <Text className='student-settings-row__title'>手机号登录</Text>
                <Text className='student-settings-row__subtitle'>功能预留中，暂不影响微信登录</Text>
              </View>
              <Text className='student-settings-row__value'>暂未启用</Text>
            </View>}
          </View>
        </View>

        {visibleNoticeRows.length > 0 && <View className='student-settings-section'>
          <Text className='student-settings-section__label'>消息通知</Text>
          <View className='student-settings-card'>
            {visibleNoticeRows.map((row) => (
              <View key={row.key} className='student-settings-row student-settings-row--tall'>
                <View className='student-settings-row__main'>
                  <Text className='student-settings-row__title'>{row.title}</Text>
                  <Text className='student-settings-row__subtitle'>{row.subtitle}</Text>
                </View>
                <View className='student-settings-switch-target' ariaRole='button' ariaLabel={`${row.title}，${settings[row.key] ? '已开启' : '已关闭'}`} onClick={() => void toggle(row.key)}>
                  <View className={`student-settings-switch ${settings[row.key] ? 'student-settings-switch--on' : ''}`}><View className='student-settings-switch__knob' /></View>
                </View>
              </View>
            ))}
            {(['order', 'private_message'] as const).map((key) => {
              const title = key === 'order' ? '订阅订单通知' : '订阅私信提醒'
              const template = subscriptionTemplates[key]
              const quotaText = template?.available === false
                ? '微信通知暂不可用，站内通知不受影响'
                : template?.remainingQuota === null || template?.remainingQuota === undefined
                  ? '通知额度待确认；一次授权只增加一次额度'
                  : template.remainingQuota > 0
                    ? `当前可用 ${template.remainingQuota} 次；实际送达以微信为准`
                    : (template.reservedQuota || 0) > 0
                      ? `有 ${template.reservedQuota} 次通知正在发送；结果以微信为准`
                      : '当前无可用额度，可按需再订阅一次'
              const statusText = subscriptionStatus[key] || quotaText
              const unavailable = template?.available === false
              return <View key={key} className='student-settings-row student-settings-row--tall'>
                <View className='student-settings-row__main'>
                  <Text className='student-settings-row__title'>{title}</Text>
                  <Text className='student-settings-row__subtitle'>{statusText}</Text>
                </View>
                <Button className='student-settings-phone' disabled={unavailable} onClick={() => void authorizeSubscription(key)}>
                  {unavailable ? '暂不可用' : preparedNotifications[key] ? '立即授权' : template?.remainingQuota ? '再订阅一次' : '订阅一次'}
                </Button>
              </View>
            })}
          </View>
        </View>}
        {(visiblePrivacyRows.length > 0 || settings.blacklistCount > 0) && <View className='student-settings-section'>
          <Text className='student-settings-section__label'>隐私</Text>
          <View className='student-settings-card'>
            {visiblePrivacyRows.map((row) => (
              <View key={row.key} className='student-settings-row student-settings-row--tall'>
                <View className='student-settings-row__main'>
                  <Text className='student-settings-row__title'>{row.title}</Text>
                  <Text className='student-settings-row__subtitle'>{row.subtitle}</Text>
                </View>
                <View className='student-settings-switch-target' ariaRole='button' ariaLabel={`${row.title}，${settings[row.key] ? '已开启' : '已关闭'}`} onClick={() => void toggle(row.key)}>
                  <View className={`student-settings-switch ${settings[row.key] ? 'student-settings-switch--on' : ''}`}><View className='student-settings-switch__knob' /></View>
                </View>
              </View>
            ))}
            <View className='student-settings-row' onClick={() => void Taro.navigateTo({ url: '/pages/blacklist/index' })}>
              <Text className='student-settings-row__title'>黑名单管理</Text>
              <View className='student-settings-row__right'>
                <Text className='student-settings-row__value'>{settings.blacklistCount} 人</Text>
                <AppIcon name='chevron-right' className='student-settings-row__arrow app-icon--chevron' />
              </View>
            </View>
          </View>
        </View>}

        <View className='student-settings-section'>
          <Text className='student-settings-section__label'>通用</Text>
          <View className='student-settings-card'>
            <View className='student-settings-row' onClick={checkUpdate}>
              <Text className='student-settings-row__title'>检查更新</Text>
              <Text className='student-settings-row__value'>{currentVersion()}</Text>
            </View>
            <View className='student-settings-row' onClick={() => void Taro.navigateTo({ url: '/pages/feedback/index' })}>
              <Text className='student-settings-row__title'>意见反馈</Text><AppIcon name='chevron-right' className='student-settings-row__arrow app-icon--chevron' />
            </View>
            <View className='student-settings-row' onClick={() => void Taro.navigateTo({ url: '/pages/about/index' })}>
              <Text className='student-settings-row__title'>关于湛科生活圈</Text><AppIcon name='chevron-right' className='student-settings-row__arrow app-icon--chevron' />
            </View>
          </View>
        </View>

        <View className='student-settings-logout' onClick={() => { logout(); void Taro.reLaunch({ url: '/pages/home/index' }) }}><Text>退出登录</Text></View>
        <View className={`student-settings-close ${closing ? 'student-settings-close--disabled' : ''}`} onClick={() => !closing && setCloseStage('first')}>
          <Text>{closing ? '正在注销…' : '注销账号'}</Text>
        </View>
      </View>
      <AppDialog
        visible={closeStage === 'first'}
        title='注销账号'
        description='注销后当前登录立即失效，未售出的商品会下架；历史帖子、评论和已完成交易会匿名保留。'
        confirmText='继续注销'
        tone='danger'
        onCancel={() => setCloseStage('idle')}
        onConfirm={() => setCloseStage('final')}
      />
      <AppDialog
        visible={closeStage === 'final'}
        title='最后确认'
        description='该操作不可撤销。确认永久注销当前账号吗？'
        confirmText='永久注销'
        tone='danger'
        loading={closing}
        onCancel={() => setCloseStage('idle')}
        onConfirm={() => void closeCurrentAccount()}
      />
      <AppDialog
        visible={closeStage === 'blocked'}
        title='暂时无法注销'
        description={closeError}
        confirmText='知道了'
        showCancel={false}
        onCancel={() => undefined}
        onConfirm={() => setCloseStage('idle')}
      />
    </View>
  )
}
