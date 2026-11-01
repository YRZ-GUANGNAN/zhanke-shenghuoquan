import { useEffect, useRef, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'

/** A history fetch never marks read. The receipt follows a committed, visible render. */
export function useVisibleConversationRead(key: string, renderedRevision: unknown, markRead: (isVisible: () => boolean) => Promise<void>, enabled: boolean) {
  const visible = useRef(true)
  const currentKey = useRef(key)
  currentKey.current = key
  const action = useRef(markRead)
  action.current = markRead
  const [foreground, setForeground] = useState(0)
  useDidHide(() => { visible.current = false })
  useDidShow(() => { visible.current = true; setForeground(value => value + 1) })
  useEffect(() => {
    let active = true
    const isVisible = () => active && visible.current && currentKey.current === key
    if (enabled && key && isVisible()) void action.current(isVisible).catch(() => undefined)
    return () => { active = false }
  }, [key, renderedRevision, foreground, enabled])
}
