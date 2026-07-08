import React, { useState } from 'react'
import { Input, Text, Textarea, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type Gender = '男' | '女' | '保密'

const genders: Gender[] = ['男', '女', '保密']

function showStaticToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

export default function EditProfilePage() {
  const [nickname, setNickname] = useState('阿泽')
  const [gender, setGender] = useState<Gender>('保密')
  const [signature, setSignature] = useState('热爱生活，随手记录湛科日常')

  const handleSave = () => {
    void Taro.showToast({ title: '保存成功', icon: 'success', duration: 900 })
    setTimeout(() => {
      void Taro.navigateBack()
    }, 900)
  }

  return (
    <View className='edit-profile-page'>
      <View className='edit-profile-nav'>
        <Text className='edit-profile-nav__action' onClick={() => void Taro.navigateBack()}>取消</Text>
        <Text className='edit-profile-nav__title'>编辑资料</Text>
        <Text className='edit-profile-nav__save' onClick={handleSave}>保存</Text>
      </View>

      <View className='edit-profile-content'>
        <View className='edit-profile-avatar-panel'>
          <View className='edit-profile-avatar' onClick={() => showStaticToast('头像更换留到下一阶段')}>
            <Text className='edit-profile-avatar__text'>泽</Text>
            <View className='edit-profile-avatar__camera'>
              <View className='edit-profile-avatar__camera-body' />
            </View>
          </View>
          <Text className='edit-profile-avatar-panel__hint'>点击更换头像</Text>
        </View>

        <View className='edit-profile-card'>
          <View className='edit-profile-row'>
            <Text className='edit-profile-row__label'>昵称</Text>
            <Input
              className='edit-profile-row__input edit-profile-row__input--strong'
              value={nickname}
              maxlength={12}
              placeholder='请输入昵称'
              onInput={(event) => {
                setNickname(String(event.detail.value ?? ''))
                return event.detail.value
              }}
            />
          </View>

          <View className='edit-profile-row'>
            <Text className='edit-profile-row__label'>湛科号</Text>
            <Text className='edit-profile-row__value edit-profile-row__value--muted'>zk_aze233</Text>
          </View>

          <View className='edit-profile-row'>
            <Text className='edit-profile-row__label'>性别</Text>
            <View className='edit-profile-genders'>
              {genders.map((item) => (
                <View
                  key={item}
                  className={`edit-profile-gender ${gender === item ? 'edit-profile-gender--active' : ''}`}
                  onClick={() => setGender(item)}
                >
                  <Text>{item}</Text>
                </View>
              ))}
            </View>
          </View>

          <View className='edit-profile-row' onClick={() => showStaticToast('学院选择静态占位')}>
            <Text className='edit-profile-row__label'>学院</Text>
            <Text className='edit-profile-row__value'>计算机学院</Text>
          </View>

          <View className='edit-profile-row edit-profile-row--last' onClick={() => showStaticToast('年级选择静态占位')}>
            <Text className='edit-profile-row__label'>年级</Text>
            <Text className='edit-profile-row__value'>大三</Text>
          </View>
        </View>

        <View className='edit-profile-signature-card'>
          <View className='edit-profile-signature-card__header'>
            <Text className='edit-profile-signature-card__label'>个性签名</Text>
            <Text className='edit-profile-signature-card__count'>{signature.length}/30</Text>
          </View>
          <Textarea
            className='edit-profile-signature-card__textarea'
            value={signature}
            maxlength={30}
            autoHeight
            placeholder='写一句介绍自己吧'
            onInput={(event) => {
              setSignature(String(event.detail.value ?? '').slice(0, 30))
              return event.detail.value
            }}
          />
        </View>
      </View>
    </View>
  )
}
