import React, { useMemo, useState } from 'react'
import { Text, Textarea, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

const TOPICS = ['#奶茶推荐', '#求助', '#搭子', '#二手', '#兼职', '#攻略', '#湛科日常']

export default function PublishPostPage() {
  const [content, setContent] = useState('')
  const [selectedTopics, setSelectedTopics] = useState<string[]>([])
  const canPublish = useMemo(() => content.trim().length > 0 || selectedTopics.length > 0, [content, selectedTopics])

  const showPlaceholder = (title: string) => {
    void Taro.showToast({ title, icon: 'none', duration: 1400 })
  }

  const toggleTopic = (topic: string) => {
    if (selectedTopics.includes(topic)) {
      setSelectedTopics((current) => current.filter((item) => item !== topic))
      return
    }
    if (selectedTopics.length >= 3) {
      showPlaceholder('最多选择 3 个话题')
      return
    }
    setSelectedTopics((current) => [...current, topic])
  }

  const handlePublish = () => {
    if (!canPublish) {
      showPlaceholder('先写点内容再发布')
      return
    }
    showPlaceholder('发布功能稍后开放')
  }

  return (
    <View className='publish-post-page'>
      <View className='publish-post-nav'>
        <View className='publish-post-nav__side' onClick={() => Taro.navigateBack()}>
          <Text>取消</Text>
        </View>
        <Text className='publish-post-nav__title'>发布帖子</Text>
        <View
          className={`publish-post-nav__publish ${canPublish ? 'publish-post-nav__publish--active' : ''}`}
          onClick={handlePublish}
        >
          <Text>发布</Text>
        </View>
      </View>

      <View className='publish-post-body'>
        <View className='publish-post-upload' onClick={() => showPlaceholder('图片功能稍后开放')}>
          <Text className='publish-post-upload__icon'>▧</Text>
          <Text className='publish-post-upload__count'>0/9</Text>
        </View>

        <Textarea
          className='publish-post-textarea'
          maxlength={600}
          value={content}
          placeholder='分享你的校园生活:好吃的、求助、找搭子、攻略...'
          placeholderClass='publish-post-textarea__placeholder'
          onInput={(event) => setContent(String(event.detail.value))}
        />

        <View className='publish-post-divider' />

        <View className='publish-post-topic-head'>
          <Text className='publish-post-topic-head__title'>添加话题</Text>
          <Text className='publish-post-topic-head__hint'>最多选 3 个</Text>
        </View>

        <View className='publish-post-topics'>
          {TOPICS.map((topic) => {
            const active = selectedTopics.includes(topic)
            return (
              <View
                key={topic}
                className={`publish-post-topic ${active ? 'publish-post-topic--active' : ''}`}
                onClick={() => toggleTopic(topic)}
              >
                <Text>{topic}</Text>
              </View>
            )
          })}
        </View>

        <View className='publish-post-note'>
          <Text className='publish-post-note__icon'>i</Text>
          <Text>发布内容对全校可见,请遵守社区规范,禁止发布违规信息。</Text>
        </View>
      </View>
    </View>
  )
}
