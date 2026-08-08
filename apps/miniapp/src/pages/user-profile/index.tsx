import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import { AppActionSheet, AppDialog } from '../../components/ui'
import { blockUser } from '../../services/a-user'
import {
  getPublicCommunityProfile,
  getPublicUserPosts,
  setUserFollowing,
  type PublicCommunityProfile
} from '../../services/b-community'
import { getPublicUserProducts } from '../../services/c-trade'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { CommunityPost, Product } from '../../types'
import { getCapsuleSafeStyle } from '../../utils/capsule-safe'
import './index.scss'

type ProfileTab = 'posts' | 'secondhand'
type PageStatus = 'loading' | 'ready' | 'error'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1600 })
}

function splitColumns<T>(items: T[]) {
  return {
    left: items.filter((_, index) => index % 2 === 0),
    right: items.filter((_, index) => index % 2 === 1)
  }
}

export default function UserProfilePage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const userId = String(params.userId || '').trim()
  const currentUserId = useSessionStore((state) => state.profile.id)
  const initialTab: ProfileTab = params.tab === 'secondhand' ? 'secondhand' : 'posts'
  const [status, setStatus] = useState<PageStatus>('loading')
  const [reloadKey, setReloadKey] = useState(0)
  const [profile, setProfile] = useState<PublicCommunityProfile | null>(null)
  const [posts, setPosts] = useState<CommunityPost[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [postTotal, setPostTotal] = useState(0)
  const [productTotal, setProductTotal] = useState(0)
  const [postPage, setPostPage] = useState(1)
  const [productPage, setProductPage] = useState(1)
  const [loadingMore, setLoadingMore] = useState(false)
  const [loadMoreError, setLoadMoreError] = useState('')
  const [activeTab, setActiveTab] = useState<ProfileTab>(initialTab)
  const [following, setFollowing] = useState(false)
  const [followerCount, setFollowerCount] = useState(0)
  const [menuVisible, setMenuVisible] = useState(false)
  const [busy, setBusy] = useState(false)
  const [blockConfirmVisible, setBlockConfirmVisible] = useState(false)
  const [blockPending, setBlockPending] = useState(false)
  const [error, setError] = useState('')
  const requestVersion = useRef(0)

  useEffect(() => {
    let active = true
    const version = requestVersion.current + 1
    requestVersion.current = version
    setLoadingMore(false)
    setLoadMoreError('')
    setPostPage(1)
    setProductPage(1)
    setStatus('loading')
    setError('')
    if (!/^[1-9]\d*$/.test(userId)) {
      setStatus('error')
      setError('用户主页入口无效，请返回后重新进入')
      return () => { active = false }
    }
    void Promise.all([
      getPublicCommunityProfile(userId),
      getPublicUserPosts(userId, 1, 20),
      getPublicUserProducts(userId, 1, 20)
    ]).then(([profileValue, postPage, productPage]) => {
      if (!active || requestVersion.current !== version) return
      setProfile(profileValue)
      setFollowing(profileValue.isFollowing)
      setFollowerCount(profileValue.followerCount)
      setPosts(postPage.items)
      setProducts(productPage.items)
      setPostTotal(postPage.total)
      setProductTotal(productPage.total)
      setPostPage(postPage.page || 1)
      setProductPage(productPage.page || 1)
      setStatus('ready')
    }).catch((value: unknown) => {
      if (!active || requestVersion.current !== version) return
      setStatus('error')
      setError(value instanceof Error ? value.message : '用户主页加载失败')
    })
    return () => { active = false }
  }, [reloadKey, userId])

  const loadMore = async () => {
    if (loadingMore) return
    const isPosts = activeTab === 'posts'
    const currentItems = isPosts ? posts : products
    const total = isPosts ? postTotal : productTotal
    if (currentItems.length >= total) return
    const nextPage = (isPosts ? postPage : productPage) + 1
    const version = requestVersion.current + 1
    requestVersion.current = version
    setLoadingMore(true)
    setLoadMoreError('')
    try {
      if (isPosts) {
        const result = await getPublicUserPosts(userId, nextPage, 20)
        if (requestVersion.current !== version) return
        setPosts((current) => {
          const byId = new Map(current.map((item) => [item.id, item]))
          result.items.forEach((item) => byId.set(item.id, item))
          return Array.from(byId.values())
        })
        setPostPage(result.page || nextPage)
        setPostTotal(result.total)
      } else {
        const result = await getPublicUserProducts(userId, nextPage, 20)
        if (requestVersion.current !== version) return
        setProducts((current) => {
          const byId = new Map(current.map((item) => [item.id, item]))
          result.items.forEach((item) => byId.set(item.id, item))
          return Array.from(byId.values())
        })
        setProductPage(result.page || nextPage)
        setProductTotal(result.total)
      }
    } catch (value) {
      if (requestVersion.current === version) {
        setLoadMoreError(value instanceof Error ? `${value.message}，点击重试` : '更多内容加载失败，点击重试')
      }
    } finally {
      if (requestVersion.current === version) setLoadingMore(false)
    }
  }

  const postColumns = useMemo(() => splitColumns(posts), [posts])
  const productColumns = useMemo(() => splitColumns(products), [products])
  const isSelf = currentUserId === userId

  const toggleFollow = async () => {
    if (busy || isSelf) return
    const next = !following
    setBusy(true)
    try {
      await setUserFollowing(userId, next)
      setFollowing(next)
      setFollowerCount((count) => Math.max(0, count + (next ? 1 : -1)))
    } catch (value) {
      showToast(value instanceof Error ? value.message : '关注操作失败')
    } finally {
      setBusy(false)
    }
  }

  const secondaryAction = () => {
    if (isSelf) {
      void Taro.setClipboardData({ data: `pages/user-profile/index?userId=${userId}` }).then(() => showToast('主页路径已复制'))
      return
    }
    const selling = products.find((item) => item.status === 'selling')
    if (!selling) {
      showToast('该用户当前没有可联系的在售商品')
      return
    }
    void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(selling.id)}` })
  }

  const copyProfilePath = async () => {
    setMenuVisible(false)
    await Taro.setClipboardData({ data: `pages/user-profile/index?userId=${userId}` })
    showToast('主页路径已复制')
  }

  const reportUser = () => {
    setMenuVisible(false)
    void Taro.navigateTo({
      url: `/pages/feedback/index?category=report&targetUserId=${encodeURIComponent(userId)}`
    })
  }

  const requestBlock = () => {
    setMenuVisible(false)
    setBlockConfirmVisible(true)
  }

  const confirmBlock = async () => {
    if (blockPending) return
    setBlockPending(true)
    try {
      await blockUser(Number(userId))
      setBlockConfirmVisible(false)
      await Taro.showToast({ title: '已加入黑名单', icon: 'success' })
      await Taro.navigateBack()
    } catch (value) {
      showToast(value instanceof Error ? value.message : '拉黑失败')
    } finally {
      setBlockPending(false)
    }
  }

  const renderPost = (item: CommunityPost) => (
    <View className='public-profile-card' key={item.id} onClick={() => void Taro.navigateTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(item.id)}` })}>
      <View className='public-profile-card__media public-profile-card__media--sky public-profile-card__media--medium'>
        {item.images[0] ? <Image className='public-profile-card__media-image' src={item.images[0]} mode='aspectFill' /> : <Text>帖 · {item.topic || '校园'}</Text>}
      </View>
      <View className='public-profile-card__body'>
        <Text className='public-profile-card__title'>{item.title || item.content}</Text>
        <View className='public-profile-card__metric'><Text className='public-profile-card__heart'>♥</Text><Text>{item.likedCount}</Text></View>
      </View>
    </View>
  )

  const renderProduct = (item: Product) => (
    <View className='public-profile-card' key={item.id} onClick={() => void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(item.id)}` })}>
      <View className='public-profile-card__media public-profile-card__media--lemon public-profile-card__media--medium'>
        {item.images[0] ? <Image className='public-profile-card__media-image' src={item.images[0]} mode='aspectFill' /> : <Text>物 · {item.category || '二手'}</Text>}
        {item.status === 'sold' ? <Text className='public-profile-card__sold'>已售</Text> : null}
      </View>
      <View className='public-profile-card__body'>
        <Text className='public-profile-card__title'>{item.title}</Text>
        <View className='public-profile-card__product-meta'>
          <Text className='public-profile-card__price'>¥{item.price}</Text>
          <Text>{item.status === 'selling' ? '出售中' : item.status === 'reserved' ? '已预留' : '已售出'}</Text>
        </View>
      </View>
    </View>
  )

  if (status === 'loading') return <View className='public-profile-page'><View className='public-profile-empty'><Text>正在加载真实主页…</Text></View></View>
  if (status === 'error' || !profile) {
    return (
      <View className='public-profile-page'>
        <View className='public-profile-empty' onClick={() => setReloadKey((value) => value + 1)}>
          <Text className='public-profile-empty__title'>{error || '用户主页加载失败'}</Text>
          <Text className='public-profile-empty__desc'>点击重试</Text>
        </View>
      </View>
    )
  }

  const value = profile.profile
  const empty = activeTab === 'posts' ? posts.length === 0 : products.length === 0
  return (
    <View className='public-profile-page' style={getCapsuleSafeStyle()}>
      <View className='public-profile-hero'>
        <CapsuleSystemRow />
        <View className='public-profile-nav'>
          <View className='public-profile-nav__back' onClick={() => void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/home/index' }))} />
          <View className='public-profile-nav__more' onClick={() => setMenuVisible(true)}><Text>•••</Text></View>
        </View>
        <View className='public-profile-identity'>
          <View className='public-profile-user'>
            <View className='public-profile-user__avatar'>
              {value.avatar ? <Image className='public-profile-user__avatar-image' src={value.avatar} mode='aspectFill' /> : <Text>{value.nickname.slice(0, 1) || '校'}</Text>}
            </View>
            <View className='public-profile-user__main'>
              <Text className='public-profile-user__name'>{value.nickname}</Text>
              <Text className='public-profile-user__meta'>用户编号：{value.id}</Text>
              <Text className='public-profile-user__meta'>{value.school || '未填写学校'}{value.campus ? ` · ${value.campus}` : ''}</Text>
            </View>
          </View>
          <Text className='public-profile-bio'>仅展示该用户审核通过的公开内容</Text>
          <View className='public-profile-stats'>
            <View><Text className='public-profile-stats__value'>{profile.followingCount}</Text><Text>关注</Text></View>
            <View><Text className='public-profile-stats__value'>{followerCount}</Text><Text>粉丝</Text></View>
            <View><Text className='public-profile-stats__value'>{profile.likedCount}</Text><Text>获赞</Text></View>
          </View>
        </View>
        <View className='public-profile-actions'>
          <View className={`public-profile-action public-profile-action--primary ${following ? 'public-profile-action--active' : ''}`} onClick={isSelf ? () => void Taro.navigateTo({ url: '/pages/edit-profile/index' }) : () => void toggleFollow()}>
            <Text>{isSelf ? '编辑资料' : busy ? '处理中…' : following ? '已关注' : '+ 关注'}</Text>
          </View>
          <View className='public-profile-action public-profile-action--secondary' onClick={secondaryAction}>
            <Text>{isSelf ? '分享主页' : '查看在售商品'}</Text>
          </View>
        </View>
      </View>
      <View className='public-profile-tabs'>
        <View className={`public-profile-tab ${activeTab === 'posts' ? 'public-profile-tab--active' : ''}`} onClick={() => setActiveTab('posts')}><Text>帖子 {postTotal}</Text></View>
        <View className={`public-profile-tab ${activeTab === 'secondhand' ? 'public-profile-tab--active' : ''}`} onClick={() => setActiveTab('secondhand')}><Text>二手 {productTotal}</Text></View>
      </View>
      <View className='public-profile-content'>
        <ScrollView
          className='public-profile-scroll'
          scrollY
          enhanced
          showScrollbar={false}
          lowerThreshold={120}
          onScrollToLower={() => void loadMore()}
        >
          {empty ? (
            <View className='public-profile-empty'><Text className='public-profile-empty__title'>{activeTab === 'posts' ? '暂无公开帖子' : '暂无公开二手商品'}</Text></View>
          ) : activeTab === 'posts' ? (
            <View className='public-profile-waterfall'><View className='public-profile-waterfall__column'>{postColumns.left.map(renderPost)}</View><View className='public-profile-waterfall__column'>{postColumns.right.map(renderPost)}</View></View>
          ) : (
            <View className='public-profile-waterfall'><View className='public-profile-waterfall__column'>{productColumns.left.map(renderProduct)}</View><View className='public-profile-waterfall__column'>{productColumns.right.map(renderProduct)}</View></View>
          )}
          {loadMoreError ? <Text className='public-profile-load-more public-profile-load-more--retry' onClick={() => void loadMore()}>{loadMoreError}</Text> : null}
          {loadingMore ? <Text className='public-profile-load-more'>正在加载更多内容…</Text> : null}
        </ScrollView>
      </View>
      <AppActionSheet visible={menuVisible} title='更多操作' onClose={() => setMenuVisible(false)}>
        <View className='public-profile-sheet__content'>
            <View className='public-profile-sheet__group'>
              <View className='public-profile-sheet__item' onClick={() => void copyProfilePath()}><Text>复制主页路径</Text></View>
              {!isSelf ? <View className='public-profile-sheet__item public-profile-sheet__item--warning' onClick={reportUser}><Text>举报该用户</Text></View> : null}
              {!isSelf ? <View className='public-profile-sheet__item public-profile-sheet__item--danger' onClick={requestBlock}><Text>拉黑该用户</Text></View> : null}
            </View>
        </View>
      </AppActionSheet>
      <AppDialog
        visible={blockConfirmVisible}
        title='确认拉黑该用户'
        description='拉黑后将不再展示对方的新内容，记录会进入设置中的黑名单。'
        confirmText='确认拉黑'
        tone='danger'
        loading={blockPending}
        onCancel={() => setBlockConfirmVisible(false)}
        onConfirm={() => void confirmBlock()}
      />
    </View>
  )
}
