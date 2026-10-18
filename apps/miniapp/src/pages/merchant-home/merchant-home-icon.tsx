import React from 'react'
import { Image } from '@tarojs/components'
import publishIcon from './icons/action-publish.svg'
import walletIcon from './icons/action-wallet.svg'
import billIcon from './icons/action-bill.svg'
import messageIcon from './icons/action-message.svg'

export type MerchantHomeIconName = 'publish' | 'wallet' | 'bill' | 'message'

const sources: Record<MerchantHomeIconName, string> = {
  publish: publishIcon,
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
