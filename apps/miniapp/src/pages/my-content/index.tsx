import React, { useEffect, useRef, useState } from 'react'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppIconButton } from '../../components/ui'
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

function productStatusLabel(value: Product): string {
  if (value.status === 'sold') return '已售'
  if (value.status === 'reserved') return '已预订'
  return reviewLabel(value.reviewStatus)
}

function ContentThumb({ src, label = '无图' }: { src?: string; label?: string }) {
  return src
    ? <Image className='my-content-thumb' src={src} mode='aspectFill' />
    : <View className='my-content-thumb my-content-thumb--empty'><Text>{label}</Text></View>
}

function mergeById<T extends { id: string }>(current: T[], incoming: T[]): T[] {
  const byId = new Map(current.map((item) => [item.id, item]))
  incoming.forEach((item) => byId.set(item.id, item))
  return Array.from(byId.values())
}

export default function MyContentPage() {
  const [activeTab, setActiveTab] = useState<TabKey>(getInitialTab)
  const [favorites, setFavorites] = useState<CommunityPost[]>([])
  const [posts, setPosts] = useState<CommunityPost[]>([])
  const [comments, setComments] = useState<MyCommunityComment[]>([])
  const [items, setItems] = useState<Product[]>([])
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [loadMoreError, setLoadMoreError] = useState('')
  const requestVersion = useRef(0)

  const load = async (tab = activeTab, nextPage = 1, append = false) => {
    const version = append ? requestVersion.current : requestVersion.current + 1
    if (!append) requestVersion.current = version
    if (append) setLoadingMore(true)
    else {
      setLoading(true)
      setLoadingMore(false)
    }
    if (append) setLoadMoreError('')
    else setError('')
    try {
      if (tab === 'favorites') {
        const result = await getMyFavoritePosts(nextPage, 20)
        if (requestVersion.current !== version) return
        setFavorites((current) => append ? mergeById(current, result.items) : result.items)
        setTotal(result.total)
        setPage(result.page || nextPage)
      }
      if (tab === 'posts') {
        const result = await getMyPosts(nextPage, 20)
        if (requestVersion.current !== version) return
        setPosts((current) => append ? mergeById(current, result.items) : result.items)
        setTotal(result.total)
        setPage(result.page || nextPage)
      }
      if (tab === 'comments') {
        const result = await getMyComments(nextPage, 20)
        if (requestVersion.current !== version) return
        setComments((current) => append ? mergeById(current, result.items) : result.items)
        setTotal(result.total)
        setPage(result.page || nextPage)
      }
      if (tab === 'items') {
        const result = await getMyProductsPage(nextPage, 20)
        if (requestVersion.current !== version) return
        setItems((current) => append ? mergeById(current, result.items) : result.items)
        setTotal(result.total)
        setPage(result.page || nextPage)
      }
    } catch (cause) {
      if (requestVersion.current !== version) return
      const message = cause instanceof Error ? cause.message : '内容加载失败'
      if (append) setLoadMoreError(`${message}，点击重试`)
      else setError(message)
    } finally {
      if (requestVersion.current === version) {
        if (append) setLoadingMore(false)
        else setLoading(false)
      }
    }
  }

  useEffect(() => {
    setPage(1)
    setTotal(0)
    setLoadMoreError('')
    void load(activeTab, 1, false)
  }, [activeTab])

  const currentLength = activeTab === 'favorites'
    ? favorites.length
    : activeTab === 'posts'
      ? posts.length
      : activeTab === 'comments'
        ? comments.length
        : items.length

  const loadMore = () => {
    if (loading || loadingMore || currentLength >= total) return
    void load(activeTab, page + 1, true)
  }

  const openPost = (postId: string, anchor?: string) => {
    const anchorQuery = anchor ? `&anchor=${encodeURIComponent(anchor)}` : ''
    void Taro.navigateTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(postId)}${anchorQuery}` })
  }
  const openProduct = (productId: string) => void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(productId)}` })

  const state = () => {
    if (loading) {
      return (
        <View className='my-content-state'>
          <Text className='my-content-state__title'>正在加载</Text>
          <Text className='my-content-state__desc'>正在读取你的内容…</Text>
        </View>
      )
    }
    if (error) {
      return (
        <View className='my-content-state my-content-state--retry' onClick={() => void load()}>
          <Text className='my-content-state__title'>内容加载失败</Text>
          <Text className='my-content-state__desc'>{error}</Text>
          <Text className='my-content-state__action'>点击重试</Text>
        </View>
      )
    }
    return null
  }

  const renderPosts = (values: CommunityPost[], favorite = false) => (
    <View className='my-content-list'>
      {values.map((item) => (
        <View key={item.id} className='my-content-row' onClick={() => openPost(item.id)}>
          <ContentThumb src={item.images[0]} />
          <View className='my-content-row__main'>
            <Text className='my-content-row__title'>{item.title || item.content}</Text>
            <Text className='my-content-row__meta'>
              {favorite
                ? `${item.author || '校园用户'} · ${item.createdAt}`
                : `${item.likedCount} 赞 · ${item.commentCount} 评论`}
            </Text>
          </View>
          <View className='my-content-row__tail'>
            <Text className='my-content-row__status'>{favorite ? `赞 ${item.likedCount}` : reviewLabel(item.reviewStatus)}</Text>
            <AppIcon name='chevron-right' className='my-content-row__chevron app-icon--chevron' />
          </View>
        </View>
      ))}
      {values.length === 0 && (
        <View className='my-content-state'>
          <Text className='my-content-state__title'>{favorite ? '还没有收藏' : '还没有发布帖子'}</Text>
          <Text className='my-content-state__desc'>{favorite ? '收藏的帖子会集中显示在这里' : '发布后的真实帖子会显示在这里'}</Text>
        </View>
      )}
    </View>
  )

  const renderComments = () => (
    <View className='my-content-list'>
      {comments.map((item) => (
        <View key={item.id} className='my-content-comment' onClick={() => openPost(item.postId, 'comments')}>
          <Text className='my-content-comment__content'>{item.content}</Text>
          <View className='my-content-comment__source'>
            <Text className='my-content-comment__source-title'>评论了《{item.postTitle}》</Text>
            <Text className='my-content-comment__meta'>{item.createdAt} · {reviewLabel(item.reviewStatus)}</Text>
          </View>
        </View>
      ))}
      {comments.length === 0 && (
        <View className='my-content-state'>
          <Text className='my-content-state__title'>还没有评论</Text>
          <Text className='my-content-state__desc'>你发表的评论会集中显示在这里</Text>
        </View>
      )}
    </View>
  )

  const renderItems = () => (
    <View className='my-content-list'>
      {items.map((item) => (
        <View key={item.id} className='my-content-row my-content-row--product' onClick={() => openProduct(item.id)}>
          <ContentThumb src={item.images[0]} />
          <View className='my-content-row__main'>
            <Text className='my-content-row__title'>{item.title}</Text>
            <Text className='my-content-row__price'>¥{item.price}</Text>
          </View>
          <View className='my-content-row__tail'>
            <Text className={`my-content-row__status${item.status === 'sold' ? ' my-content-row__status--muted' : ''}`}>
              {productStatusLabel(item)}
            </Text>
            <AppIcon name='chevron-right' className='my-content-row__chevron app-icon--chevron' />
          </View>
        </View>
      ))}
      {items.length === 0 && (
        <View className='my-content-state'>
          <Text className='my-content-state__title'>还没有二手宝贝</Text>
          <Text className='my-content-state__desc'>你发布的商品会集中显示在这里</Text>
        </View>
      )}
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
      <CapsulePageHeader title='我的内容' left={<AppIconButton name='back' label='返回' className='my-content-header__back' onClick={() => void Taro.navigateBack()} />} />
      <View className='my-content-tabs'>{tabs.map((tab) => <View key={tab.key} className={`my-content-tab ${activeTab === tab.key ? 'my-content-tab--active' : ''}`} onClick={() => setActiveTab(tab.key)}><Text>{tab.label}</Text></View>)}</View>
      <ScrollView
        className='my-content-body'
        scrollY
        enhanced
        showScrollbar={false}
        lowerThreshold={120}
        onScrollToLower={loadMore}
      >
        {renderContent()}
        {loadMoreError ? (
          <Text className='my-content-load-more my-content-load-more--retry' onClick={loadMore}>{loadMoreError}</Text>
        ) : loadingMore ? (
          <Text className='my-content-load-more'>正在加载更多内容…</Text>
        ) : currentLength > 0 && currentLength >= total ? (
          <Text className='my-content-load-more'>— 已显示全部内容 —</Text>
        ) : null}
      </ScrollView>
    </View>
  )
}
