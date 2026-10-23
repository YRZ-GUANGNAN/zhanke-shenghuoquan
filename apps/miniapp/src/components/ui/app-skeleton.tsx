import { View } from '@tarojs/components'
import './app-skeleton.scss'

export interface AppSkeletonProps {
  variant?: 'list' | 'detail' | 'metrics' | 'feed'
  rows?: number
}

export function AppSkeleton({ variant = 'list', rows = 3 }: AppSkeletonProps) {
  if (variant === 'feed') {
    return <View className='app-skeleton app-skeleton--feed' ariaLabel='正在加载内容'>{Array.from({ length: rows }, (_, item) => <View className='app-skeleton__feed-item' key={item}><View className='app-skeleton__media' /><View className='app-skeleton__line app-skeleton__line--title' /><View className='app-skeleton__line app-skeleton__line--short' /></View>)}</View>
  }
  if (variant === 'metrics') {
    return <View className='app-skeleton app-skeleton--metrics'>{[0, 1, 2].map((item) => <View className='app-skeleton__metric' key={item} />)}</View>
  }
  if (variant === 'detail') {
    return <View className='app-skeleton app-skeleton--detail'><View className='app-skeleton__media' /><View className='app-skeleton__line app-skeleton__line--title' /><View className='app-skeleton__line' /><View className='app-skeleton__line app-skeleton__line--short' /></View>
  }
  return <View className='app-skeleton app-skeleton--list'>{Array.from({ length: rows }, (_, item) => <View className='app-skeleton__row' key={item}><View className='app-skeleton__thumb' /><View className='app-skeleton__copy'><View className='app-skeleton__line app-skeleton__line--title' /><View className='app-skeleton__line' /></View></View>)}</View>
}
