import { withFeaturePage } from '../../components/feature-page-gate'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Image, Input, Text, Textarea, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import Taro from '../../services/taro'
import { isApiError } from '../../services/api'
import {
  createCommunityClientRequestId,
  createCommunityPost,
  listHotCommunityTags,
} from '../../services/b-community'
import { chooseAndUploadImages, waitForMediaApprovals, type UploadedMedia } from '../../services/media'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import './index.scss'

const MAX_IMAGES = 9
const MAX_TAGS = 3
const SUBMIT_TIMEOUT_MS = 120000
const APPROVAL_TIMEOUT_MS = 105000
type SubmitStage = 'idle' | 'reviewing' | 'publishing'

const toast = (title: string, icon: 'success' | 'error' | 'loading' | 'none' = 'none') => {
  void Taro.showToast({ title, icon, duration: 1800 })
}

const normalizeTag = (value: string) => value.replace(/^#+/, '').trim().replace(/\s+/g, ' ')

const validateTag = (value: string): string => {
  const tag = normalizeTag(value)
  const length = Array.from(tag).length
  if (!tag) return '请输入标签内容'
  if (length > 12) return '标签最多 12 个字'
  if (/[\u0000-\u001f\u007f-\u009f]/.test(tag)) return '标签包含无效字符'
  return ''
}

function PublishPostPage() {
  const [content, setContent] = useState('')
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  const [hotTags, setHotTags] = useState<string[]>([])
  const [customTag, setCustomTag] = useState('')
  const [images, setImages] = useState<UploadedMedia[]>([])
  const [uploading, setUploading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [submitStage, setSubmitStage] = useState<SubmitStage>('idle')
  const [uploadHint, setUploadHint] = useState('0/9')
  const [editorError, setEditorError] = useState('')
  const visibility = useFeatureVisibility()
  const clientRequestIdRef = useRef<string | null>(null)
  const submitLock = useRef(false)
  const uploadLock = useRef(false)

  const canPublish = useMemo(
    () => content.trim().length > 0 && !uploading && !submitting,
    [content, uploading, submitting],
  )

  useEffect(() => {
    let active = true
    void listHotCommunityTags()
      .then((items) => {
        if (!active) return
        const tags = items.map((item) => normalizeTag(item.tag)).filter(Boolean)
        setHotTags(tags)
      })
      .catch((error) => console.error('load community hot tags failed', error))
    return () => { active = false }
  }, [])

  const resetClientRequestId = () => {
    clientRequestIdRef.current = null
    setEditorError('')
  }

  const toggleTag = (rawTag: string) => {
    if (submitLock.current) return
    const tag = normalizeTag(rawTag)
    if (!tag) return
    resetClientRequestId()
    setSelectedTags((current) => {
      const matched = current.find((item) => item.toLocaleLowerCase() === tag.toLocaleLowerCase())
      if (matched) return current.filter((item) => item !== matched)
      if (current.length >= MAX_TAGS) {
        toast(`最多选择 ${MAX_TAGS} 个标签`)
        return current
      }
      return [...current, tag]
    })
  }

  const addCustomTag = () => {
    if (submitLock.current) return
    const validationError = validateTag(customTag)
    if (validationError) {
      toast(validationError)
      return
    }
    const tag = normalizeTag(customTag)
    const alreadySelected = selectedTags.some((item) => item.toLocaleLowerCase() === tag.toLocaleLowerCase())
    if (!alreadySelected && selectedTags.length >= MAX_TAGS) {
      toast(`最多选择 ${MAX_TAGS} 个标签`)
      return
    }
    resetClientRequestId()
    if (!alreadySelected) setSelectedTags((current) => [...current, tag])
    setCustomTag('')
  }

  const handlePickImages = async () => {
    if (uploadLock.current || submitLock.current) return
    const remaining = MAX_IMAGES - images.length
    if (remaining <= 0) {
      toast(`最多上传 ${MAX_IMAGES} 张图`)
      return
    }
    uploadLock.current = true
    setUploading(true)
    setEditorError('')
    setUploadHint('上传中')
    try {
      const uploaded = await chooseAndUploadImages('post_image', remaining, (progress) => {
        setUploadHint(`${Math.min(MAX_IMAGES, images.length + progress.index + 1)}/${MAX_IMAGES}`)
      })
      if (uploaded.length > 0) resetClientRequestId()
      setImages((current) => [...current, ...uploaded].slice(0, MAX_IMAGES))
      setUploadHint(`${Math.min(MAX_IMAGES, images.length + uploaded.length)}/${MAX_IMAGES}`)
      if (uploaded.length > 0) toast('图片已上传', 'success')
    } catch (error) {
      console.error('upload post image failed', error)
      setUploadHint(`${images.length}/${MAX_IMAGES}`)
      const message = error instanceof Error ? error.message : '图片上传失败'
      setEditorError(message)
      toast(message, 'error')
    } finally {
      uploadLock.current = false
      setUploading(false)
    }
  }

  const handlePublish = async () => {
    if (uploadLock.current || submitLock.current) return
    if (!content.trim()) {
      toast('先写点内容吧')
      return
    }
    submitLock.current = true
    const clientRequestId = clientRequestIdRef.current ?? createCommunityClientRequestId()
    clientRequestIdRef.current = clientRequestId
    setSubmitting(true)
    setEditorError('')
    setSubmitStage('reviewing')
    const deadline = Date.now() + SUBMIT_TIMEOUT_MS
    try {
      setUploadHint('审核中')
      await waitForMediaApprovals(
        images.map((image) => image.assetId),
        () => setUploadHint('审核中'),
        Math.min(APPROVAL_TIMEOUT_MS, deadline - Date.now()),
        3
      )
      if (deadline - Date.now() < 12000) {
        throw new Error('本次提交等待超时，草稿和图片已保留，请稍后重试')
      }
      setSubmitStage('publishing')
      setUploadHint('发布中')
      const normalizedContent = content.trim()
      const title = normalizedContent.split('\n')[0].slice(0, 32) || '校园动态'
      const post = await createCommunityPost({
        title,
        content: normalizedContent,
        topic: selectedTags[0] ?? '湛科日常',
        tags: selectedTags,
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
      const message = !isApiError(error)
        ? error instanceof Error ? error.message : '发布失败，请稍后再试'
        : error.status === 0
          ? '网络异常，内容已保留'
          : error.status === 409
            ? '发布内容已变化，请重新提交'
            : error.status >= 500
              ? '服务暂不可用，稍后可重试'
              : error.message || '发布内容不符合要求'
      toast(message)
      setEditorError(message)
    } finally {
      submitLock.current = false
      setSubmitting(false)
      setSubmitStage('idle')
      setUploadHint(`${images.length}/${MAX_IMAGES}`)
    }
  }

  if (visibility && !visibility.post) {
    return <View className='publish-post-page'><Text>发帖功能暂未开放</Text></View>
  }
  return (
    <View className='publish-post-page' style={getCapsuleSafeStyle()}>
      <CapsulePageHeader
        title='发布帖子'
        left={<Text className={`publish-post-nav__side${submitting ? ' publish-post-control--pending' : ''}`} onClick={() => {
          if (submitLock.current) { toast('内容正在提交，请稍候'); return }
          void Taro.navigateBack()
        }}>取消</Text>}
        right={
          <Text
            className={`publish-post-nav__publish${canPublish ? ' publish-post-nav__publish--active' : ''}`}
            onClick={() => void handlePublish()}
          >
            {submitting ? submitStage === 'reviewing' ? '审核中' : '发布中' : '发布'}
          </Text>
        }
      />

      <View className='publish-post-body'>
        <View className='publish-post-media'>
          {images.map((image) => <View className='publish-post-media__item' key={image.assetId}>
            <Image className='publish-post-media__image' src={image.previewPath} mode='aspectFill' />
          </View>)}
          <View className={`publish-post-upload${uploading || submitting || images.length === MAX_IMAGES ? ' publish-post-control--pending' : ''}`} role='button' aria-label='添加帖子图片' aria-disabled={uploading || submitting || images.length === MAX_IMAGES} onClick={() => void handlePickImages()}>
            <Text className='publish-post-upload__icon'>{images.length === MAX_IMAGES ? '✓' : '＋'}</Text>
            <Text className='publish-post-upload__count'>{uploading ? uploadHint : `${images.length}/${MAX_IMAGES}`}</Text>
          </View>
        </View>

        <Textarea
          className='publish-post-textarea'
          value={content}
          disabled={submitting}
          maxlength={600}
          autoHeight={false}
          placeholder='分享你的校园生活：好吃的、求助、找搭子、攻略……'
          placeholderClass='publish-post-textarea__placeholder'
          onInput={(event) => {
            if (submitLock.current) return
            resetClientRequestId()
            setContent(event.detail.value)
          }}
        />
        <Text className='publish-post-length'>{content.length} / 600</Text>
        {editorError && <View className='publish-post-error' role='alert'><Text>{editorError}</Text></View>}

        <View className='publish-post-divider' />

        <View className='publish-post-topic-head'>
          <Text className='publish-post-topic-head__title'>添加标签</Text>
          <Text className='publish-post-topic-head__hint'>最多选择 {MAX_TAGS} 个</Text>
        </View>
        <View className={`publish-post-topics${submitting ? ' publish-post-control--pending' : ''}`}>
          {hotTags.length > 0
            ? hotTags.map((tag) => (
                <Text
                  key={tag}
                  className={`publish-post-topic${selectedTags.includes(tag) ? ' publish-post-topic--active' : ''}`}
                  onClick={() => toggleTag(tag)}
                >
                  #{tag}
                </Text>
              ))
            : <Text className='publish-post-topics__empty'>暂无热门标签，可在下方自定义</Text>}
        </View>

        <View className='publish-post-custom-tag'>
          <Input
            className='publish-post-custom-tag__input'
            value={customTag}
            disabled={submitting}
            maxlength={13}
            confirmType='done'
            placeholder='自定义标签，最多 12 个字'
            onInput={(event) => { if (!submitLock.current) setCustomTag(event.detail.value) }}
            onConfirm={addCustomTag}
          />
          <Text className={`publish-post-custom-tag__button${submitting ? ' publish-post-control--pending' : ''}`} onClick={addCustomTag}>添加</Text>
        </View>

        {selectedTags.length > 0 && (
          <View className='publish-post-selected'>
            <Text className='publish-post-selected__label'>已选择</Text>
            {selectedTags.map((tag) => (
              <Text key={tag} className='publish-post-selected__item' onClick={() => toggleTag(tag)}>#{tag} ×</Text>
            ))}
          </View>
        )}

        <View className='publish-post-note'>
          <Text className='publish-post-note__icon'>i</Text>
          <Text>发布内容对全校可见，请遵守社区规范，禁止发布违规信息。</Text>
        </View>
      </View>
    </View>
  )
}

export default withFeaturePage(PublishPostPage)
