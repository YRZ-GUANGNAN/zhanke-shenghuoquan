import React, { useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { AvatarMark, BottomNav } from '../../components'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import { chooseAndUploadImages } from '../../services/media'
import { updateProfile as saveProfile } from '../../services/a-user'
import './index.scss'

const actions = ['收藏', '帖子', '评论', '宝贝']
const settings = ['内容审核记录', '问题反馈', '用户协议', '清除缓存', '检查新版本']

export default function ProfilePage() {
  const profile = useSessionStore((state) => state.profile)
	const setProfile = useSessionStore((state) => state.updateProfile)
	const [changingAvatar, setChangingAvatar] = useState(false)

	const changeAvatar = async () => {
		if (!profile.id || changingAvatar) return
		setChangingAvatar(true)
		try {
			const [uploaded] = await chooseAndUploadImages('avatar', 1)
			if (!uploaded) return
			const updated = await saveProfile(profile, uploaded.assetId)
			setProfile(updated)
			Taro.showToast({ title: '头像已审核并更新', icon: 'success' })
		} catch (error) {
			Taro.showToast({ title: error instanceof Error ? error.message : '头像更新失败', icon: 'none' })
		} finally {
			setChangingAvatar(false)
		}
	}

  return (
    <View className='page profile-page'>
      <View className='profile-head'>
		<View onClick={changeAvatar}><AvatarMark size='large' label={profile.nickname} src={profile.avatar} /></View>
        <View className='profile-head__info'>
          <View className='between'>
            <Text className='profile-head__name'>{profile.nickname}</Text>
			<Text className='profile-head__edit' onClick={changeAvatar}>{changingAvatar ? '审核中' : '修改头像'}</Text>
          </View>
          <Text className='profile-head__id'>ID: {profile.id}  复制</Text>
		  <Text className='profile-head__id'>{profile.school} · {profile.campus}</Text>
        </View>
      </View>

      <View className='quick-actions'>
        {actions.map((action, index) => (
          <View key={action} className='quick-actions__item card'>
            <View className='quick-actions__icon'>{index + 1}</View>
            <Text>{action}</Text>
          </View>
        ))}
      </View>

      <View className='settings card'>
        <View className='settings__row' onClick={() => Taro.navigateTo({ url: '/pages/conversations/index' })}>
          <Text>交易会话</Text><Text className='settings__arrow'>›</Text>
        </View>
        <View className='settings__row' onClick={() => Taro.navigateTo({ url: '/pages/notifications/index' })}>
          <Text>社区通知</Text><Text className='settings__arrow'>›</Text>
        </View>
        {settings.map((item) => (
          <View key={item} className='settings__row' onClick={() => Taro.showToast({ title: item, icon: 'none' })}>
            <Text>{item}</Text>
            <Text className='settings__arrow'>›</Text>
          </View>
        ))}
      </View>

      <Button className='cc-button cc-button--primary login-button'>微信登录</Button>
      <BottomNav active='profile' />
    </View>
  )
}
