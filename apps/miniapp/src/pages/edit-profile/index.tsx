import React, { useState } from 'react'
import { Image, Input, Text, View } from '@tarojs/components'
import { updateProfile } from '../../services/a-user'
import { chooseAndUploadImages, waitForMediaApproval, type UploadedMedia } from '../../services/media'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

export default function EditProfilePage() {
  const profile = useSessionStore((state) => state.profile)
  const updateSessionProfile = useSessionStore((state) => state.updateProfile)
  const [nickname, setNickname] = useState(profile.nickname)
  const [school, setSchool] = useState(profile.school)
  const [campus, setCampus] = useState(profile.campus)
  const [avatar, setAvatar] = useState<UploadedMedia | null>(null)
  const [progress, setProgress] = useState('')
  const [saving, setSaving] = useState(false)

  const chooseAvatar = async () => {
    try {
      const [uploaded] = await chooseAndUploadImages('avatar', 1, (value) => setProgress(value.message))
      await waitForMediaApproval(uploaded.assetId, (value) => setProgress(value.message))
      setAvatar(uploaded)
      setProgress('')
    } catch (error) {
      setProgress('')
      void Taro.showToast({ title: error instanceof Error ? error.message : '头像上传失败', icon: 'none' })
    }
  }

  const save = async () => {
    const name = nickname.trim()
    if (!name) {
      void Taro.showToast({ title: '昵称不能为空', icon: 'none' })
      return
    }
    if (saving) return
    setSaving(true)
    try {
      const value = await updateProfile({ ...profile, nickname: name, school: school.trim(), campus: campus.trim() }, avatar?.assetId)
      updateSessionProfile(value)
      await Taro.showToast({ title: '资料已保存', icon: 'success' })
      await Taro.navigateBack()
    } catch (error) {
      void Taro.showToast({ title: error instanceof Error ? error.message : '保存失败', icon: 'none' })
    } finally {
      setSaving(false)
    }
  }

  const avatarSource = avatar?.previewPath || profile.avatar

  return (
    <View className='edit-profile-page' style={getCapsuleSafeStyle()}>
      <View className='edit-profile-nav'>
        <Text className='edit-profile-nav__action' onClick={() => void Taro.navigateBack()}>取消</Text>
        <Text className='edit-profile-nav__title'>编辑资料</Text>
        <Text className='edit-profile-nav__save' onClick={() => void save()}>{saving ? '保存中' : '保存'}</Text>
      </View>

      <View className='edit-profile-content'>
        <View className='edit-profile-avatar-panel'>
          <View className='edit-profile-avatar' onClick={() => void chooseAvatar()}>
            {avatarSource ? <Image className='edit-profile-avatar__image' src={avatarSource} mode='aspectFill' /> : <Text className='edit-profile-avatar__text'>{nameInitial(nickname)}</Text>}
            <View className='edit-profile-avatar__camera'><View className='edit-profile-avatar__camera-body' /></View>
          </View>
          <Text className='edit-profile-avatar-panel__hint'>{progress || '点击更换头像'}</Text>
        </View>

        <View className='edit-profile-card'>
          <View className='edit-profile-row'>
            <Text className='edit-profile-row__label'>昵称</Text>
            <Input className='edit-profile-row__input edit-profile-row__input--strong' value={nickname} maxlength={32} placeholder='请输入昵称' onInput={(event) => setNickname(String(event.detail.value ?? ''))} />
          </View>
          <View className='edit-profile-row'>
            <Text className='edit-profile-row__label'>学校</Text>
            <Input className='edit-profile-row__input' value={school} maxlength={80} placeholder='请输入学校' onInput={(event) => setSchool(String(event.detail.value ?? ''))} />
          </View>
          <View className='edit-profile-row edit-profile-row--last'>
            <Text className='edit-profile-row__label'>校区 / 学院</Text>
            <Input className='edit-profile-row__input' value={campus} maxlength={80} placeholder='请输入校区或学院' onInput={(event) => setCampus(String(event.detail.value ?? ''))} />
          </View>
        </View>
      </View>
    </View>
  )
}

function nameInitial(value: string): string { return value.trim().slice(0, 1) || '湛' }
