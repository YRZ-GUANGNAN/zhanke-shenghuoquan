import React, { useState } from 'react'
import { Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type TabKey = 'favorites' | 'posts' | 'comments' | 'items'
type ToneKey = 'peach' | 'mint' | 'lavender' | 'rose' | 'blue'

interface TabOption {
  key: TabKey
  label: string
}

interface FavoriteItem {
  id: string
  title: string
  author: string
  time: string
  likes: string
  tone: ToneKey
}

interface PostItem {
  id: string
  title: string
  meta: string
  tone: ToneKey
}

interface CommentItem {
  id: string
  content: string
  source: string
  time: string
}

interface MarketItem {
  id: string
  title: string
  price: string
  status: '出售中' | '已售'
  tone: ToneKey
}

const tabs: TabOption[] = [
  { key: 'favorites', label: '收藏' },
  { key: 'posts', label: '帖子' },
  { key: 'comments', label: '评论' },
  { key: 'items', label: '宝贝' }
]

const favoriteItems: FavoriteItem[] = [
  {
    id: 'fav-1',
    title: '今天的晚霞，操场随手拍的，太治愈了',
    author: '小鹿乱撞',
    time: '今天',
    likes: '156',
    tone: 'peach'
  },
  {
    id: 'fav-2',
    title: '期末周图书馆占座攻略｜亲测有效的三个时间点',
    author: '卷心菜',
    time: '昨天',
    likes: '214',
    tone: 'lavender'
  },
  {
    id: 'fav-3',
    title: '宿舍改造第二弹｜300 块把上床下桌改成 ins 风',
    author: '柚子茶',
    time: '6月30日',
    likes: '342',
    tone: 'rose'
  }
]

const postItems: PostItem[] = [
  {
    id: 'post-1',
    title: '求助：明天上午有人能帮拿北门快递吗，有偿',
    meta: '今天 11:02 · 12 赞 · 3 评论',
    tone: 'peach'
  },
  {
    id: 'post-2',
    title: '周末羽毛球局召集，体育馆 3 号场还差两人',
    meta: '6月29日 · 31 赞 · 9 评论',
    tone: 'mint'
  }
]

const commentItems: CommentItem[] = [
  {
    id: 'comment-1',
    content: '学生证减 5 毛是真的吗？？明天就去',
    source: '评论了「三食堂新出的芋泥波波真的绝」',
    time: '40 分钟前'
  },
  {
    id: 'comment-2',
    content: '蹲一个后排，考完一起吃饭',
    source: '评论了「期末周图书馆占座攻略」',
    time: '昨天'
  },
  {
    id: 'comment-3',
    content: '+1 求拼，我有学生证优惠券',
    source: '评论了「有没有周末去金沙湾的搭子」',
    time: '6月30日'
  }
]

const marketItems: MarketItem[] = [
  {
    id: 'item-1',
    title: '九成新自行车，毕业急出，车况好无锈',
    price: '¥150',
    status: '出售中',
    tone: 'blue'
  },
  {
    id: 'item-2',
    title: '四级真题一套+听力网课，半价出',
    price: '¥25',
    status: '已售',
    tone: 'mint'
  }
]

function getInitialTab(): TabKey {
  const tab = Taro.getCurrentInstance().router?.params?.tab
  if (tab === 'posts' || tab === 'comments' || tab === 'items' || tab === 'favorites') {
    return tab
  }
  return 'favorites'
}

function previewToast() {
  void Taro.showToast({
    title: '静态预览',
    icon: 'none',
    duration: 1200
  })
}

function Placeholder({ tone, label }: { tone: ToneKey; label?: string }) {
  return (
    <View className={`my-content-thumb my-content-thumb--${tone}`}>
      <Text>图</Text>
      {label ? <Text> · {label}</Text> : null}
    </View>
  )
}

function renderEndText(label: string) {
  return <View className="my-content-end">— {label} —</View>
}

export default function MyContentPage() {
  const [activeTab, setActiveTab] = useState<TabKey>(getInitialTab)

  const renderFavorites = () => (
    <View className="my-content-card my-content-card--list">
      {favoriteItems.map((item) => (
        <View key={item.id} className="favorite-row" onClick={previewToast}>
          <Placeholder tone={item.tone} />
          <View className="favorite-row__main">
            <Text className="favorite-row__title">{item.title}</Text>
            <Text className="favorite-row__meta">
              {item.author} · {item.time}
            </Text>
          </View>
          <View className="favorite-row__like">
            <Text className="favorite-row__heart">♥</Text>
            <Text>{item.likes}</Text>
          </View>
        </View>
      ))}
      {renderEndText('仅展示最近收藏')}
    </View>
  )

  const renderPosts = () => (
    <View className="my-content-card my-content-card--list">
      {postItems.map((item) => (
        <View key={item.id} className="post-row" onClick={previewToast}>
          <Placeholder tone={item.tone} />
          <View className="post-row__main">
            <Text className="post-row__title">{item.title}</Text>
            <Text className="post-row__meta">{item.meta}</Text>
          </View>
          <Text className="post-row__arrow">›</Text>
        </View>
      ))}
      {renderEndText('到底了')}
    </View>
  )

  const renderComments = () => (
    <View className="my-content-card my-content-card--comments">
      {commentItems.map((item) => (
        <View key={item.id} className="comment-row" onClick={previewToast}>
          <Text className="comment-row__content">{item.content}</Text>
          <View className="comment-row__source">
            <Text className="comment-row__icon">▱</Text>
            <Text className="comment-row__source-text">{item.source}</Text>
            <Text className="comment-row__time">{item.time}</Text>
          </View>
        </View>
      ))}
      {renderEndText('仅展示最近评论')}
    </View>
  )

  const renderItems = () => (
    <View className="my-content-card my-content-card--list">
      {marketItems.map((item) => (
        <View key={item.id} className="item-row" onClick={previewToast}>
          <Placeholder tone={item.tone} />
          <View className="item-row__main">
            <Text className="item-row__title">{item.title}</Text>
            <Text className={`item-row__price ${item.status === '已售' ? 'item-row__price--muted' : ''}`}>
              {item.price}
            </Text>
          </View>
          <Text className={`item-row__status ${item.status === '已售' ? 'item-row__status--sold' : ''}`}>
            {item.status}
          </Text>
        </View>
      ))}
      {renderEndText('到底了')}
    </View>
  )

  const renderContent = () => {
    if (activeTab === 'posts') return renderPosts()
    if (activeTab === 'comments') return renderComments()
    if (activeTab === 'items') return renderItems()
    return renderFavorites()
  }

  return (
    <View className="my-content-page">
      <View className="my-content-header">
        <View className="my-content-header__back" onClick={() => void Taro.navigateBack()}>
          <Text>‹</Text>
        </View>
        <Text className="my-content-header__title">我的内容</Text>
      </View>

      <View className="my-content-tabs">
        {tabs.map((tab) => (
          <View
            key={tab.key}
            className={`my-content-tab ${activeTab === tab.key ? 'my-content-tab--active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            <Text>{tab.label}</Text>
          </View>
        ))}
      </View>

      <View className="my-content-body">{renderContent()}</View>
    </View>
  )
}
