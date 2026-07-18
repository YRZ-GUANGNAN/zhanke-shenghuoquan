import React from 'react'
import { Image } from '@tarojs/components'
import navHome from '../assets/icons/nav-home.svg'
import navHomeActive from '../assets/icons/nav-home-active.svg'
import navTakeaway from '../assets/icons/nav-takeaway.svg'
import navTakeawayActive from '../assets/icons/nav-takeaway-active.svg'
import navMarket from '../assets/icons/nav-market.svg'
import navMarketActive from '../assets/icons/nav-market-active.svg'
import navProfile from '../assets/icons/nav-profile.svg'
import navProfileActive from '../assets/icons/nav-profile-active.svg'
import addWhite from '../assets/icons/add-white.svg'
import image from '../assets/icons/image.svg'
import roleStudent from '../assets/icons/role-student.svg'
import roleStudentActive from '../assets/icons/role-student-active.svg'
import roleRider from '../assets/icons/role-rider.svg'
import roleRiderActive from '../assets/icons/role-rider-active.svg'
import roleMerchant from '../assets/icons/role-merchant.svg'
import roleMerchantActive from '../assets/icons/role-merchant-active.svg'
import './app-icon.scss'

export type AppIconName = 'home' | 'takeaway' | 'market' | 'profile' | 'add' | 'image' | 'student' | 'rider' | 'merchant'

const sources: Record<AppIconName, { normal: string; active: string }> = {
  home: { normal: navHome, active: navHomeActive },
  takeaway: { normal: navTakeaway, active: navTakeawayActive },
  market: { normal: navMarket, active: navMarketActive },
  profile: { normal: navProfile, active: navProfileActive },
  add: { normal: addWhite, active: addWhite },
  image: { normal: image, active: image },
  student: { normal: roleStudent, active: roleStudentActive },
  rider: { normal: roleRider, active: roleRiderActive },
  merchant: { normal: roleMerchant, active: roleMerchantActive }
}

interface AppIconProps {
  name: AppIconName
  active?: boolean
  className?: string
}

export default function AppIcon({ name, active = false, className = '' }: AppIconProps) {
  const source = sources[name][active ? 'active' : 'normal']
  return <Image className={`app-icon ${className}`.trim()} src={source} mode='aspectFit' />
}
