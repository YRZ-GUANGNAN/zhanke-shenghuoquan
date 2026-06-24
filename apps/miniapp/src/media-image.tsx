import React, { useEffect, useState } from 'react'
import { Image, Text, View } from '@tarojs/components'
import { resolveDisplayContentURL } from './services/g-takeout-buyer'

interface CachedURL {
  url: string
  expiresAt: number
}

const cache = new Map<string, CachedURL>()
const refreshSkewMs = 10_000

export function ResolvedMediaImage({
  contentUrl,
  className,
  placeholder = '暂无图片'
}: {
  contentUrl?: string
  className?: string
  placeholder?: string
}) {
  const [src, setSrc] = useState('')
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let active = true
    const key = contentUrl?.trim()
    setFailed(false)
    if (!key) {
      setSrc('')
      return () => {
        active = false
      }
    }
    const cached = cache.get(key)
    if (cached && cached.expiresAt - refreshSkewMs > Date.now()) {
      setSrc(cached.url)
      return () => {
        active = false
      }
    }
    resolveDisplayContentURL(key)
      .then((value) => {
        if (!active) return
        const ttlMs = Math.max(1, value.expiresInSeconds) * 1000
        cache.set(key, { url: value.url, expiresAt: Date.now() + ttlMs })
        setSrc(value.url)
      })
      .catch(() => {
        if (active) {
          setSrc('')
          setFailed(true)
        }
      })
    return () => {
      active = false
    }
  }, [contentUrl])

  if (src && !failed) {
    return <Image className={className} src={src} mode='aspectFill' onError={() => setFailed(true)} />
  }

  return (
    <View className={`cc-placeholder ${className || ''}`}>
      <Text>{failed ? '图片暂不可见' : placeholder}</Text>
    </View>
  )
}

export function clearResolvedMediaCache() {
  cache.clear()
}
