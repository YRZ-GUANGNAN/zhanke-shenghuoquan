import { useEffect, useRef, useState } from 'react'
import { getCurrentInstance, hideShareMenu, showShareMenu, useDidShow, useShareAppMessage } from '@tarojs/taro'
import { buildShareCard, type ShareTarget } from '../services/share-model'
import { getShareSettings, shareSettingsSnapshot, subscribeShareSettings } from '../services/share-settings'
import { featureVisibilitySnapshot } from '../services/feature-visibility'

function allowed(target: ShareTarget): boolean {
  const flags = featureVisibilitySnapshot()
  if (!flags.available) return false
  if (target.kind === 'post' || target.kind === 'profile') return flags.forum
  if (target.kind === 'product' || target.kind === 'secondhand') return flags.secondhand
  if (target.kind === 'services') return flags.takeout || flags.campus_tools
  return true
}

export function usePageShare(target: ShareTarget) {
  const [settings, setSettings] = useState(shareSettingsSnapshot)
  const current = useRef(target)
  current.current = target
  const route = useRef(getCurrentInstance().router?.$taroPath)
  const isCurrentPage = () => Boolean(route.current) && getCurrentInstance().router?.$taroPath === route.current
  const syncMenu = () => {
    if (!isCurrentPage()) return
    const value = current.current
    const canShare = allowed(value) && Boolean(buildShareCard(value, shareSettingsSnapshot()))
    if (canShare) {
      void showShareMenu({ menus: ['shareAppMessage'] }).catch(() => undefined)
      void hideShareMenu({ menus: ['shareTimeline'] }).catch(() => undefined)
    } else {
      void hideShareMenu({ menus: ['shareAppMessage', 'shareTimeline'] }).catch(() => undefined)
    }
  }
  useEffect(() => {
    const unsubscribe = subscribeShareSettings(setSettings)
    void getShareSettings().then(setSettings)
    return unsubscribe
  }, [])
  const card = buildShareCard(target, settings)
  useEffect(syncMenu, [target.enabled, target.kind, card?.path, card?.imageUrl, card?.title])
  useDidShow(() => { syncMenu(); void getShareSettings(true).then(setSettings) })
  useShareAppMessage(() => {
    const value = current.current
    const config = shareSettingsSnapshot()
    return buildShareCard({ ...value, enabled: value.enabled && allowed(value) }, config) ||
      buildShareCard({ kind: 'home', enabled: true }, config)!
  })
  return Boolean(card) && allowed(target)
}
