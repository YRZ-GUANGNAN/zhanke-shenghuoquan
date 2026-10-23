import { useEffect, useState } from 'react'
import { Button, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton, AppState } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { getCampusGuide, type CampusGuide } from '../../services/campus-info'
import { validGuideId } from '../../services/campus-info-model'
import { usePageShare } from '../../hooks/use-page-share'
import Taro from '../../services/taro'
import { navigateBackOrHome } from '../../utils/navigation'
import './index.scss'

function GuideContent() {
  const id = Taro.getCurrentInstance().router?.params?.id || ''
  const [guide, setGuide] = useState<CampusGuide | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)
  usePageShare({ kind: 'guide', enabled: !loading && !error, id, content: guide })
  const goHome = () => void Taro.reLaunch({ url: '/pages/home/index' })
  const goBack = () => void navigateBackOrHome()

  useEffect(() => {
    let cancelled = false
    setLoading(true); setError(''); setGuide(null)
    void getCampusGuide(id).then(value => { if (!cancelled) setGuide(value) }).catch(reason => {
      if (!cancelled) setError(reason instanceof Error ? reason.message : '指南加载失败')
    }).then(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [id, attempt])

  const copySource = () => {
    if (!guide) return
    void Taro.setClipboardData({ data: guide.sourceUrl }).catch(() => {
      void Taro.showToast({ title: '复制失败，可长按下方链接复制', icon: 'none' })
    })
  }

  return <View className='campus-info-page campus-guide-page'>
    <CapsulePageHeader title='阅读指南' left={<AppIconButton name='back' label='返回' onClick={goBack} />} />
    <ScrollView scrollY className='campus-info-scroll'>
      {loading ? <AppState kind='loading' title='正在读取指南' /> : error ? <AppState kind='error' title='暂时无法打开这篇指南' description={error} actionText={validGuideId(id) ? '重试' : '返回首页'} onAction={() => validGuideId(id) ? setAttempt(value => value + 1) : goHome()} /> : guide && <View className='campus-guide-article'>
        <Text className='campus-guide-category'>{guide.category}</Text>
        <Text className='campus-guide-title'>{guide.title}</Text>
        <Text className='campus-guide-meta'>资料核验于 {guide.verifiedAt.replace(/-/g, '.')} · 独立整理</Text>
        <Text className='campus-guide-summary'>{guide.summary}</Text>
        {guide.sections.map((section, index) => <View className='campus-guide-section' key={section.title}>
          <View className='campus-guide-section__heading'><Text className='campus-guide-section__number'>{String(index + 1).padStart(2, '0')}</Text><Text>{section.title}</Text></View>
          {section.paragraphs.map((paragraph, paragraphIndex) => <Text selectable className='campus-guide-paragraph' key={paragraphIndex}>{paragraph}</Text>)}
        </View>)}
        <View className='campus-guide-source'>
          <Text className='campus-guide-source__label'>资料来源</Text>
          <Text>{guide.sourceName}</Text>
          <Text selectable className='campus-guide-source__url'>{guide.sourceUrl}</Text>
          <Button className='campus-info-text-button' onClick={copySource}>复制官方来源链接</Button>
          <Text>本应用非学校官方应用，提供公开信息的阅读指引，不代办借阅或预约。服务安排以馆方最新通知为准。</Text>
        </View>
        <View className='campus-guide-actions'><Button className='campus-info-text-button' onClick={goHome}>返回首页</Button><Button className='campus-info-text-button' onClick={() => void Taro.navigateTo({ url: '/pages/feedback/index' })}>反馈信息变化</Button></View>
      </View>}
    </ScrollView>
  </View>
}

export default function CampusGuidePage() {
  return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><GuideContent /></RuntimeSessionGate>
}
