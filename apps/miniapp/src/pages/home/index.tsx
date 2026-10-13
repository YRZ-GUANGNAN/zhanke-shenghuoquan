import { useEffect, useState } from 'react'
import { Button, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppState } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import StudentBottomNav from '../../components/student-bottom-nav'
import { listCampusGuides, type CampusGuide } from '../../services/campus-info'
import { usePageShare } from '../../hooks/use-page-share'
import Taro from '../../services/taro'
import './index.scss'

function HomeContent() {
  const [guides, setGuides] = useState<CampusGuide[]>([])
  const [category, setCategory] = useState('全部')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)
  usePageShare({ kind: 'home', enabled: !loading && !error && guides.length > 0 })

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')
    void listCampusGuides().then(items => {
      if (!cancelled) { setGuides(items); setCategory('全部') }
    }).catch(reason => {
      if (!cancelled) { setGuides([]); setError(reason instanceof Error ? reason.message : '指南加载失败') }
    }).then(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [attempt])

  const categories = ['全部', ...Array.from(new Set(guides.map(guide => guide.category)))]
  const visibleGuides = guides.filter(guide => category === '全部' || guide.category === category)
  const openGuide = (id: string) => {
    void Taro.navigateTo({ url: `/pages/campus-guide/index?id=${encodeURIComponent(id)}` }).catch(() => {
      void Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' })
    })
  }

  return (
    <View className='campus-info-page'>
      <CapsulePageHeader title='校园指南' divider={false} />
      <ScrollView scrollY className='campus-info-scroll' key={category}>
        <View className='campus-info-intro'>
          <Text className='campus-info-eyebrow'>在校园，把日常理清楚</Text>
          <Text className='campus-info-heading'>找得到，也看得明白。</Text>
          <Text className='campus-info-description'>查时间、找资料、确认校区。把实用信息整理在一起，每篇都附上来源。</Text>
        </View>
        {!loading && !error && guides.length > 0 && <View className='campus-info-categories'>
          {categories.map(item => <Button key={item} className={`campus-info-category ${category === item ? 'campus-info-category--active' : ''}`} onClick={() => setCategory(item)}>{item}</Button>)}
        </View>}
        {loading ? <AppState kind='loading' title='正在读取校园指南' description='内容来自已核验的公开资料' /> : error ? <AppState kind='error' title='暂时没有加载成功' description={error} actionText='重新加载' onAction={() => setAttempt(value => value + 1)} /> : visibleGuides.length === 0 ? <AppState kind='empty' title='暂时没有可阅读的指南' description='你可以稍后刷新，或通过意见反馈告诉我们需要的信息。' actionText='刷新内容' onAction={() => setAttempt(value => value + 1)} /> : <View className='campus-info-list'>
          {visibleGuides.map((guide, index) => <Button className='campus-info-item' key={guide.id} onClick={() => openGuide(guide.id)}>
            <View className='campus-info-item__meta'><Text>{guide.category}</Text><Text className='campus-info-item__number'>{String(index + 1).padStart(2, '0')}</Text></View>
            <Text className='campus-info-item__title'>{guide.title}</Text>
            <Text className='campus-info-item__summary'>{guide.summary}</Text>
            <View className='campus-info-item__footer'><Text>来源已核验 · {guide.verifiedAt.replace(/-/g, '.')}</Text><AppIcon name='chevron-right' /></View>
          </Button>)}
        </View>}
        <View className='campus-info-note'>
          <Text>独立整理 · 非学校官方应用</Text>
          <Text>具体服务与临时安排，请以学校及图书馆的最新通知为准。</Text>
          <Button className='campus-info-text-button' onClick={() => void Taro.navigateTo({ url: '/pages/feedback/index' })}>发现信息有变化？告诉我们</Button>
        </View>
      </ScrollView>
      <StudentBottomNav active='home' />
    </View>
  )
}

export default function HomePage() {
  return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><HomeContent /></RuntimeSessionGate>
}
