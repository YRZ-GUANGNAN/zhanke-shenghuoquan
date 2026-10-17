import { withFeaturePage } from '../../components/feature-page-gate'
import { usePageShare } from '../../hooks/use-page-share'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Image, ScrollView, Swiper, SwiperItem, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import AppIcon from '../../components/app-icon'
import { ConfirmedIcon, ExpandableText } from '../../components/content-motion'
import { AppActionSheet, AppDialog, AppIconButton, AppNotice, AppState } from '../../components/ui'
import { isApiError } from '../../services/api'
import { changeProductStatus, contactSeller, getProduct, getTradeViewerState, setProductFavorited } from '../../services/c-trade'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { Product } from '../../types'
import { formatPostDate, preparePostMedia, removeFailedPostMedia, type PostGalleryState } from '../../utils/post-presentation'
import { useGalleryPresentation } from '../../hooks/use-gallery-presentation'
import ProductComments from './product-comments'
import { navigateToPublicProfile } from '../../utils/navigation'
import './index.scss'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function writeErrorMessage(error: unknown, action: '收藏' | '联系卖家' | '更新状态'): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : `${action}失败，请稍后重试`
  if (error.status === 0) return '网络连接失败，请稍后重试'
  if (error.status === 401 || error.status === 403) return '登录状态已失效，请重新登录'
  if (error.status === 404) return '宝贝不存在或已下架'
  if (error.status === 409) return `当前宝贝状态不允许${action}`
  if (error.status === 422) return action === '联系卖家' ? '不能联系自己发布的宝贝' : '当前收藏状态无法更新'
  return error.message || `${action}失败，请稍后重试`
}

function SecondhandDetailPage() {
  const visibility = useFeatureVisibility()
  const requestedId = Taro.getCurrentInstance().router?.params?.id ?? ''
  const targetCommentId = Taro.getCurrentInstance().router?.params?.commentId ?? ''
  const validId = /^[1-9]\d*$/.test(requestedId)
  const currentUserId = useSessionStore((state) => state.profile.id)
  const sessionToken = useSessionStore((state) => state.token)
  const ownerKey = `${requestedId}:${currentUserId}:${sessionToken}`
  const ownerRef = useRef(ownerKey)
  ownerRef.current = ownerKey
  const [product, setProduct] = useState<Product | null>(null)
  const [descriptionLayoutReady, setDescriptionLayoutReady] = useState(false)
  const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>(
    validId ? 'loading' : 'empty'
  )
  const [reloadKey, setReloadKey] = useState(0)
  const requestVersion = useRef(0)
  const [gallery, setGallery] = useState<PostGalleryState>({ items: [], index: 0 })
  const [mediaPending, setMediaPending] = useState(false)
  const [avatarFailed, setAvatarFailed] = useState(false)
  const [viewerLoading, setViewerLoading] = useState(true)
  const actionLocks = useRef({ favorite: false, contact: false, status: false })
  const [contactVisible, setContactVisible] = useState(false)
  const [favorited, setFavorited] = useState<boolean | null>(null)
  const [favoritePending, setFavoritePending] = useState(false)
  const [favoriteFeedback, setFavoriteFeedback] = useState(0)
  const [contactPending, setContactPending] = useState(false)
  const [statusPending, setStatusPending] = useState(false)
  const [statusCandidate, setStatusCandidate] = useState<Product['status'] | null>(null)
  const [commentsVisible, setCommentsVisible] = useState(Boolean(targetCommentId))
  const retryProduct = useCallback(() => setReloadKey((value) => value + 1), [])
  usePageShare({ kind: 'product', enabled: visibility.secondhand && status === 'ready', id: requestedId, content: product })

  useEffect(() => {
    if (!validId) {
      setStatus('empty')
      return undefined
    }
    const version = requestVersion.current + 1
    requestVersion.current = version
    let active = true
    setStatus('loading')
    setDescriptionLayoutReady(false)
    setGallery({ items: [], index: 0 })
    setMediaPending(false)
    setAvatarFailed(false)
    setViewerLoading(true)
    setFavorited(null)
    actionLocks.current = { favorite: false, contact: false, status: false }
    setFavoritePending(false)
    setFavoriteFeedback(0)
    setContactPending(false)
    setStatusPending(false)
    setContactVisible(false)
    setCommentsVisible(Boolean(targetCommentId))
    setStatusCandidate(null)

    void getTradeViewerState(requestedId).catch(() => null).then((viewerState) => {
      if (!active || requestVersion.current !== version || ownerRef.current !== ownerKey) return
      setFavorited(viewerState?.favorited ?? null)
      setViewerLoading(false)
    })
    void getProduct(requestedId).then(async (value) => {
      if (!active || requestVersion.current !== version || ownerRef.current !== ownerKey) return
      setProduct(value)
      setStatus('ready')
      setMediaPending(value.images.length > 0)
      const media = await preparePostMedia(value.images, (src) => Taro.getImageInfo({ src }))
      if (!active || requestVersion.current !== version || ownerRef.current !== ownerKey) return
      setGallery({ items: media, index: 0 })
      setMediaPending(false)
    }).catch((error: unknown) => {
      if (!active || requestVersion.current !== version || ownerRef.current !== ownerKey) return
      setProduct(null)
      setStatus(isApiError(error) && error.status === 404 ? 'empty' : 'error')
    })

    return () => {
      active = false
      requestVersion.current += 1
    }
  }, [reloadKey, requestedId, validId, ownerKey])

  useEffect(() => { if (!visibility.secondhand_comments) setCommentsVisible(false) }, [visibility.secondhand_comments])

  const goBack = () => {
    if (commentsVisible) {
      setCommentsVisible(false)
      void Taro.hideKeyboard().catch(() => {})
      return
    }
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/student-secondhand/index' }))
  }

  const openSellerProfile = () => {
    if (!visibility.forum) return
    if (!product?.sellerId) {
      showToast('卖家资料暂不可用')
      return
    }
    void navigateToPublicProfile(product.sellerId, 'secondhand')
  }

  const statusLabel = product?.status === 'selling'
    ? '出售中'
    : product?.status === 'reserved'
      ? '已预订'
      : '已售'
  const tags = product ? [product.category, product.condition].filter(Boolean) : []
  const sellerMeta = product?.campus || '校内'
  const sellerAvatar = product?.sellerName.trim().charAt(0) || '校'
  const isSeller = Boolean(product && currentUserId === product.sellerId)
  const hasActions = isSeller || visibility.favorites || visibility.secondhand_chat || visibility.secondhand_comments
  const galleryView = useGalleryPresentation(requestedId, gallery.items.length, undefined, 'product', hasActions)

  const handleFavorite = async () => {
    if (!product || !visibility.favorites || viewerLoading || actionLocks.current.favorite) return
    const version = requestVersion.current
    const isCurrent = () => requestVersion.current === version && ownerRef.current === ownerKey
    actionLocks.current.favorite = true
    setFavoritePending(true)
    try {
      const serverState = favorited === null ? await getTradeViewerState(product.id) : null
      if (!isCurrent()) return
      const desired = !(serverState?.favorited ?? favorited)
      if (product.status === 'sold' && desired) {
        showToast('已售商品不能新增收藏')
        return
      }
      const next = await setProductFavorited(product.id, desired)
      if (!isCurrent()) return
      setFavorited(next)
      setFavoriteFeedback(value => value + 1)
      await Taro.showToast({ title: next ? '已收藏' : '已取消收藏', icon: 'success', duration: 1400 })
    } catch (error) {
      if (!isCurrent()) return
      try {
        const serverState = await getTradeViewerState(product.id)
        if (!isCurrent()) return
        setFavorited(serverState.favorited)
      } catch {
        // 结果未知时保留最后一次已确认状态，不在前端猜测成功。
      }
      if (isCurrent()) await Taro.showToast({ title: writeErrorMessage(error, '收藏'), icon: 'none', duration: 1800 })
    } finally {
      if (isCurrent()) {
        actionLocks.current.favorite = false
        setFavoritePending(false)
      }
    }
  }

  const handleStartChat = async () => {
    if (!product || !visibility.secondhand_chat || actionLocks.current.contact) return
    const version = requestVersion.current
    const isCurrent = () => requestVersion.current === version && ownerRef.current === ownerKey
    if (product.status === 'sold') {
      showToast('已售商品不能新建私信会话')
      return
    }
    actionLocks.current.contact = true
    setContactPending(true)
    try {
      const relation = await contactSeller(product.id)
      if (!isCurrent()) return
      setContactVisible(false)
      await Taro.navigateTo({
        url: `/pages/secondhand-chat/index?relationId=${encodeURIComponent(String(relation.relationId))}`
      })
    } catch (error) {
      if (isCurrent()) await Taro.showToast({ title: writeErrorMessage(error, '联系卖家'), icon: 'none', duration: 1800 })
    } finally {
      if (isCurrent()) {
        actionLocks.current.contact = false
        setContactPending(false)
      }
    }
  }

  const requestStatusChange = (nextStatus: Product['status']) => {
    if (!product || actionLocks.current.status || !isSeller) return
    setStatusCandidate(nextStatus)
  }

  const confirmStatusChange = async () => {
    const nextStatus = statusCandidate
    if (!product || actionLocks.current.status || !isSeller) return
    if (!nextStatus) return
    const version = requestVersion.current
    const isCurrent = () => requestVersion.current === version && ownerRef.current === ownerKey
    actionLocks.current.status = true
    setStatusPending(true)
    try {
      await changeProductStatus(product.id, nextStatus)
      if (!isCurrent()) return
      const updated = await getProduct(product.id)
      if (!isCurrent()) return
      setProduct(updated)
      setStatusCandidate(null)
      await Taro.showToast({ title: '状态已更新', icon: 'success' })
    } catch (error) {
      if (isCurrent()) await Taro.showToast({ title: writeErrorMessage(error, '更新状态'), icon: 'none', duration: 1800 })
    } finally {
      if (isCurrent()) {
        actionLocks.current.status = false
        setStatusPending(false)
      }
    }
  }

  const renderState = () => {
    const stateCopy = status === 'loading'
      ? { title: '正在加载宝贝', description: '请稍候...' }
      : status === 'empty'
        ? { title: '宝贝不存在或已下架', description: '请返回二手集市重新选择。' }
        : { title: '加载失败，点击重试', description: '网络开小差了，请稍后再试。' }
    return <AppState kind={status === 'ready' ? 'empty' : status} title={stateCopy.title} description={stateCopy.description} actionText={status === 'error' ? '重新加载' : undefined} onAction={status === 'error' ? retryProduct : undefined} />
  }

  return (
    <View className='secondhand-detail-page'>
      <CapsulePageHeader
        className='secondhand-detail-header'
        title='宝贝详情'
        left={<AppIconButton name='back' label='返回' className='secondhand-detail-header__back' onClick={goBack} />}
      />

      <ScrollView
        className='secondhand-detail-scroll'
        scrollY={!commentsVisible}
        enhanced
        showScrollbar={false}
      >
        {status !== 'ready' || !product ? renderState() : (
          <>
            <View className='secondhand-detail-seller'>
              <View className='secondhand-detail-seller__avatar' ariaRole={visibility.forum ? 'button' : undefined} ariaLabel='查看卖家主页' onClick={openSellerProfile}>
                {product.sellerAvatar && !avatarFailed ? <Image src={product.sellerAvatar} mode='aspectFill' onError={() => setAvatarFailed(true)} /> : <Text>{sellerAvatar}</Text>}
              </View>
              <View className='secondhand-detail-seller__main' ariaRole={visibility.forum ? 'button' : undefined} ariaLabel='查看卖家主页' onClick={openSellerProfile}>
                <Text className='secondhand-detail-seller__name'>{product.sellerName}</Text>
                <Text className='secondhand-detail-seller__meta'>{sellerMeta}{product.createdAt ? ` · ${formatPostDate(product.createdAt)} 发布` : ''}</Text>
              </View>
              {visibility.forum && <View className='secondhand-detail-seller__button' onClick={openSellerProfile} hoverClass='secondhand-detail-control--pressed'>
                <Text>主页</Text><AppIcon name='chevron-right' />
              </View>}
            </View>
            <View className='secondhand-detail-info'>
              <View className='secondhand-detail-info__price-row'>
                <Text className='secondhand-detail-info__price'><Text className='secondhand-detail-info__currency'>¥</Text>{product.price}</Text>
                <View className={`secondhand-detail-info__status secondhand-detail-info__status--${product.status}`}><Text>{statusLabel}</Text></View>
              </View>
              <Text className='secondhand-detail-info__title'>{product.title}</Text>
              {tags.length > 0 && <View className='secondhand-detail-tags'>
                {tags.map(tag => <View className='secondhand-detail-tag' key={tag}><Text>{tag}</Text></View>)}
              </View>}
              <ExpandableText key={product.id} text={product.description} lines={8} className='secondhand-detail-info__desc' onLayoutReady={setDescriptionLayoutReady} />
            </View>
            <View className={'secondhand-detail-hero' + (product.images.length === 0 ? ' secondhand-detail-hero--empty' : '')}
              style={product.images.length ? { height: `${galleryView.height}px` } : undefined}
              onTouchStart={galleryView.onTouchStart} onTouchEnd={galleryView.onTouchEnd} onTouchCancel={galleryView.onTouchEnd}>
              {mediaPending ? <Text>正在读取商品图片…</Text> : gallery.items.length > 0 ? <>
                <Swiper className='secondhand-detail-gallery' current={gallery.index} circular={false} duration={200}
                  onChange={(event) => {
                    if (event.detail.source !== 'touch') return
                    setGallery((current) => ({ ...current, index: Math.max(0, Math.min(event.detail.current, current.items.length - 1)) }))
                    galleryView.revealCounter()
                  }}>
                  {gallery.items.map((image) => <SwiperItem key={image.src}>
                    <Image className='secondhand-detail-hero__image' src={image.src} mode='aspectFit'
                      onError={() => setGallery((current) => removeFailedPostMedia(current, image.src))}
                      onClick={() => { void Taro.previewImage({ urls: gallery.items.map((item) => item.src), current: image.src }).catch(() => showToast('图片预览失败，请稍后重试')) }} />
                  </SwiperItem>)}
                </Swiper>
                {gallery.items.length > 1 && <Text className={'secondhand-detail-gallery__count' + (galleryView.counterVisible ? ' secondhand-detail-gallery__count--visible' : '')}>{gallery.index + 1} / {gallery.items.length}</Text>}
              </> : <Text>{product.images.length ? '图片暂不可用，商品信息仍可查看' : '卖家未上传图片'}</Text>}
            </View>

            <View className='secondhand-detail-section'>
              <View className='secondhand-detail-warning'>
                <AppIcon name='info' className='secondhand-detail-warning__icon' />
                <Text>平台仅展示信息、不参与交易。请与卖家私下当面交易，验货后再付款，谨防转账诈骗。</Text>
              </View>

              {visibility.secondhand_comments && <View className='secondhand-detail-comments-entry' onClick={() => setCommentsVisible(true)} ariaRole='button' ariaLabel='查看商品留言'>
                <View><Text className='secondhand-detail-comments-entry__title'>商品留言</Text><Text className='secondhand-detail-comments-entry__hint'>{product.status === 'sold' ? '查看这件宝贝的留言' : '成色、规格和交付细节，在这里问卖家'}</Text></View>
                <AppIcon name='chevron-right' />
              </View>}
            </View>
          </>
        )}
      </ScrollView>

      {status === 'ready' && product && hasActions && (
        <View className={`secondhand-detail-actionbar ${isSeller ? 'secondhand-detail-actionbar--seller' : ''}`}>
          {visibility.secondhand_comments && <View className='secondhand-detail-comment-action' onClick={() => setCommentsVisible(true)} ariaRole='button' ariaLabel='打开商品留言'>
            <AppIcon name='comment' /><Text>留言</Text>
          </View>}
          {isSeller ? (
            <View className='secondhand-detail-seller-actions'>
              {product.status === 'selling' && (
                <View className='secondhand-detail-seller-action' onClick={() => requestStatusChange('reserved')}>
                  <Text>标记预订</Text>
                </View>
              )}
              {product.status !== 'sold' && (
                <View className='secondhand-detail-seller-action secondhand-detail-seller-action--primary' onClick={() => requestStatusChange('sold')}>
                  <Text>标记售出</Text>
                </View>
              )}
              {product.status !== 'selling' && (
                <View className='secondhand-detail-seller-action secondhand-detail-seller-action--primary' onClick={() => requestStatusChange('selling')}>
                  <Text>重新上架</Text>
                </View>
              )}
            </View>
          ) : (
            <>
              {visibility.favorites && <View
                className={`secondhand-detail-favorite ${favorited ? 'secondhand-detail-favorite--active' : ''} ${favoritePending || viewerLoading ? 'secondhand-detail-favorite--pending' : ''}`}
                ariaRole='button'
                ariaLabel={favorited ? '取消收藏' : '收藏商品'}
                onClick={() => void handleFavorite()}
              >
                <ConfirmedIcon
                  name='star'
                  active={Boolean(favorited)}
                  event={favoriteFeedback}
                  owner={ownerKey}
                  className='secondhand-detail-favorite__star'
                />
                <Text>{viewerLoading ? '读取中' : favoritePending ? '处理中' : favorited ? '已收藏' : '收藏'}</Text>
              </View>}
              {visibility.secondhand_chat && <View
                className={`secondhand-detail-contact ${contactPending ? 'secondhand-detail-contact--pending' : ''} ${product.status === 'sold' ? 'secondhand-detail-contact--disabled' : ''}`}
                onClick={() => {
                  if (contactPending) return
                  if (product.status === 'sold') {
                    showToast('商品已售出，不能新建私信会话')
                    return
                  }
                  setContactVisible(true)
                }}
              >
                <AppIcon name='comment' className='secondhand-detail-contact__icon' />
                <Text>{product.status === 'sold' ? '商品已售' : '联系卖家'}</Text>
              </View>}
            </>
          )}
        </View>
      )}

      {status === 'ready' && product && visibility.secondhand_comments && <ProductComments
        key={ownerKey} productId={product.id} productStatus={product.status} targetCommentId={targetCommentId}
        layoutReady={descriptionLayoutReady && !mediaPending} sheetVisible={commentsVisible} onClose={() => setCommentsVisible(false)}
      />}

      <AppActionSheet visible={contactVisible && visibility.secondhand_chat && Boolean(product)} title='联系卖家' description='请选择一种安全的沟通方式' loading={contactPending} onClose={() => setContactVisible(false)}>
        {product ? <View className='secondhand-contact-sheet__content'>
            <View className='secondhand-contact-option' onClick={() => void handleStartChat()}>
              <AppIcon name='comment' className='secondhand-contact-option__icon' />
              <View className='secondhand-contact-option__main'>
                <Text className='secondhand-contact-option__title'>发起私聊</Text>
                <Text className='secondhand-contact-option__desc'>
                  {contactPending ? '正在建立安全会话' : '通过平台私信沟通'}
                </Text>
              </View>
              <AppIcon name='chevron-right' className='secondhand-contact-option__arrow app-icon--chevron' />
            </View>

          <AppNotice tone='warning'>交易由买卖双方私下完成，平台不担保、不代收款。</AppNotice>
        </View> : null}
      </AppActionSheet>

      <AppDialog
        visible={statusCandidate !== null}
        title={statusCandidate === 'selling' ? '重新上架' : statusCandidate === 'reserved' ? '标记预订' : '标记售出'}
        description={product && statusCandidate ? `确认更新“${product.title}”的商品状态吗？` : ''}
        confirmText='确认更新'
        loading={statusPending}
        onCancel={() => setStatusCandidate(null)}
        onConfirm={() => void confirmStatusChange()}
      />
    </View>
  )
}

export default withFeaturePage(SecondhandDetailPage, RuntimeSessionGate)
