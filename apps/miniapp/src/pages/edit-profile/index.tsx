import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Button, Image, Input, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import ProfileCoverEditor from '../../components/profile-cover-editor'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { currentUser, updateProfile } from '../../services/a-user'
import { chooseAndUploadImages, confirmDirectUpload, uploadPickedImage, type UploadedMedia } from '../../services/media'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { userInitial } from '../../utils/user-initial'
import './index.scss'

function EditProfileContent() {
  const profile = useSessionStore((state) => state.profile)
  const token = useSessionStore((state) => state.token)
  const updateSessionProfile = useSessionStore((state) => state.updateProfile)
  const [nickname, setNickname] = useState(profile.nickname)
  const [avatar, setAvatar] = useState<UploadedMedia | null>(null)
  const [progress, setProgress] = useState('')
  const [choosingAvatar, setChoosingAvatar] = useState(false)
  const [saving, setSaving] = useState(false)
  const [coverBusy, setCoverBusy] = useState(false)
  const coverBusyRef = useRef(false)
  const profileBusyRef = useRef(false)
  const mountedRef = useRef(true)
  useEffect(() => { mountedRef.current = true; return () => { mountedRef.current = false } }, [])
  const isCurrentSession = () => mountedRef.current && useSessionStore.getState().token === token && useSessionStore.getState().profile.id === profile.id
  const onCoverBusyChange = useCallback((busy: boolean) => { coverBusyRef.current = busy; setCoverBusy(busy) }, [])

  const chooseAvatar = async () => {
    if (profileBusyRef.current || coverBusyRef.current) return
    profileBusyRef.current = true
    setChoosingAvatar(true)
    try {
      const [uploaded] = await chooseAndUploadImages('avatar', 1, (value) => { if (isCurrentSession()) setProgress(value.message) }, isCurrentSession)
      if (!uploaded || !isCurrentSession()) return
      const usable = await confirmDirectUpload(uploaded, isCurrentSession)
      if (!isCurrentSession()) return
      setAvatar({ ...uploaded, ...usable })
      setProgress('头像已上传，保存后生效')
    } catch (error) {
      if (!isCurrentSession()) return
      setAvatar(null)
      setProgress('')
      void Taro.showToast({ title: error instanceof Error ? error.message : '头像上传失败', icon: 'none' })
    } finally {
      profileBusyRef.current = false
      if (isCurrentSession()) setChoosingAvatar(false)
    }
  }

  const save = async () => {
    const name = nickname.trim()
    if (!name) {
      void Taro.showToast({ title: '昵称不能为空', icon: 'none' })
      return
    }
    if (profileBusyRef.current || coverBusyRef.current) return
    profileBusyRef.current = true
    setSaving(true)
    try {
      // Keep historical school/campus fields in the object sent to the API for
      // compatibility, but do not expose or rewrite them in this editor.
      const value = await updateProfile({ ...profile, nickname: name }, avatar?.assetId)
      if (!isCurrentSession()) return
      const refreshed = await currentUser().catch(() => value)
      if (!isCurrentSession()) return
      updateSessionProfile(refreshed)
      await Taro.showToast({ title: '资料已保存', icon: 'success' })
      await Taro.navigateBack()
    } catch (error) {
      if (isCurrentSession()) void Taro.showToast({ title: error instanceof Error ? error.message : '保存失败', icon: 'none' })
    } finally {
      profileBusyRef.current = false
      if (isCurrentSession()) setSaving(false)
    }
  }

  const onChooseWechatAvatar = async (event: { detail?: { avatarUrl?: string } }) => {
    const tempFilePath = event?.detail?.avatarUrl
    if (!tempFilePath || profileBusyRef.current || coverBusyRef.current) return
    profileBusyRef.current = true
    setChoosingAvatar(true)
    try {
      const uploaded = await uploadPickedImage(
        tempFilePath,
        'avatar',
        (_percent, _stage, message) => { if (isCurrentSession()) setProgress(message) },
        isCurrentSession
      )
      if (!isCurrentSession()) return
      const usable = await confirmDirectUpload(uploaded, isCurrentSession)
      if (!isCurrentSession()) return
      setAvatar({ ...uploaded, ...usable })
      setProgress('微信头像已就绪，保存后生效')
    } catch (error) {
      if (!isCurrentSession()) return
      setAvatar(null)
      setProgress('')
      void Taro.showToast({ title: error instanceof Error ? error.message : '微信头像上传失败', icon: 'none' })
    } finally {
      profileBusyRef.current = false
      if (isCurrentSession()) setChoosingAvatar(false)
    }
  }

  const avatarSource = avatar?.previewPath || profile.avatar

  return (
    <View className='edit-profile-page'>
      <CapsulePageHeader
        title='编辑资料'
        divider={false}
        left={(
          <View className='edit-profile-nav__action' onClick={() => { if (!profileBusyRef.current && !coverBusyRef.current) void Taro.navigateBack() }}>
            <Text>取消</Text>
          </View>
        )}
        right={(
          <View className={`edit-profile-nav__save ${saving || choosingAvatar || coverBusy ? 'edit-profile-nav__save--disabled' : ''}`} onClick={() => void save()}>
            <Text>{saving ? '保存中' : '保存'}</Text>
          </View>
        )}
      />

      <View className='edit-profile-content'>
        <View className='edit-profile-avatar-panel'>
          <View className='edit-profile-avatar-row'>
            <View className='edit-profile-avatar-copy'>
              <Text className='edit-profile-field-title'>头像</Text>
              <Text className={`edit-profile-avatar-panel__hint${avatar ? ' edit-profile-avatar-panel__hint--ready' : ''}`}>
                {progress || '点击头像从相册更换，保存后生效'}
              </Text>
              <Button
                className='edit-profile-avatar-wechat'
                openType='chooseAvatar'
                onChooseAvatar={onChooseWechatAvatar}
              >
                用微信头像
              </Button>
            </View>
            <View className={`edit-profile-avatar${choosingAvatar ? ' edit-profile-avatar--busy' : ''}`} onClick={() => void chooseAvatar()} ariaRole='button' ariaLabel='更换头像'>
              {avatarSource ? <Image className='edit-profile-avatar__image' src={avatarSource} mode='aspectFill' /> : <Text className='edit-profile-avatar__text'>{nameInitial(nickname)}</Text>}
              <View className='edit-profile-avatar__camera' aria-hidden='true'><View className='edit-profile-avatar__camera-body' /></View>
            </View>
          </View>
        </View>

        <View className='edit-profile-card'>
          <View className='edit-profile-row edit-profile-row--last'>
            <Text className='edit-profile-row__label'>昵称</Text>
            <Input className='edit-profile-row__input edit-profile-row__input--strong' type='nickname' value={nickname} maxlength={32} placeholder='请输入昵称（可用微信昵称）' onInput={(event) => setNickname(String(event.detail.value ?? ''))} />
          </View>
        </View>
        <Text className='edit-profile-save-hint'>头像和昵称修改后，点击右上角“保存”。</Text>
        <ProfileCoverEditor disabled={saving || choosingAvatar} isProfileBusy={() => profileBusyRef.current} onBusyChange={onCoverBusyChange} />

        <View className='edit-profile-home-card edit-profile-entry-card'>
          <View className='edit-profile-home-row' onClick={() => void Taro.navigateTo({ url: '/pages/appearance/index' })}>
            <View className='edit-profile-home-row__main'>
              <Text className='edit-profile-home-row__title'>首页背景</Text>
              <Text className='edit-profile-home-row__subtitle'>照片背景开关、选择与裁剪、效果预览</Text>
            </View>
            <View className='edit-profile-home-row__right'>
              <AppIcon name='chevron-right' className='edit-profile-home-row__arrow app-icon--chevron' />
            </View>
          </View>
          <View className='edit-profile-home-row edit-profile-home-row--last' onClick={() => void Taro.navigateTo({ url: '/pages/student-verify/index' })}>
            <View className='edit-profile-home-row__main'>
              <Text className='edit-profile-home-row__title'>学生验证</Text>
              <Text className='edit-profile-home-row__subtitle'>学生证照片与身份信息</Text>
            </View>
            <View className='edit-profile-home-row__right'>
              <AppIcon name='chevron-right' className='edit-profile-home-row__arrow app-icon--chevron' />
            </View>
          </View>
        </View>
      </View>
    </View>
  )
}

export default function EditProfilePage() {
  const accountScope = useSessionStore((state) => `${state.profile.id}:${state.token}`)
  return <RuntimeSessionGate><EditProfileContent key={accountScope} /></RuntimeSessionGate>
}

function nameInitial(value: string): string { return userInitial(value, '湛') }
