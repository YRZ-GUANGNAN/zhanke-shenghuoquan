import React from 'react'
import { Image } from '@tarojs/components'
import walletIcon from './icons/action-wallet.svg'
import billIcon from './icons/action-bill.svg'
import messageIcon from './icons/action-message.svg'

export type MerchantHomeIconName = 'wallet' | 'bill' | 'message'

const sources: Record<MerchantHomeIconName, string> = {
  wallet: walletIcon,
  bill: billIcon,
  message: messageIcon,
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
