import { useEffect, useRef, useState } from 'react'
import { nextTick, useDidHide, useDidShow } from '@tarojs/taro'
import { Image, Text, Video, View } from '@tarojs/components'
import Taro from '../services/taro'
import type { PostMotion } from '../types'
import { previewSeconds, type MotionPreviewCoordinator } from '../utils/motion-preview'
import './post-motion-media.scss'
import { getCapsuleLayout } from '../utils/capsule-safe'

type Props = {
  id: string; motion: PostMotion; mode: 'preview' | 'detail'; enabled?: boolean
  activeId?: string; coordinator?: MotionPreviewCoordinator; order?: number
}

export default function PostMotionMedia({ id, motion, mode, enabled = true, activeId, coordinator, order = 0 }: Props) {
  const videoId = `post-video-${mode}-${id.replace(/[^a-z0-9-]/gi, '')}`
  const elementId = `${videoId}-frame`
  const [requested, setRequested] = useState(false)
  const [pageVisible, setPageVisible] = useState(true)
  const [failed, setFailed] = useState(false)
  const [posterFailed, setPosterFailed] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const loadingTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const stopped = useRef(false)
  const preview = mode === 'preview'
  const playing = !failed && enabled && pageVisible && (preview ? activeId === id : requested)
  const playingRef = useRef(playing)
  playingRef.current = playing
  const clearTimer = () => { if (timer.current) clearTimeout(timer.current); timer.current = null }
  const stop = () => {
    clearTimer()
    if (loadingTimer.current) clearTimeout(loadingTimer.current)
    loadingTimer.current = null
    stopped.current = true
    try { Taro.createVideoContext(videoId).pause() } catch { /* Older runtimes keep the static poster. */ }
  }
  const finish = () => { stop(); if (preview) coordinator?.finish(id); else setRequested(false) }
  useDidHide(() => { playingRef.current = false; stop(); setPageVisible(false); setRequested(false) })
  useDidShow(() => setPageVisible(true))
  useEffect(() => {
    setFailed(false); setPosterFailed(false); setRequested(false)
  }, [motion.url, motion.posterUrl])
  useEffect(() => {
    if (!playing) stop()
    else {
      stopped.current = false
      // A stalled player must not occupy the list's sole preview slot forever.
      if (preview) loadingTimer.current = setTimeout(finish, 8000)
    }
    return stop
  }, [playing, videoId])
  useEffect(() => {
    if (!preview || !enabled || !pageVisible || failed || !coordinator) return
    let disposed = false
    let observer: ReturnType<typeof Taro.createIntersectionObserver> | undefined
    nextTick(() => { try {
      if (disposed) return
      observer = Taro.createIntersectionObserver(undefined, { thresholds: [0, .25, .65, 1] })
      observer.relativeToViewport({ top: -getCapsuleLayout().totalHeaderHeight, bottom: -90 }).observe(`#${elementId}`, (entry) => {
        if (disposed) return
        if (entry.intersectionRatio >= .65) coordinator.visible(id, order, stop)
        else if (entry.intersectionRatio < .25) coordinator.hidden(id)
      })
    } catch { /* Poster fallback when native observation is unavailable. */ } })
    return () => { disposed = true; observer?.disconnect(); coordinator.hidden(id); stop() }
  }, [preview, enabled, pageVisible, failed, coordinator, id, elementId, order])
  useEffect(() => () => stop(), [])

  const label = motion.kind === 'live_photo' ? '实况' : '视频'
  return <View id={elementId} className={`post-motion post-motion--${mode}`}>
    {!posterFailed ? <Image className='post-motion__poster' src={motion.posterUrl} mode={preview ? 'aspectFill' : 'aspectFit'} lazyLoad onError={() => setPosterFailed(true)} /> : <Text className='post-motion__fallback'>封面暂时无法显示</Text>}
    {playing && <Video id={videoId} className='post-motion__video' src={motion.url} poster={motion.posterUrl}
      autoplay muted={preview || motion.kind === 'live_photo'} controls={!preview} loop={false}
      showCenterPlayBtn={!preview} showPlayBtn={!preview} showFullscreenBtn={!preview}
      enableProgressGesture={!preview} objectFit={preview ? 'cover' : 'contain'}
      onPlay={() => {
        if (!playingRef.current || stopped.current) { stop(); return }
        if (loadingTimer.current) clearTimeout(loadingTimer.current)
        loadingTimer.current = null
        if (preview && !timer.current) timer.current = setTimeout(finish, previewSeconds(motion.durationMs) * 1000)
      }}
      onTimeUpdate={event => { if (preview && event.detail.currentTime >= previewSeconds(motion.durationMs)) finish() }}
      onEnded={finish}
      onError={() => { finish(); setFailed(true) }}
    />}
    {preview ? <Text className='post-motion__badge'>{label}{motion.kind === 'video' ? ` · ${Math.ceil(motion.durationMs / 1000)}秒` : ''}</Text> : !playing && <View className='post-motion__play' role='button' aria-label={`播放${label}`} onClick={() => { setFailed(false); setRequested(true) }}><Text className='post-motion__play-icon'>▶</Text><Text>{failed ? '重新播放' : `播放${label}`}</Text></View>}
  </View>
}
