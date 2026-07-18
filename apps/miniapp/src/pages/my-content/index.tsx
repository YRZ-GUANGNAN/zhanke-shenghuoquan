import React, { useEffect, useState } from 'react'
import { Image, Text, View } from '@tarojs/components'
import type { CommunityPost, Product } from '../../types'
import { getMyComments, getMyFavoritePosts, getMyPosts, type MyCommunityComment } from '../../services/b-community'
import { getMyProductsPage } from '../../services/c-trade'
import Taro from '../../services/taro'
import './index.scss'

type TabKey = 'favorites' | 'posts' | 'comments' | 'items'

const tabs: Array<{ key: TabKey; label: string }> = [
  { key: 'favorites', label: '收藏' },
  { key: 'posts', label: '帖子' },
  { key: 'comments', label: '评论' },
  { key: 'items', label: '宝贝' }
]

function getInitialTab(): TabKey {
  const tab = Taro.getCurrentInstance().router?.params?.tab
  return tab === 'posts' || tab === 'comments' || tab === 'items' ? tab : 'favorites'
}

function reviewLabel(value?: string): string {
  if (value === 'approved') return '已发布'
  if (value === 'rejected' || value === 'taken_down') return '未通过'
  return '审核中'
}

function PostThumb({ value }: { value: CommunityPost }) {
  return value.images[0]
    ? <Image className='my-content-thumb' src={value.images[0]} mode='aspectFill' />
    : <View className='my-content-thumb my-content-thumb--blue'><Text>暂无图片</Text></View>
}

export default function MyContentPage() {
  const [activeTab, setActiveTab] = useState<TabKey>(getInitialTab)
  const [favorites, setFavorites] = useState<CommunityPost[]>([])
  const [posts, setPosts] = useState<CommunityPost[]>([])
  const [comments, setComments] = useState<MyCommunityComment[]>([])
  const [items, setItems] = useState<Product[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = async (tab = activeTab) => {
    setLoading(true)
    setError('')
    try {
      if (tab === 'favorites') setFavorites((await getMyFavoritePosts()).items)
      if (tab === 'posts') setPosts((await getMyPosts()).items)
      if (tab === 'comments') setComments((await getMyComments()).items)
      if (tab === 'items') setItems((await getMyProductsPage()).items)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '内容加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load(activeTab) }, [activeTab])

  const openPost = (postId: string, anchor?: string) => {
    const anchorQuery = anchor ? `&anchor=${encodeURIComponent(anchor)}` : ''
    void Taro.navigateTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(postId)}${anchorQuery}` })
  }
  const openProduct = (productId: string) => void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(productId)}` })

  const state = () => {
    if (loading) return <View className='my-content-card my-content-end'>正在加载…</View>
    if (error) return <View className='my-content-card my-content-end' onClick={() => void load()}>加载失败：{error}，点击重试</View>
    return null
  }

  const renderPosts = (values: CommunityPost[], favorite = false) => (
    <View className='my-content-card my-content-card--list'>
      {values.map((item) => (
        <View key={item.id} className={favorite ? 'favorite-row' : 'post-row'} onClick={() => openPost(item.id)}>
          <PostThumb value={item} />
          <View className={favorite ? 'favorite-row__main' : 'post-row__main'}>
            <Text className={favorite ? 'favorite-row__title' : 'post-row__title'}>{item.title || item.content}</Text>
            <Text className={favorite ? 'favorite-row__meta' : 'post-row__meta'}>
              {favorite ? `${item.author} · ${item.createdAt}` : `${reviewLabel(item.reviewStatus)} · ${item.likedCount} 赞 · ${item.commentCount} 评论`}
            </Text>
          </View>
          {favorite ? <Text className='favorite-row__like'>♥ {item.likedCount}</Text> : <Text className='post-row__arrow'>›</Text>}
        </View>
      ))}
      {values.length === 0 && <View className='my-content-end'>暂无内容</View>}
    </View>
  )

  const renderComments = () => (
    <View className='my-content-card my-content-card--comments'>
      {comments.map((item) => (
        <View key={item.id} className='comment-row' onClick={() => openPost(item.postId, 'comments')}>
          <Text className='comment-row__content'>{item.content}</Text>
          <View className='comment-row__source'>
            <Text className='comment-row__source-text'>评论了《{item.postTitle}》</Text>
            <Text className='comment-row__time'>{item.createdAt} · {reviewLabel(item.reviewStatus)}</Text>
          </View>
        </View>
      ))}
      {comments.length === 0 && <View className='my-content-end'>暂无评论</View>}
    </View>
  )

  const renderItems = () => (
    <View className='my-content-card my-content-card--list'>
      {items.map((item) => (
        <View key={item.id} className='item-row' onClick={() => openProduct(item.id)}>
          {item.images[0] ? <Image className='my-content-thumb' src={item.images[0]} mode='aspectFill' /> : <View className='my-content-thumb my-content-thumb--blue'><Text>暂无图片</Text></View>}
          <View className='item-row__main'><Text className='item-row__title'>{item.title}</Text><Text className='item-row__price'>¥{item.price}</Text></View>
          <Text className={`item-row__status ${item.status === 'sold' ? 'item-row__status--sold' : ''}`}>{item.status === 'sold' ? '已售' : reviewLabel(item.reviewStatus)}</Text>
        </View>
      ))}
      {items.length === 0 && <View className='my-content-end'>暂无二手商品</View>}
    </View>
  )

  const renderContent = () => {
    const status = state()
    if (status) return status
    if (activeTab === 'posts') return renderPosts(posts)
    if (activeTab === 'comments') return renderComments()
    if (activeTab === 'items') return renderItems()
    return renderPosts(favorites, true)
  }

  return (
    <View className='my-content-page'>
      <View className='my-content-header'><View className='my-content-header__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View><Text className='my-content-header__title'>我的内容</Text></View>
      <View className='my-content-tabs'>{tabs.map((tab) => <View key={tab.key} className={`my-content-tab ${activeTab === tab.key ? 'my-content-tab--active' : ''}`} onClick={() => setActiveTab(tab.key)}><Text>{tab.label}</Text></View>)}</View>
      <View className='my-content-body'>{renderContent()}</View>
    </View>
  )
}
