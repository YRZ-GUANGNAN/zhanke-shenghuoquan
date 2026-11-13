import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
import {
  listMyFeedbacks,
  listMyReports,
  type MyFeedbackView,
  type MyReportView
} from '../../services/a-user'
import Taro from '../../services/taro'
import './index.scss'

const FEEDBACK_LABELS: Record<string, string> = {
  open: '待处理',
  in_progress: '处理中',
  resolved: '已解决',
  ignored: '不处理'
}

const REPORT_LABELS: Record<string, string> = {
  submitted: '已提交',
  reviewing: '处理中',
  approved: '已处理',
  rejected: '无需处理'
}

const TARGET_LABELS: Record<string, string> = {
  post: '帖子',
  comment: '评论',
  product: '商品',
  product_comment: '商品留言',
  user: '用户'
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return `${date.getMonth() + 1}月${date.getDate()}日`
}

export default function MyFeedbackPage() {
  const [feedbacks, setFeedbacks] = useState<MyFeedbackView[]>([])
  const [reports, setReports] = useState<MyReportView[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const [feedbackList, reportList] = await Promise.all([listMyFeedbacks(), listMyReports()])
      setFeedbacks(feedbackList)
      setReports(reportList)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '记录加载失败')
    } finally {
      setLoading(false)
    }
  }

  useDidShow(() => { void load() })

  const empty = !loading && !error && feedbacks.length === 0 && reports.length === 0

  return <View className='my-feedback-page'>
    <CapsulePageHeader title='反馈与举报' left={<AppIconButton name='back' label='返回' className='my-feedback-nav__back' onClick={() => void Taro.navigateBack()} />} />

    <View className='my-feedback-content'>
      {loading && <View className='my-feedback-state'><Text className='my-feedback-state__text'>正在读取记录…</Text></View>}
      {!loading && error && <View className='my-feedback-state my-feedback-state--error' onClick={() => void load()}>
        <Text className='my-feedback-state__text'>{error}</Text>
        <Text className='my-feedback-state__action'>点击重试</Text>
      </View>}
      {empty && <View className='my-feedback-state'>
        <Text className='my-feedback-state__text'>还没有提交过反馈或举报</Text>
        <Text className='my-feedback-state__hint'>在「设置 → 意见反馈」中可以向我们反馈问题。</Text>
      </View>}

      {feedbacks.length > 0 && <View className='my-feedback-section'>
        <Text className='my-feedback-section__title'>我的反馈</Text>
        {feedbacks.map(item => <View className='my-feedback-card' key={item.id}>
          <View className='my-feedback-card__head'>
            <Text className='my-feedback-card__time'>{formatTime(item.createdAt)}</Text>
            <Text className={`my-feedback-status my-feedback-status--${item.status}`}>{FEEDBACK_LABELS[item.status] || item.status}</Text>
          </View>
          <Text className='my-feedback-card__body'>{item.content}</Text>
          {item.reply ? <View className='my-feedback-reply'>
            <Text className='my-feedback-reply__label'>处理说明</Text>
            <Text className='my-feedback-reply__text'>{item.reply}</Text>
          </View> : null}
        </View>)}
      </View>}

      {reports.length > 0 && <View className='my-feedback-section'>
        <Text className='my-feedback-section__title'>我的举报</Text>
        {reports.map(item => <View className='my-feedback-card' key={item.id}>
          <View className='my-feedback-card__head'>
            <Text className='my-feedback-card__time'>{formatTime(item.createdAt)}</Text>
            <Text className={`my-feedback-status my-feedback-status--${item.reviewStatus}`}>{REPORT_LABELS[item.reviewStatus] || item.reviewStatus}</Text>
          </View>
          <Text className='my-feedback-card__body'>
            {TARGET_LABELS[item.targetType] || item.targetType} #{item.targetId} · {item.reason}
          </Text>
        </View>)}
      </View>}
    </View>
  </View>
}
