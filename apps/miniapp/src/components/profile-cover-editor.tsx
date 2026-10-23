import React, { useEffect, useRef, useState } from 'react'
import { Button, Image, Text, View } from '@tarojs/components'
import AppIcon from './app-icon'
import { updateProfileCover } from '../services/a-user'
import { isApiError } from '../services/api'
import { chooseAndUploadImages, confirmDirectUpload, type UploadedMedia } from '../services/media'
import Taro from '../services/taro'
import { useSessionStore } from '../stores/session'
import './profile-cover-editor.scss'

interface Props {
  disabled?: boolean
  isProfileBusy: () => boolean
  onBusyChange: (busy: boolean) => void
}

// Cover changes are saved independently from the surrounding profile form.
export default function ProfileCoverEditor({ disabled = false, isProfileBusy, onBusyChange }: Props) {
  const profile = useSessionStore((state) => state.profile)
  const token = useSessionStore((state) => state.token)
  const [candidate, setCandidate] = useState<UploadedMedia | null>(null)
  const [stage, setStage] = useState<'idle' | 'uploading' | 'saving' | 'clearing'>('idle')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [failedSource, setFailedSource] = useState('')
  const lock = useRef(false)
  const alive = useRef(true)
  useEffect(() => { alive.current = true; return () => { alive.current = false } }, [])
  const isCurrent = () => alive.current && useSessionStore.getState().token === token && useSessionStore.getState().profile.id === profile.id
  const busy = stage !== 'idle' || disabled
  const source = candidate?.previewPath || profile.cover || ''

  const begin = (next: typeof stage) => {
    if (lock.current || disabled || isProfileBusy() || !isCurrent()) return false
    lock.current = true
    setStage(next)
    setError('')
    onBusyChange(true)
    return true
  }
  const finish = () => {
    lock.current = false
    if (isCurrent()) { setStage('idle'); onBusyChange(false) }
  }
  const choose = async () => {
    if (!begin('uploading')) return
    setMessage('请选择一张照片')
    try {
      const [uploaded] = await chooseAndUploadImages('profile_cover', 1, (value) => {
        if (isCurrent()) setMessage(value.message)
      }, isCurrent)
      if (!uploaded || !isCurrent()) return
      const usable = await confirmDirectUpload(uploaded, isCurrent)
      if (!isCurrent()) return
      setCandidate({ ...uploaded, ...usable })
      setFailedSource('')
      setMessage('图片已上传，保存封面后生效')
    } catch (reason) {
      if (!isCurrent()) return
      const text = isApiError(reason) && reason.status === 400 && reason.message === 'invalid media upload request'
        ? '服务器未接受封面上传请求，请确认封面功能已更新后重试。'
        : reason instanceof Error ? reason.message : String((reason as { errMsg?: string })?.errMsg || '封面上传失败，请重试')
      if (!/cancel|取消/i.test(text)) setError(text)
      setMessage('')
    } finally { finish() }
  }
  const save = async (clear = false) => {
    if ((!clear && !candidate) || !begin(clear ? 'clearing' : 'saving')) return
    try {
      if (clear) {
        const result = await Taro.showModal({ title: '恢复默认背景？', content: '只移除个人封面，头像和其他资料保持不变。', confirmText: '恢复默认' })
        if (!result.confirm || !isCurrent()) return
      }
      const value = await updateProfileCover(clear ? '' : candidate!.assetId)
      if (!isCurrent()) return
      // A slow cover response must not overwrite a concurrent avatar/name edit.
      useSessionStore.getState().updateProfile({ cover: value.cover || '' })
      setCandidate(null)
      setFailedSource('')
      setMessage(clear ? '已恢复默认背景' : '封面已保存')
    } catch (reason) {
      if (isCurrent()) setError(reason instanceof Error ? reason.message : '封面保存失败，请重试')
    } finally { finish() }
  }

  return (
    <View className='profile-cover-editor' id='profile-cover-editor'>
      <View className='profile-cover-editor__heading'><Text className='profile-cover-editor__title'>主页背景</Text><Text className='profile-cover-editor__scope'>独立保存</Text></View>
      <View className='profile-cover-editor__preview'>
        {source && source !== failedSource
          ? <Image className='profile-cover-editor__image' src={source} mode='aspectFill' ariaLabel='个人封面预览' onError={() => setFailedSource(source)} />
          : <View className='profile-cover-editor__empty'><AppIcon name='image' /><Text>{source ? '图片暂时无法加载' : '让主页多一点你的样子'}</Text></View>}
      </View>
      <Text className={`profile-cover-editor__hint${error ? ' profile-cover-editor__hint--error' : ''}`}>
        {error || message || '与主页使用相同取景，照片居中裁切，下沿自然融入白底。保存封面后生效。'}
      </Text>
      <View className='profile-cover-editor__actions'>
        <Button className={`profile-cover-editor__choose${busy ? ' profile-cover-editor__action--disabled' : ''}`} ariaLabel='选择个人封面照片' disabled={busy} onClick={() => void choose()}>
          <AppIcon name='image' /><Text>{stage === 'uploading' ? '正在处理…' : source ? '更换照片' : '选择照片'}</Text>
        </Button>
        {candidate && <Button className={`profile-cover-editor__save${busy ? ' profile-cover-editor__action--disabled' : ''}`} ariaLabel='保存个人封面' disabled={busy} onClick={() => void save()}><Text>{stage === 'saving' ? '保存中…' : '保存封面'}</Text></Button>}
        {!candidate && profile.cover && <Button className={`profile-cover-editor__clear${busy ? ' profile-cover-editor__action--disabled' : ''}`} ariaLabel='恢复默认背景' disabled={busy} onClick={() => void save(true)}><Text>{stage === 'clearing' ? '处理中…' : '恢复默认'}</Text></Button>}
        {candidate && <Button className={`profile-cover-editor__discard${busy ? ' profile-cover-editor__action--disabled' : ''}`} ariaLabel='取消封面更换' disabled={busy} onClick={() => { if (!lock.current && !disabled && !isProfileBusy()) { setCandidate(null); setError(''); setMessage('') } }}><Text>取消更换</Text></Button>}
      </View>
    </View>
  )
}
