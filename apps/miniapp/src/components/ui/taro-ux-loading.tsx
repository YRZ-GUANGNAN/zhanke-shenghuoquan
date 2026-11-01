import { memo, type CSSProperties } from 'react'
import { View } from '@tarojs/components'
import './taro-ux-loading.scss'

// Directly ported from nanxiaobei/taro-ux-kits/blur-loading/Loading.jsx.
// Preserve the original three-dot primitive; the native refresh wrapper owns
// request state and viewport placement, not a second touch/scroll controller.
const TaroUxLoading = memo(({ className = '', style }: { className?: string; style?: CSSProperties & { '--blur'?: string } }) => (
  <View className={`taro-ux-loading ${className}`.trim()} style={style} aria-hidden='true'>
    <View className='taro-ux-loading__dot' />
    <View className='taro-ux-loading__dot' />
    <View className='taro-ux-loading__dot' />
  </View>
))

export default TaroUxLoading
