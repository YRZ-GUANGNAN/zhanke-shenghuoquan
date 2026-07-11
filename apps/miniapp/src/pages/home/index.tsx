import React, { useMemo, useState } from 'react'
import { Text, View } from '@tarojs/components'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import StudentBottomNav from '../../components/student-bottom-nav'
import { useIdentityGuard } from '../../hooks/use-identity-guard'
import Taro from '../../services/taro'
import './index.scss'

type HomeTab = 'messages' | 'discover' | 'tools'

type DiscoverCard = {
  id: string
  target: 'post' | 'secondhand'
  tone: 'peach' | 'sky' | 'mint' | 'lilac' | 'lemon' | 'cyan'
  mediaLabel: string
  title: string
  author: string
  avatar: string
  likes: number
  height: 'short' | 'medium' | 'tall'
}

type Conversation = {
  id: string
  name: string
  avatar: string
  preview: string
  time: string
  unread?: boolean
  scene: string
}

type ToolItem = {
  id: string
  icon: string
  label: string
  sub: string
}

const tabs: Array<{ key: HomeTab; label: string }> = [
  { key: 'messages', label: '私信' },
  { key: 'discover', label: '发现' },
  { key: 'tools', label: '工具' }
]

const discoverCards: DiscoverCard[] = [
  {
    id: 'milk-tea',
    target: 'post',
    tone: 'peach',
    mediaLabel: '图 · 奶茶',
    title: '三食堂新出的芋泥波波真的绝，一天一杯根本...',
    author: '奶茶脑袋',
    avatar: '奶',
    likes: 128,
    height: 'medium'
  },
  {
    id: 'bike',
    target: 'secondhand',
    tone: 'sky',
    mediaLabel: '图 · 自行车',
    title: '出一辆九成新自行车，毕业急出，150 拿走',
    author: '向北',
    avatar: '向',
    likes: 36,
    height: 'tall'
  },
  {
    id: 'sea',
    target: 'post',
    tone: 'mint',
    mediaLabel: '图 · 海边',
    title: '有没有周末去金沙湾的搭子？拍照吹海风',
    author: '小鹿乱撞',
    avatar: '鹿',
    likes: 87,
    height: 'medium'
  },
  {
    id: 'library',
    target: 'post',
    tone: 'lilac',
    mediaLabel: '图 · 图书馆',
    title: '期末周图书馆占座攻略｜亲测有效的三个时间...',
    author: '卷心菜',
    avatar: '卷',
    likes: 214,
    height: 'medium'
  },
  {
    id: 'express',
    target: 'post',
    tone: 'lemon',
    mediaLabel: '图 · 快递',
    title: '求助：明天上午有人能帮拿北门快递吗，有偿',
    author: '阿黎',
    avatar: '黎',
    likes: 12,
    height: 'short'
  },
  {
    id: 'part-time',
    target: 'post',
    tone: 'cyan',
    mediaLabel: '图 · 招聘',
    title: '周末兼职招募，咖啡店需要两名同学帮忙',
    author: '椰椰',
    avatar: '椰',
    likes: 68,
    height: 'medium'
  }
]

const conversations: Conversation[] = [
  {
    id: 'c1',
    name: '小鹿乱撞',
    avatar: '鹿',
    preview: '明天去海边的话我可以带相机～',
    time: '12:06',
    unread: true,
    scene: 'sea'
  },
  {
    id: 'c2',
    name: '卷心菜',
    avatar: '卷',
    preview: '图书馆三楼靠窗还有位置',
    time: '昨天',
    scene: 'library'
  },
  {
    id: 'c3',
    name: '向北',
    avatar: '向',
    preview: '自行车可以今晚看车',
    time: '周二',
    unread: true,
    scene: 'bike'
  }
]

const tools: ToolItem[] = [
  { id: 'schedule', icon: '课', label: '课表', sub: '一键看今日课程' },
  { id: 'bus', icon: '车', label: '校车', sub: '班次与校区路线' },
  { id: 'print', icon: '印', label: '打印', sub: '附近打印点导航' },
  { id: 'map', icon: '图', label: '校园地图', sub: '教学楼与宿舍区' },
  { id: 'lost', icon: '失', label: '失物招领', sub: '找回遗落小物' },
  { id: 'repair', icon: '修', label: '报修', sub: '宿舍维修入口' }
]

function splitCards(cards: DiscoverCard[]) {
  return {
    left: cards.filter((_, index) => index % 2 === 0),
    right: cards.filter((_, index) => index % 2 === 1)
  }
}

function HomeContent() {
  const [activeTab, setActiveTab] = useState<HomeTab>('discover')
  const [liked, setLiked] = useState<Record<string, boolean>>({})
  const columns = useMemo(() => splitCards(discoverCards), [])
  const identityAllowed = useIdentityGuard('student')

  const showTodoToast = (title = '静态预览，后续接入') => {
    void Taro.showToast({ title, icon: 'none', duration: 1400 })
  }

  const openSearch = () => {
    void Taro.navigateTo({ url: '/pages/search/index' })
  }

  const toggleLike = (id: string) => {
    setLiked((current) => ({ ...current, [id]: !current[id] }))
  }

  const openConversation = (item: Conversation) => {
    const query = `scene=${encodeURIComponent(item.scene)}&peerName=${encodeURIComponent(item.name)}&peerAvatar=${encodeURIComponent(item.avatar)}`
    void Taro.navigateTo({ url: `/pages/secondhand-chat/index?${query}` })
  }

  const openCard = (card: DiscoverCard) => {
    const url = card.target === 'secondhand'
      ? `/pages/secondhand-detail/index?id=${encodeURIComponent(card.id)}`
      : `/pages/post-detail/index?id=${encodeURIComponent(card.id)}`
    void Taro.navigateTo({ url })
  }

  const renderDiscoverCard = (card: DiscoverCard) => {
    const isLiked = Boolean(liked[card.id])
    return (
      <View
        key={card.id}
        className='student-post-card'
        onClick={() => openCard(card)}
      >
        <View className={`student-post-card__media student-post-card__media--${card.tone} student-post-card__media--${card.height}`}>
          <Text>{card.mediaLabel}</Text>
        </View>
        <View className='student-post-card__body'>
          <Text className='student-post-card__title'>{card.title}</Text>
          <View className='student-post-card__meta'>
            <View className={`student-post-card__avatar student-post-card__avatar--${card.tone}`}>
              <Text>{card.avatar}</Text>
            </View>
            <Text className='student-post-card__author'>{card.author}</Text>
            <View
              className={`student-post-card__like ${isLiked ? 'student-post-card__like--active' : ''}`}
              onClick={(event) => {
                event.stopPropagation()
                toggleLike(card.id)
              }}
            >
              <Text className='student-post-card__heart'>{isLiked ? '♥' : '♡'}</Text>
              <Text>{card.likes + (isLiked ? 1 : 0)}</Text>
            </View>
          </View>
        </View>
      </View>
    )
  }

  if (!identityAllowed) return <View className='student-home-page' />

  return (
    <View className='page student-home-page'>
      <View className='student-home-header'>
        <View className='student-home-header__tabs'>
          {tabs.map((tab) => (
            <View
              key={tab.key}
              className={`student-home-header__tab ${activeTab === tab.key ? 'student-home-header__tab--active' : ''}`}
              onClick={() => setActiveTab(tab.key)}
            >
              <Text>{tab.label}</Text>
            </View>
          ))}
        </View>
        <View className='student-home-header__search' onClick={openSearch} />
      </View>

      <View className='student-home-content'>
        {activeTab === 'messages' && (
          <View className='student-message-panel'>
            {conversations.map((item) => (
              <View
                key={item.id}
                className='student-message-row'
                onClick={() => openConversation(item)}
              >
                <View className='student-message-row__avatar'>
                  <Text>{item.avatar}</Text>
                  {item.unread && <View className='student-message-row__dot' />}
                </View>
                <View className='student-message-row__main'>
                  <View className='student-message-row__top'>
                    <Text className='student-message-row__name'>{item.name}</Text>
                    <Text className='student-message-row__time'>{item.time}</Text>
                  </View>
                  <Text className={`student-message-row__preview ${item.unread ? 'student-message-row__preview--unread' : ''}`}>
                    {item.preview}
                  </Text>
                </View>
              </View>
            ))}
            <Text className='student-home-footnote'>仅展示最近会话</Text>
          </View>
        )}

        {activeTab === 'discover' && (
          <View className='student-waterfall'>
            <View className='student-waterfall__column'>
              {columns.left.map(renderDiscoverCard)}
            </View>
            <View className='student-waterfall__column'>
              {columns.right.map(renderDiscoverCard)}
            </View>
          </View>
        )}

        {activeTab === 'tools' && (
          <View className='student-tools-grid'>
            {tools.map((item) => (
              <View
                key={item.id}
                className='student-tool-card'
                onClick={() => showTodoToast(`${item.label}静态占位`)}
              >
                <View className='student-tool-card__icon'>
                  <Text>{item.icon}</Text>
                </View>
                <Text className='student-tool-card__label'>{item.label}</Text>
                <Text className='student-tool-card__sub'>{item.sub}</Text>
              </View>
            ))}
          </View>
        )}
      </View>

      <StudentBottomNav active='home' />
    </View>
  )
}

export default function HomePage() {
  return (
    <RuntimeSessionGate>
      <HomeContent />
    </RuntimeSessionGate>
  )
}
