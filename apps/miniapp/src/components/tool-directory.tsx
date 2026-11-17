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
  kind?: 'tool' | 'maker'
  onOpen: (tool: ToolLink) => void
  onRetry: () => void
}
export default function ToolDirectory({ tools, status, jumpingId, refreshError, compact, kind = 'tool', onOpen, onRetry }: ToolDirectoryProps) {
  const groups = new Map<string, { name: string; items: ToolLink[] }>()
  tools.forEach(tool => {
    const key = tool.categoryId || ''
    if (!groups.has(key)) groups.set(key, { name: tool.categoryName || '常用工具', items: [] })
    groups.get(key)!.items.push(tool)
  })
  const copy = kind === 'maker'
    ? { errorTitle: '众创入口加载失败', emptyTitle: '暂无众创入口', emptyDescription: '众创空间与创业服务接入后会显示在这里。', entryFallback: '前往众创小程序' }
    : { errorTitle: '工具加载失败', emptyTitle: '暂无可用工具', emptyDescription: '新的校园服务接入后，会显示在这里。', entryFallback: '前往服务小程序' }
  return <View className={`tool-directory${compact ? ' tool-directory--compact' : ''}`}>
    {status === 'idle' || status === 'loading' ? <AppSkeleton rows={3} /> : status === 'error' ?
      <AppState kind='error' title={copy.errorTitle} description='请检查网络后重新加载。' actionText='重新加载' onAction={onRetry} /> : status === 'empty' ?
      <AppState kind='empty' title={copy.emptyTitle} description={copy.emptyDescription} actionText='刷新看看' onAction={onRetry} /> :
      <>{Array.from(groups, ([id, group]) => <View className='tool-directory__group' key={id}>
        <View className='tool-directory__group-heading'><Text>{group.name}</Text><Text className='tool-directory__count'>{group.items.length} 项</Text></View>
        {group.items.map(tool => <View key={tool.id} className='tool-entry' hoverClass='tool-entry--pressed'
          ariaRole='button' ariaLabel={`打开${tool.title}`} onClick={() => onOpen(tool)}>
          <View className='tool-entry__mark' style={/^#[0-9a-f]{6}$/i.test(tool.color) ? { color: tool.color } : undefined}>
            {tool.iconText.trim() ? <Text>{[...tool.iconText.trim()].slice(0, 2).join('')}</Text> : <AppIcon name='document' />}
          </View>
          <View className='tool-entry__body'><Text className='tool-entry__title'>{tool.title}</Text>
            <Text className='tool-entry__description'>{tool.subtitle || copy.entryFallback}</Text></View>
          {jumpingId === tool.id ? <StatusGlyph kind='loading' small /> : <AppIcon name='chevron-right' className='tool-entry__arrow' />}
        </View>)}
      </View>)}
      {refreshError && <View className='tool-directory__retry' ariaRole='button' onClick={onRetry}><Text>更新失败，已保留当前工具 · 重试</Text></View>}</>}
  </View>
}
