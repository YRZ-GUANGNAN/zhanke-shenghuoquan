import React from 'react'
import { Image } from '@tarojs/components'
import refreshIcon from './icons/action-refresh.svg'
import messageIcon from './icons/action-message.svg'
import phoneIcon from './icons/action-phone.svg'
import emptyIcon from './icons/empty-route.svg'

export type RiderWorkbenchIconName = 'refresh' | 'message' | 'phone' | 'empty'

const sources: Record<RiderWorkbenchIconName, string> = {
  refresh: refreshIcon,
  message: messageIcon,
  phone: phoneIcon,
  empty: emptyIcon,
}

export default function RiderWorkbenchIcon({
  name,
  className = '',
}: {
  name: RiderWorkbenchIconName
  className?: string
}) {
  return (
    <Image
      className={`rider-workbench-icon ${className}`.trim()}
      src={sources[name]}
      mode='aspectFit'
    />
  )
}
