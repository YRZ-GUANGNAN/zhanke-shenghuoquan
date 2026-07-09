import React from 'react'
import { Text, View } from '@tarojs/components'
import type { IdentityRoleKey } from '../types'
import './IdentitySwitchModal.scss'

export type { IdentityRoleKey } from '../types'

interface IdentitySwitchModalProps {
  visible: boolean
  currentRole: IdentityRoleKey
  targetRole: IdentityRoleKey
  onCancel: () => void
  onConfirm: () => void
}

const roleNames: Record<IdentityRoleKey, string> = {
  student: '学生',
  rider: '骑手',
  merchant: '商家'
}

const roleDescriptions: Record<IdentityRoleKey, string> = {
  student: '主界面将回到校园发现、外卖、二手与我的页面。',
  rider: '主界面将变为抢单大厅，可随时在「我的」切回学生身份。',
  merchant: '主界面将变为商家工作台，可随时在「我的」切回学生身份。'
}

export default function IdentitySwitchModal({
  visible,
  currentRole,
  targetRole,
  onCancel,
  onConfirm
}: IdentitySwitchModalProps) {
  if (!visible) return null

  const title = targetRole === 'student'
    ? '切回学生身份?'
    : `切换为${roleNames[targetRole]}身份?`

  const confirmText = targetRole === 'student'
    ? '确认切回'
    : '确认切换'

  return (
    <View className='identity-switch-modal'>
      <View className='identity-switch-modal__mask' onClick={onCancel} />
      <View className='identity-switch-modal__panel'>
        <Text className='identity-switch-modal__title'>{title}</Text>
        <Text className='identity-switch-modal__desc'>
          {roleDescriptions[targetRole]}
        </Text>
        <View className='identity-switch-modal__roles'>
          <View className='identity-switch-modal__role identity-switch-modal__role--muted'>
            <Text>{roleNames[currentRole]}</Text>
          </View>
          <Text className='identity-switch-modal__arrow'>→</Text>
          <View className='identity-switch-modal__role identity-switch-modal__role--active'>
            <Text>{roleNames[targetRole]}</Text>
          </View>
        </View>
        <View className='identity-switch-modal__actions'>
          <View className='identity-switch-modal__button identity-switch-modal__button--cancel' onClick={onCancel}>
            <Text>取消</Text>
          </View>
          <View className='identity-switch-modal__button identity-switch-modal__button--confirm' onClick={onConfirm}>
            <Text>{confirmText}</Text>
          </View>
        </View>
      </View>
    </View>
  )
}
