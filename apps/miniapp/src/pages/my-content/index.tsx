import ProductPrice from '../../components/product-price'
import { withFeaturePage } from '../../components/feature-page-gate'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import React, { useRef, useState } from 'react'
import { useDidShow } from '@tarojs/taro'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import ChannelPager, { useChannelInteraction } from '../../components/channel-pager'
import AppIcon from '../../components/app-icon'
import { AppIconButton } from '../../components/ui'
import type { CommunityPost, Product } from '../../types'
import { getMyComments, getMyFavoritePosts, getMyPosts, type MyCommunityComment } from '../../services/b-community'
import { getMyProductsPage } from '../../services/c-trade'
import Taro from '../../services/taro'
import { formatPostDate } from '../../utils/post-presentation'
import { navigateBackOrHome } from '../../utils/navigation'
import { useSessionStore } from '../../stores/session'
import { useContentTabs, type ContentReaders, type ContentTabKey } from './use-content-tabs'
import './index.scss'

type TabKey = ContentTabKey

const contentReaders: ContentReaders = {
  favorites: (page) => getMyFavoritePosts(page, 20),
  posts: (page) => getMyPosts(page, 20),
  comments: (page) => getMyComments(page, 20),
  items: (page) => getMyProductsPage(page, 20),
}

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
  const [failedSrc, setFailedSrc] = useState('')
  return src && src !== failedSrc
    ? <Image className='my-content-thumb' src={src} mode='aspectFill' lazyLoad onError={() => setFailedSrc(src)} />
    : <View className='my-content-thumb my-content-thumb--empty'><Text>{src ? '图片暂不可用' : label}</Text></View>
}

function MyContentContent() {
  const visibility = useFeatureVisibility()
  const visibleTabs = tabs.filter(tab => tab.key === 'items' ? visibility.secondhand : tab.key === 'favorites' ? visibility.forum && visibility.favorites : tab.key === 'comments' ? visibility.forum && visibility.comment : visibility.forum)
  const { activeTab, setActiveTab, states, current, getScrollTop, rememberScrollFor, loadMoreFor, reloadFor, reload } = useContentTabs(getInitialTab(), visibleTabs.map((tab) => tab.key), contentReaders)
  const interaction = useChannelInteraction()
  const favorites = states.favorites.items as CommunityPost[]
  const posts = states.posts.items as CommunityPost[]
  const comments = states.comments.items as MyCommunityComment[]
  const items = states.items.items as Product[]
  const navigating = useRef(false)
  useDidShow(() => { navigating.current = false })

  const navigate = (url: string) => {
    if (!interaction.canActivate() || navigating.current) return
    navigating.current = true
    void Taro.navigateTo({ url }).catch(() => {
      navigating.current = false
      void Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' })
    })
  }

  const openPost = (postId: string, anchor?: string) => {
    const anchorQuery = anchor ? `&anchor=${encodeURIComponent(anchor)}` : ''
    navigate(`/pages/post-detail/index?id=${encodeURIComponent(postId)}${anchorQuery}`)
  }
  const openProduct = (productId: string) => navigate(`/pages/secondhand-detail/index?id=${encodeURIComponent(productId)}`)

  const retry = (tab: TabKey) => {
    if (interaction.canActivate()) reloadFor(tab)
  }

  const state = (tab: TabKey) => {
    const value = states[tab]
    if (value.phase === 'idle' || value.phase === 'loading') {
      return (
        <View className='my-content-state'>
          <Text className='my-content-state__title'>正在加载</Text>
          <Text className='my-content-state__desc'>正在读取你的内容…</Text>
        </View>
      )
    }
    if (value.error) {
      return (
        <View className='my-content-state my-content-state--retry' onClick={() => retry(tab)} ariaRole='button' ariaLabel='重新读取我的内容'>
          <Text className='my-content-state__title'>内容加载失败</Text>
          <Text className='my-content-state__desc'>{value.error}</Text>
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
                ? `${item.author || '校园用户'} · ${formatPostDate(item.createdAt)}`
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
            <Text className='my-content-comment__meta'>{formatPostDate(item.createdAt)} · {reviewLabel(item.reviewStatus)}</Text>
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
            <ProductPrice className='my-content-row__price' value={item.price} size='compact' />
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

  const renderContent = (tab: TabKey) => {
    const status = state(tab)
    if (status) return status
    if (tab === 'posts') return renderPosts(posts)
    if (tab === 'comments') return renderComments()
    if (tab === 'items') return renderItems()
    return renderPosts(favorites, true)
  }

  const renderPage = (tab: TabKey) => {
    const value = states[tab]
    return (
      <ScrollView
        className={`my-content-body${activeTab === tab ? ' my-content-body--active' : ''}`}
        scrollY
        enhanced
        showScrollbar={false}
        lowerThreshold={120}
        scrollTop={getScrollTop(tab)}
        onScroll={(event) => rememberScrollFor(tab, event.detail.scrollTop)}
        onScrollToLower={() => loadMoreFor(tab)}
      >
        {renderContent(tab)}
        {value.loadMoreError ? (
          <Text className='my-content-load-more my-content-load-more--retry' onClick={() => { if (interaction.canActivate()) loadMoreFor(tab) }}>{value.loadMoreError}</Text>
        ) : value.loadingMore ? (
          <Text className='my-content-load-more'>正在加载更多内容…</Text>
        ) : value.items.length > 0 && !value.hasMore ? (
          <Text className='my-content-load-more'>— 已显示全部内容 —</Text>
        ) : null}
      </ScrollView>
    )
  }

  return (
    <View className='my-content-page'>
      <CapsulePageHeader
        title='我的内容'
        left={<AppIconButton name='back' label='返回' className='my-content-header__back' onClick={() => void navigateBackOrHome()} />}
        right={current.refreshError ? (
          <View className='my-content-refresh my-content-refresh--retry' onClick={reload} ariaRole='button' ariaLabel={current.refreshError}>
            <AppIcon name='refresh' className='my-content-refresh__icon' /><Text>重试</Text>
          </View>
        ) : current.refreshing ? <Text className='my-content-refresh my-content-refresh--pending'>更新中</Text> : null}
      />
      <View className='my-content-tabs' style={{ gridTemplateColumns: `repeat(${Math.max(1, visibleTabs.length)}, 1fr)` }}>{visibleTabs.map((tab) => <View key={tab.key} className={`my-content-tab ${activeTab === tab.key ? 'my-content-tab--active' : ''}`} ariaRole='button' ariaLabel={`${tab.label}${activeTab === tab.key ? '，已选中' : ''}`} onClick={() => { if (interaction.canActivate()) setActiveTab(tab.key) }}><Text>{tab.label}</Text></View>)}</View>
      <ChannelPager keys={visibleTabs.map(tab => tab.key)} activeKey={activeTab} onChange={setActiveTab} renderPage={renderPage} interaction={interaction} className='my-content-pager' />
    </View>
  )
}

function MyContentPage() {
  const accountScope = useSessionStore((state) => `${state.profile.id}:${state.token}`)
  return <MyContentContent key={accountScope} />
}

export default withFeaturePage(MyContentPage)
