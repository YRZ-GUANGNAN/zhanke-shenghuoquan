export type ProfileTab = 'posts' | 'secondhand'
export type ProfileTone = 'peach' | 'sky' | 'mint' | 'lemon' | 'lilac'
export type CardHeight = 'short' | 'medium' | 'tall'

export interface PublicProfilePost {
  id: string
  title: string
  mediaLabel: string
  likes: number
  tone: ProfileTone
  height: CardHeight
}

export interface PublicProfileProduct {
  id: string
  title: string
  mediaLabel: string
  price: number
  status: 'selling' | 'sold'
  tone: ProfileTone
  height: CardHeight
}

export interface PublicProfileView {
  id: string
  nickname: string
  account: string
  avatarText: string
  school: string
  grade?: string
  bio: string
  followingCount: number
  followerCount: number
  likedCount: number
  isSelf: boolean
  isFollowing: boolean
  posts: PublicProfilePost[]
  products: PublicProfileProduct[]
}

const profiles: Record<string, PublicProfileView> = {
  'current-user': {
    id: 'current-user',
    nickname: '阿泽',
    account: 'zk_aze233',
    avatarText: '泽',
    school: '湛江科技学院',
    grade: '大二',
    bio: '记录校园日常，也分享认真淘来的好东西',
    followingCount: 26,
    followerCount: 118,
    likedCount: 486,
    isSelf: true,
    isFollowing: false,
    posts: [
      { id: 'aze-library', title: '期末周图书馆安静角落分享', mediaLabel: '图 · 图书馆', likes: 96, tone: 'sky', height: 'medium' },
      { id: 'aze-sunset', title: '下课路上遇到的蓝调晚霞', mediaLabel: '图 · 晚霞', likes: 73, tone: 'peach', height: 'short' },
      { id: 'aze-food', title: '北门新开的砂锅粉，学生价很实在', mediaLabel: '图 · 美食', likes: 52, tone: 'mint', height: 'medium' }
    ],
    products: [
      { id: 'aze-calculator', title: '科学计算器，考试课可用', mediaLabel: '图 · 计算器', price: 28, status: 'selling', tone: 'lemon', height: 'short' },
      { id: 'aze-books', title: '大二公共课教材和复习资料', mediaLabel: '图 · 教材', price: 35, status: 'selling', tone: 'lilac', height: 'medium' }
    ]
  },
  'milk-tea': {
    id: 'milk-tea',
    nickname: '奶茶脑袋',
    account: 'zk_naicha01',
    avatarText: '奶',
    school: '湛江科技学院',
    grade: '大三',
    bio: '万物皆可奶茶 🧋 分享校园好喝榜',
    followingCount: 88,
    followerCount: 326,
    likedCount: 1204,
    isSelf: false,
    isFollowing: false,
    posts: [
      { id: 'milk-tea-main', title: '三食堂新出的芋泥波波真的绝，一天一杯根本停不下来', mediaLabel: '图 · 奶茶', likes: 128, tone: 'peach', height: 'tall' },
      { id: 'milk-tea-review', title: '大学城奶茶测评｜十家喝下来这三家最能打', mediaLabel: '图 · 测评', likes: 256, tone: 'lemon', height: 'medium' },
      { id: 'milk-tea-menu', title: '隐藏喝法：这家的四季春加波波椰果绝配', mediaLabel: '图 · 隐藏菜单', likes: 89, tone: 'sky', height: 'medium' }
    ],
    products: []
  },
  north: {
    id: 'north',
    nickname: '向北',
    account: 'zk_xiangbei',
    avatarText: '向',
    school: '湛江科技学院',
    grade: '大四',
    bio: '毕业清物，校内可当面看货',
    followingCount: 42,
    followerCount: 79,
    likedCount: 216,
    isSelf: false,
    isFollowing: false,
    posts: [
      { id: 'north-graduation', title: '毕业季最后一次绕学校骑行', mediaLabel: '图 · 校园', likes: 64, tone: 'mint', height: 'medium' }
    ],
    products: [
      { id: 'bike', title: '九成新自行车，毕业急出，车况好无锈', mediaLabel: '图 · 自行车', price: 150, status: 'selling', tone: 'sky', height: 'tall' },
      { id: 'north-lamp', title: '宿舍护眼台灯，亮度三档可调', mediaLabel: '图 · 台灯', price: 35, status: 'selling', tone: 'lemon', height: 'short' }
    ]
  }
}

const fallbackProfile: PublicProfileView = {
  id: 'campus-user',
  nickname: '校园同学',
  account: 'campus_user',
  avatarText: '同',
  school: '湛江科技学院',
  bio: '这位同学还没有填写个人简介',
  followingCount: 0,
  followerCount: 0,
  likedCount: 0,
  isSelf: false,
  isFollowing: false,
  posts: [],
  products: []
}

export function resolvePublicProfile(userId?: string): { profile: PublicProfileView; usedFallback: boolean } {
  const key = userId?.trim() || ''
  const profile = profiles[key]
  if (profile) return { profile, usedFallback: false }
  return { profile: { ...fallbackProfile, id: key || fallbackProfile.id }, usedFallback: true }
}
