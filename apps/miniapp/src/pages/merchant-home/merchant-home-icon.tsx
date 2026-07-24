import React from 'react'
import { Image } from '@tarojs/components'
import publishIcon from './icons/action-publish.svg'
import walletIcon from './icons/action-wallet.svg'
import billIcon from './icons/action-bill.svg'
import messageIcon from './icons/action-message.svg'
import emptyIcon from './icons/empty-orders.svg'

export type MerchantHomeIconName = 'publish' | 'wallet' | 'bill' | 'message' | 'empty'

const sources: Record<MerchantHomeIconName, string> = {
  publish: publishIcon,
  wallet: walletIcon,
  bill: billIcon,
  message: messageIcon,
  empty: emptyIcon,
}

export default function MerchantHomeIcon({
  name,
  className = '',
}: {
  name: MerchantHomeIconName
  className?: string
}) {
  return (
    <Image
      className={`merchant-home-icon ${className}`.trim()}
      src={sources[name]}
      mode='aspectFit'
    />
  )
}
