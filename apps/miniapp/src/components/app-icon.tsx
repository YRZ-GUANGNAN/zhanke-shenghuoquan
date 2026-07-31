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
import actionBack from '../assets/icons/action-back.svg'
import actionClose from '../assets/icons/action-close.svg'
import actionSearch from '../assets/icons/action-search.svg'
import actionChevronRight from '../assets/icons/action-chevron-right.svg'
import actionMore from '../assets/icons/action-more.svg'
import actionRefresh from '../assets/icons/action-refresh.svg'
import actionHeart from '../assets/icons/action-heart.svg'
import actionHeartActive from '../assets/icons/action-heart-active.svg'
import actionStar from '../assets/icons/action-star.svg'
import actionStarActive from '../assets/icons/action-star-active.svg'
import actionComment from '../assets/icons/action-comment.svg'
import actionWallet from '../assets/icons/action-wallet.svg'
import actionOrder from '../assets/icons/action-order.svg'
import './app-icon.scss'

export type AppIconName =
  | 'home'
  | 'takeaway'
  | 'market'
  | 'profile'
  | 'add'
  | 'image'
  | 'student'
  | 'rider'
  | 'merchant'
  | 'back'
  | 'close'
  | 'search'
  | 'chevron-right'
  | 'more'
  | 'refresh'
  | 'heart'
  | 'star'
  | 'comment'
  | 'wallet'
  | 'order'

const sources: Record<AppIconName, { normal: string; active: string }> = {
  home: { normal: navHome, active: navHomeActive },
  takeaway: { normal: navTakeaway, active: navTakeawayActive },
  market: { normal: navMarket, active: navMarketActive },
  profile: { normal: navProfile, active: navProfileActive },
  add: { normal: addWhite, active: addWhite },
  image: { normal: image, active: image },
  student: { normal: roleStudent, active: roleStudentActive },
  rider: { normal: roleRider, active: roleRiderActive },
  merchant: { normal: roleMerchant, active: roleMerchantActive },
  back: { normal: actionBack, active: actionBack },
  close: { normal: actionClose, active: actionClose },
  search: { normal: actionSearch, active: actionSearch },
  'chevron-right': { normal: actionChevronRight, active: actionChevronRight },
  more: { normal: actionMore, active: actionMore },
  refresh: { normal: actionRefresh, active: actionRefresh },
  heart: { normal: actionHeart, active: actionHeartActive },
  star: { normal: actionStar, active: actionStarActive },
  comment: { normal: actionComment, active: actionComment },
  wallet: { normal: actionWallet, active: actionWallet },
  order: { normal: actionOrder, active: actionOrder }
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
