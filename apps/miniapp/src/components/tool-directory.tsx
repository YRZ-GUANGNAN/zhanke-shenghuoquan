import { Text, View } from '@tarojs/components'
import type { ToolLink } from '../types'
import AppIcon from './app-icon'
import { AppSkeleton, AppState } from './ui'
import { StatusGlyph } from './ui/status-glyph'
import './tool-directory.scss'

export interface ToolDirectoryProps {
  tools: ToolLink[]
  status: 'idle' | 'loading' | 'ready' | 'empty' | 'error'
  jumpingId?: string | null
  refreshError?: boolean
  compact?: boolean
  onOpen: (tool: ToolLink) => void
  onRetry: () => void
}
export default function ToolDirectory({ tools, status, jumpingId, refreshError, compact, onOpen, onRetry }: ToolDirectoryProps) {
  const groups = new Map<string, { name: string; items: ToolLink[] }>()
  tools.forEach(tool => {
    const key = tool.categoryId || ''
    if (!groups.has(key)) groups.set(key, { name: tool.categoryName || '常用工具', items: [] })
    groups.get(key)!.items.push(tool)
  })
  return <View className={`tool-directory${compact ? ' tool-directory--compact' : ''}`}>
    <View className='tool-directory__heading'>
      <Text className='tool-directory__eyebrow'>校园服务</Text>
      <Text className='tool-directory__title'>{compact ? '常用服务' : '校园工具'}</Text>
      <Text className='tool-directory__description'>需要的服务，从这里出发</Text>
    </View>
    {status === 'idle' || status === 'loading' ? <AppSkeleton rows={3} /> : status === 'error' ?
      <AppState kind='error' title='工具加载失败' description='请检查网络后重新加载。' actionText='重新加载' onAction={onRetry} /> : status === 'empty' ?
      <AppState kind='empty' title='暂无可用工具' description='新的校园服务接入后，会显示在这里。' actionText='刷新看看' onAction={onRetry} /> :
      <>{Array.from(groups, ([id, group]) => <View className='tool-directory__group' key={id}>
        <View className='tool-directory__group-heading'><Text>{group.name}</Text><Text className='tool-directory__count'>{group.items.length} 项</Text></View>
        {group.items.map(tool => <View key={tool.id} className='tool-entry' hoverClass='tool-entry--pressed'
          ariaRole='button' ariaLabel={`打开${tool.title}`} onClick={() => onOpen(tool)}>
          <View className='tool-entry__mark' style={/^#[0-9a-f]{6}$/i.test(tool.color) ? { color: tool.color } : undefined}>
            {tool.iconText.trim() ? <Text>{[...tool.iconText.trim()].slice(0, 2).join('')}</Text> : <AppIcon name='document' />}
          </View>
          <View className='tool-entry__body'><Text className='tool-entry__title'>{tool.title}</Text>
            <Text className='tool-entry__description'>{tool.subtitle || '前往服务小程序'}</Text></View>
          {jumpingId === tool.id ? <StatusGlyph kind='loading' small /> : <AppIcon name='chevron-right' className='tool-entry__arrow' />}
        </View>)}
      </View>)}
      {refreshError && <View className='tool-directory__retry' ariaRole='button' onClick={onRetry}><Text>更新失败，已保留当前工具 · 重试</Text></View>}
      <Text className='tool-directory__footer'>点击工具后，将打开对应的服务小程序</Text></>}
  </View>
}
