import { Text, View } from '@tarojs/components'
import type { IdentityApplicationRole, IdentityApplicationView } from '../types'
import AppIcon from './app-icon'
import './identity-application-summary.scss'

export const applicationRoleCopy = {
  merchant: { name: '商家', title: '商家身份申请', description: '通过审核后，可发布配送单、管理订单与联系骑手。' },
  rider: { name: '骑手', title: '骑手身份申请', description: '通过审核后，可进入接单大厅，处理取货与送达任务。' },
} satisfies Record<IdentityApplicationRole, { name: string; title: string; description: string }>

export function applicationStatusCopy(application: IdentityApplicationView | null) {
  switch (application?.status) {
    case 'pending': return { label: '审核中', title: '申请已提交', description: '管理员正在核验你的资料，无需重复提交。', action: '查看申请详情', tone: 'pending' }
    case 'approved': return { label: '已通过', title: '身份申请已通过', description: '返回“我的”即可切换身份，进入对应工作台。', action: '查看申请详情', tone: 'approved' }
    case 'rejected': return { label: '需完善', title: '请完善申请资料', description: '请按照审核意见修改，再次提交申请。', action: '修改申请资料', tone: 'rejected' }
    default: return { label: '未申请', title: '准备好资料，即可申请', description: '填写联系信息并上传证明材料，提交后由管理员人工核验。', action: '开始申请', tone: 'new' }
  }
}

// 服务端时间可能使用 Z 或带偏移的 ISO 格式，均显示为北京时间。
export function applicationTime(value?: string): string {
  if (!value) return '时间暂不可用'
  const time = Date.parse(value)
  if (!Number.isFinite(time)) return '时间暂不可用'
  return new Date(time + 8 * 60 * 60 * 1000).toISOString().slice(0, 16).replace('T', ' ')
}

export function ApplicationActivity() {
  return <View className='application-activity'><View /><View /><View /></View>
}

export function ApplicationSummary({ role, application, compact = false }: {
  role: IdentityApplicationRole
  application: IdentityApplicationView | null
  compact?: boolean
}) {
  const status = applicationStatusCopy(application)
  return <View className={`application-summary application-summary--${status.tone}${compact ? ' application-summary--compact' : ''}`}>
    <View className='application-summary__top'>
      <View className='application-summary__role'><AppIcon name={role} active /></View>
      <View className='application-summary__identity'>
        <Text className='application-summary__eyebrow'>{applicationRoleCopy[role].name}身份</Text>
        <Text className='application-summary__title'>{status.title}</Text>
      </View>
      <Text className={`application-status application-status--${status.tone}`}>{status.label}</Text>
    </View>
    <Text className='application-summary__description'>{status.description}</Text>
    {!application && <Text className='application-summary__benefit'>{applicationRoleCopy[role].description}</Text>}
    {application && <View className='application-summary__reference'>
      <Text className='application-summary__reference-label'>申请编号</Text>
      <Text className='application-summary__reference-id' selectable>{application.applicationId}</Text>
    </View>}
  </View>
}

export function ApplicationTimeline({ application }: { application: IdentityApplicationView }) {
  const resolved = application.status !== 'pending'
  return <View className='application-timeline'>
    <Text className='application-timeline__heading'>本次申请进度</Text>
    <View className='application-timeline__steps'>
      <View className='application-timeline__step'>
        <View className='application-timeline__rail'><View className='application-timeline__dot application-timeline__dot--done' /></View>
        <View className='application-timeline__copy'>
          <Text className='application-timeline__title'>资料已提交</Text>
          <Text className='application-timeline__time'>{applicationTime(application.submittedAt)}</Text>
        </View>
      </View>
      <View className='application-timeline__step application-timeline__step--last'>
        <View className='application-timeline__rail'><View className={`application-timeline__dot application-timeline__dot--${application.status}`} /></View>
        <View className='application-timeline__copy'>
          <Text className='application-timeline__title'>{resolved ? application.status === 'approved' ? '审核通过' : '审核未通过，待完善' : '等待人工审核'}</Text>
          <Text className='application-timeline__time'>{resolved ? applicationTime(application.reviewedAt) : '审核结果将在此更新'}</Text>
        </View>
      </View>
    </View>
  </View>
}
