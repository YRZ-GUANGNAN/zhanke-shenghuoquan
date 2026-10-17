import { useEffect } from 'react'
import { hideShareMenu, useDidShow } from '@tarojs/taro'
import type { ShareTarget } from '../services/share-model'
export function usePageShare(_target: ShareTarget) {
  const hide = () => { void hideShareMenu({ menus: ['shareAppMessage','shareTimeline'] }).catch(()=>undefined) }
  useEffect(hide, [])
  useDidShow(hide)
  return false
}
