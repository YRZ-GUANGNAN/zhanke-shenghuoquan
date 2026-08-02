import { Text, View } from '@tarojs/components'
import type { IdentityRoleKey } from '../types'
import AppIcon from './app-icon'
import { AppDialog } from './ui'
import './IdentitySwitchModal.scss'

interface IdentitySwitchModalProps {
  visible: boolean
  currentRole: IdentityRoleKey
  targetRole: IdentityRoleKey
  onCancel: () => void
  onConfirm: () => void
  confirming?: boolean
  error?: string
}

const roleNames: Record<IdentityRoleKey, string> = {
  student: '学生',
  rider: '骑手',
  merchant: '商家'
}

const roleDescriptions: Record<IdentityRoleKey, string> = {
  student: '返回校园生活主界面，已通过的工作身份仍会保留。',
  rider: '进入骑手接单工作台，之后可随时切回学生模式。',
  merchant: '进入商家配送工作台，之后可随时切回学生模式。'
}

export default function IdentitySwitchModal({
  visible,
  currentRole,
  targetRole,
  onCancel,
  onConfirm,
  confirming = false,
  error = ''
}: IdentitySwitchModalProps) {
  const title = targetRole === 'student'
    ? '切回学生模式'
    : `切换到${roleNames[targetRole]}模式`

  const confirmText = targetRole === 'student'
    ? '确认切回'
    : '确认切换'

  return (
    <AppDialog
      visible={visible}
      title={title}
      description={roleDescriptions[targetRole]}
      confirmText={confirmText}
      loading={confirming}
      error={error}
      onCancel={onCancel}
      onConfirm={onConfirm}
    >
      <View className='identity-switch-modal__roles'>
        <View className='identity-switch-modal__role identity-switch-modal__role--muted'>
          <View className='identity-switch-modal__role-icon'>
            <AppIcon name={currentRole} />
          </View>
          <Text className='identity-switch-modal__role-name'>{roleNames[currentRole]}</Text>
          <Text className='identity-switch-modal__role-state'>当前</Text>
        </View>
        <View className='identity-switch-modal__arrow'>
          <AppIcon name='chevron-right' />
        </View>
        <View className='identity-switch-modal__role identity-switch-modal__role--active'>
          <View className='identity-switch-modal__role-icon'>
            <AppIcon name={targetRole} active />
          </View>
          <Text className='identity-switch-modal__role-name'>{roleNames[targetRole]}</Text>
          <Text className='identity-switch-modal__role-state'>即将使用</Text>
        </View>
      </View>
    </AppDialog>
  )
}
