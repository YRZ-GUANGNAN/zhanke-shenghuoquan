import { useMemo, useRef, useState } from 'react'
import { Text, Textarea, View } from '@tarojs/components'
import Taro from '../../services/taro'
import { isApiError } from '../../services/api'
import { createCommunityClientRequestId, createCommunityPost } from '../../services/b-community'
import { chooseAndUploadImages, waitForMediaApproval, type UploadedMedia } from '../../services/media'
import './index.scss'

const TOPICS = ['#奶茶推荐', '#求助', '#搭子', '#二手', '#兼职', '#攻略', '#湛科日常']
const MAX_IMAGES = 9

const toast = (title: string, icon: 'success' | 'error' | 'loading' | 'none' = 'none') => {
  void Taro.showToast({ title, icon, duration: 1800 })
}

export default function PublishPostPage() {
  const [content, setContent] = useState('')
  const [selectedTopics, setSelectedTopics] = useState<string[]>([])
  const [images, setImages] = useState<UploadedMedia[]>([])
  const [uploading, setUploading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [uploadHint, setUploadHint] = useState('0/9')
  const clientRequestIdRef = useRef<string | null>(null)

  const canPublish = useMemo(() => content.trim().length > 0 && !uploading && !submitting, [content, uploading, submitting])

  const resetClientRequestId = () => {
    clientRequestIdRef.current = null
  }

  const toggleTopic = (topic: string) => {
    resetClientRequestId()
    setSelectedTopics((current) => {
      if (current.includes(topic)) return current.filter((item) => item !== topic)
      if (current.length >= 3) {
        toast('最多选 3 个话题')
        return current
      }
      return [...current, topic]
    })
  }

  const handlePickImages = async () => {
    if (uploading || submitting) return
    const remaining = MAX_IMAGES - images.length
    if (remaining <= 0) {
      toast('最多上传 9 张图')
      return
    }
    setUploading(true)
    setUploadHint('上传中')
    try {
      const uploaded = await chooseAndUploadImages('post_image', remaining, (progress) => {
        setUploadHint(`${Math.max(images.length, images.length + progress.index - 1)}/${MAX_IMAGES}`)
      })
      if (uploaded.length > 0) resetClientRequestId()
      setImages((current) => [...current, ...uploaded].slice(0, MAX_IMAGES))
      setUploadHint(`${Math.min(MAX_IMAGES, images.length + uploaded.length)}/${MAX_IMAGES}`)
      if (uploaded.length > 0) toast('图片已上传', 'success')
    } catch (error) {
      console.error('upload post image failed', error)
      setUploadHint(`${images.length}/${MAX_IMAGES}`)
      toast('图片上传失败', 'error')
    } finally {
      setUploading(false)
    }
  }

  const handlePublish = async () => {
    if (!content.trim()) {
      toast('先写点内容吧')
      return
    }
    if (submitting) return
    const clientRequestId = clientRequestIdRef.current ?? createCommunityClientRequestId()
    clientRequestIdRef.current = clientRequestId
    setSubmitting(true)
    try {
      for (const image of images) {
        setUploadHint('审核中')
        await waitForMediaApproval(image.assetId)
      }
      const topic = selectedTopics[0] ?? '#湛科日常'
      const normalizedContent = content.trim()
      const title = normalizedContent.split('\n')[0].slice(0, 32) || '校园动态'
      const post = await createCommunityPost({
        title,
        content: normalizedContent,
        topic,
        imageAssetIds: images.map((image) => image.assetId),
        clientRequestId,
      })
      clientRequestIdRef.current = null
      toast('发布成功', 'success')
      void Taro.redirectTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(post.id)}` })
    } catch (error) {
      console.error('publish community post failed', error)
      if (isApiError(error) && error.status >= 400 && error.status < 500) {
        clientRequestIdRef.current = null
      }
      const title = !isApiError(error)
        ? '发布失败，请稍后再试'
        : error.status === 0
          ? '网络异常，内容已保留'
          : error.status === 409
            ? '发布内容已变化，请重新提交'
            : error.status >= 500
              ? '服务暂不可用，稍后可重试'
              : error.message || '发布内容不符合要求'
      toast(title, 'none')
    } finally {
      setSubmitting(false)
      setUploadHint(`${images.length}/${MAX_IMAGES}`)
    }
  }

  return (
    <View className="publish-post-page">
      <View className="publish-post-nav">
        <Text className="publish-post-nav__side" onClick={() => void Taro.navigateBack()}>
          取消
        </Text>
        <Text className="publish-post-nav__title">发布帖子</Text>
        <Text className={`publish-post-nav__publish${canPublish ? ' publish-post-nav__publish--active' : ''}`} onClick={handlePublish}>
          {submitting ? '发布中' : '发布'}
        </Text>
      </View>

      <View className="publish-post-body">
        <View className="publish-post-upload" onClick={handlePickImages}>
          <Text className="publish-post-upload__icon">{images.length > 0 ? '✓' : '▧'}</Text>
          <Text className="publish-post-upload__count">{uploading ? uploadHint : `${images.length}/${MAX_IMAGES}`}</Text>
        </View>

        <Textarea
          className="publish-post-textarea"
          value={content}
          maxlength={600}
          autoHeight={false}
          placeholder="分享你的校园生活：好吃的、求助、找搭子、攻略..."
          onInput={(event) => {
            resetClientRequestId()
            setContent(event.detail.value)
          }}
        />

        <View className="publish-post-divider" />

        <View className="publish-post-topic-head">
          <Text>添加话题</Text>
          <Text>最多选 3 个</Text>
        </View>
        <View className="publish-post-topics">
          {TOPICS.map((topic) => (
            <Text key={topic} className={`publish-post-topic${selectedTopics.includes(topic) ? ' publish-post-topic--active' : ''}`} onClick={() => toggleTopic(topic)}>
              {topic}
            </Text>
          ))}
        </View>

        <View className="publish-post-note">
          <Text className="publish-post-note__icon">ⓘ</Text>
          <Text>发布内容对全校可见，请遵守社区规范，禁止发布违规信息。</Text>
        </View>
      </View>
    </View>
  )
}
