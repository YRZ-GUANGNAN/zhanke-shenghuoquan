import { Text, View } from '@tarojs/components'
import { formatProductPrice } from '../utils/product-presentation'
import './product-price.scss'

interface ProductPriceProps {
  value: number | string | null | undefined
  size?: 'card' | 'compact' | 'detail'
  className?: string
}

export default function ProductPrice({ value, size = 'card', className = '' }: ProductPriceProps) {
  const price = formatProductPrice(value)
  const unavailable = price.major === '—'
  return (
    <View className={`product-price product-price--${size}${price.major.length > 6 ? ' product-price--long' : ''} ${className}`}
      ariaLabel={unavailable ? '价格待确认' : `价格 ${price.display} 元`}>
      {!unavailable && <Text className='product-price__currency'>¥</Text>}
      <Text className='product-price__major'>{price.major}</Text>
      {price.hasMinor && <Text className='product-price__minor'>.{price.minor}</Text>}
    </View>
  )
}
