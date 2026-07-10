import React, { useEffect, useMemo, useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import {
  type ProfileTab,
  type PublicProfilePost,
  type PublicProfileProduct,
  resolvePublicProfile
} from './data'
import './index.scss'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function splitColumns<T>(items: T[]) {
  return {
    left: items.filter((_, index) => index % 2 === 0),
    right: items.filter((_, index) => index % 2 === 1)
  }
}

export default function UserProfilePage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const initialTab: ProfileTab = params.tab === 'secondhand' ? 'secondhand' : 'posts'
  const resolved = useMemo(() => resolvePublicProfile(params.userId), [params.userId])
  const profile = resolved.profile
  const [activeTab, setActiveTab] = useState<ProfileTab>(initialTab)
  const [following, setFollowing] = useState(profile.isFollowing)
  const [followerCount, setFollowerCount] = useState(profile.followerCount)
  const [menuVisible, setMenuVisible] = useState(false)
  const postColumns = useMemo(() => splitColumns(profile.posts), [profile.posts])
  const productColumns = useMemo(() => splitColumns(profile.products), [profile.products])

  useEffect(() => {
    if (resolved.usedFallback) showToast('未找到该用户，已显示通用主页')
  }, [resolved.usedFallback])

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/profile/index' }))
  }

  const toggleFollow = () => {
    setFollowing((value) => {
      setFollowerCount((count) => Math.max(0, count + (value ? -1 : 1)))
      return !value
    })
  }

  const handleSecondaryAction = () => {
    if (profile.isSelf) {
      showToast('主页分享静态预览')
      return
    }
    showToast('私信功能待接入通用会话')
  }

  const handleMenuAction = (action: 'copy' | 'report' | 'block') => {
    setMenuVisible(false)
    const copy = {
      copy: '主页链接已复制（静态预览）',
      report: '举报入口已记录为静态预览',
      block: '拉黑入口已记录为静态预览'
    }
    showToast(copy[action])
  }

  const openPost = (item: PublicProfilePost) => {
    void Taro.navigateTo({ url: `/pages/post-detail/index?id=${encodeURIComponent(item.id)}` })
  }

  const openProduct = (item: PublicProfileProduct) => {
    void Taro.navigateTo({ url: `/pages/secondhand-detail/index?id=${encodeURIComponent(item.id)}` })
  }

  const renderPost = (item: PublicProfilePost) => (
    <View className='public-profile-card' key={item.id} onClick={() => openPost(item)}>
      <View className={`public-profile-card__media public-profile-card__media--${item.tone} public-profile-card__media--${item.height}`}>
        <Text>{item.mediaLabel}</Text>
      </View>
      <View className='public-profile-card__body'>
        <Text className='public-profile-card__title'>{item.title}</Text>
        <View className='public-profile-card__metric'>
          <Text className='public-profile-card__heart'>♥</Text>
          <Text>{item.likes}</Text>
        </View>
      </View>
    </View>
  )

  const renderProduct = (item: PublicProfileProduct) => (
    <View className='public-profile-card' key={item.id} onClick={() => openProduct(item)}>
      <View className={`public-profile-card__media public-profile-card__media--${item.tone} public-profile-card__media--${item.height}`}>
        <Text>{item.mediaLabel}</Text>
        {item.status === 'sold' && <Text className='public-profile-card__sold'>已售</Text>}
      </View>
      <View className='public-profile-card__body'>
        <Text className='public-profile-card__title'>{item.title}</Text>
        <View className='public-profile-card__product-meta'>
          <Text className='public-profile-card__price'>¥{item.price}</Text>
          <Text>{item.status === 'selling' ? '出售中' : '已售出'}</Text>
        </View>
      </View>
    </View>
  )

  const empty = activeTab === 'posts' ? profile.posts.length === 0 : profile.products.length === 0

  return (
    <View className='public-profile-page'>
      <View className='public-profile-hero'>
        <View className='public-profile-nav'>
          <View className='public-profile-nav__back' onClick={goBack} />
          <View className='public-profile-nav__more' onClick={() => setMenuVisible(true)}>
            <Text>•••</Text>
          </View>
        </View>

        <View className='public-profile-user'>
          <View className='public-profile-user__avatar'><Text>{profile.avatarText}</Text></View>
          <View className='public-profile-user__main'>
            <Text className='public-profile-user__name'>{profile.nickname}</Text>
            <Text className='public-profile-user__meta'>湛科号：{profile.account}</Text>
            <Text className='public-profile-user__meta'>{profile.school}{profile.grade ? ` · ${profile.grade}` : ''}</Text>
          </View>
        </View>

        <Text className='public-profile-bio'>{profile.bio}</Text>

        <View className='public-profile-stats'>
          <View><Text className='public-profile-stats__value'>{profile.followingCount}</Text><Text>关注</Text></View>
          <View><Text className='public-profile-stats__value'>{followerCount}</Text><Text>粉丝</Text></View>
          <View><Text className='public-profile-stats__value'>{profile.likedCount}</Text><Text>获赞</Text></View>
        </View>

        <View className='public-profile-actions'>
          <View className={`public-profile-action public-profile-action--primary ${following ? 'public-profile-action--active' : ''}`} onClick={profile.isSelf ? () => void Taro.navigateTo({ url: '/pages/edit-profile/index' }) : toggleFollow}>
            <Text>{profile.isSelf ? '编辑资料' : following ? '已关注' : '+ 关注'}</Text>
          </View>
          <View className='public-profile-action public-profile-action--secondary' onClick={handleSecondaryAction}>
            <Text>{profile.isSelf ? '分享主页' : '▱ 私信'}</Text>
          </View>
        </View>
      </View>

      <View className='public-profile-tabs'>
        <View className={`public-profile-tab ${activeTab === 'posts' ? 'public-profile-tab--active' : ''}`} onClick={() => setActiveTab('posts')}>
          <Text>帖子 {profile.posts.length}</Text>
        </View>
        <View className={`public-profile-tab ${activeTab === 'secondhand' ? 'public-profile-tab--active' : ''}`} onClick={() => setActiveTab('secondhand')}>
          <Text>二手 {profile.products.length}</Text>
        </View>
      </View>

      <View className='public-profile-content'>
        <ScrollView className='public-profile-scroll' scrollY enhanced showScrollbar={false}>
          {empty ? (
            <View className='public-profile-empty'>
              <Text className='public-profile-empty__icon'>{activeTab === 'posts' ? '文' : '物'}</Text>
              <Text className='public-profile-empty__title'>{activeTab === 'posts' ? '还没有发布帖子' : '还没有发布二手'}</Text>
              <Text className='public-profile-empty__desc'>发布后会展示在个人主页</Text>
            </View>
          ) : activeTab === 'posts' ? (
            <View className='public-profile-waterfall'>
              <View className='public-profile-waterfall__column'>{postColumns.left.map(renderPost)}</View>
              <View className='public-profile-waterfall__column'>{postColumns.right.map(renderPost)}</View>
            </View>
          ) : (
            <View className='public-profile-waterfall'>
              <View className='public-profile-waterfall__column'>{productColumns.left.map(renderProduct)}</View>
              <View className='public-profile-waterfall__column'>{productColumns.right.map(renderProduct)}</View>
            </View>
          )}
        </ScrollView>
      </View>

      {menuVisible && (
        <View className='public-profile-sheet-layer'>
          <View className='public-profile-sheet-layer__mask' onClick={() => setMenuVisible(false)} />
          <View className='public-profile-sheet'>
            <View className='public-profile-sheet__group'>
              <View className='public-profile-sheet__item' onClick={() => handleMenuAction('copy')}><Text>复制主页链接</Text></View>
              {!profile.isSelf && <View className='public-profile-sheet__item public-profile-sheet__item--warning' onClick={() => handleMenuAction('report')}><Text>举报该用户</Text></View>}
              {!profile.isSelf && <View className='public-profile-sheet__item public-profile-sheet__item--danger' onClick={() => handleMenuAction('block')}><Text>拉黑该用户</Text></View>}
            </View>
            <View className='public-profile-sheet__cancel' onClick={() => setMenuVisible(false)}><Text>取消</Text></View>
          </View>
        </View>
      )}
    </View>
  )
}
