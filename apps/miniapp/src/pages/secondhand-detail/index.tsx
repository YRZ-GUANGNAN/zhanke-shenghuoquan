import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { isApiError } from '../../services/api'
import { contactSeller, getProduct, getTradeViewerState, setProductFavorited } from '../../services/c-trade'
import Taro from '../../services/taro'
import type { Product } from '../../types'
import './index.scss'

function showToast(title: string) {
  void Taro.showToast({ title, icon: 'none', duration: 1400 })
}

function writeErrorMessage(error: unknown, action: '收藏' | '联系卖家'): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : `${action}失败，请稍后重试`
  if (error.status === 0) return '网络连接失败，请稍后重试'
  if (error.status === 401 || error.status === 403) return '登录状态已失效，请重新登录'
  if (error.status === 404) return '宝贝不存在或已下架'
  if (error.status === 409) return `当前宝贝状态不允许${action}`
  if (error.status === 422) return action === '联系卖家' ? '不能联系自己发布的宝贝' : '当前收藏状态无法更新'
  return error.message || `${action}失败，请稍后重试`
}

export default function SecondhandDetailPage() {
  const requestedId = Taro.getCurrentInstance().router?.params?.id ?? ''
  const validId = /^[1-9]\d*$/.test(requestedId)
  const [product, setProduct] = useState<Product | null>(null)
  const [status, setStatus] = useState<'loading' | 'ready' | 'empty' | 'error'>(
    validId ? 'loading' : 'empty'
  )
  const [reloadKey, setReloadKey] = useState(0)
  const requestVersion = useRef(0)
  const [imageFailed, setImageFailed] = useState(false)
  const [contactVisible, setContactVisible] = useState(false)
  const [favorited, setFavorited] = useState<boolean | null>(null)
  const [favoritePending, setFavoritePending] = useState(false)
  const [contactPending, setContactPending] = useState(false)
  const retryProduct = useCallback(() => setReloadKey((value) => value + 1), [])

  useEffect(() => {
    if (!validId) {
      setStatus('empty')
      return undefined
    }
    const version = requestVersion.current + 1
    requestVersion.current = version
    let active = true
    setStatus('loading')
    setImageFailed(false)
    setFavorited(null)

    void Promise.all([
      getProduct(requestedId),
      getTradeViewerState(requestedId).catch(() => null)
    ]).then(([value, viewerState]) => {
      if (!active || requestVersion.current !== version) return
      setProduct(value)
      setFavorited(viewerState?.favorited ?? null)
      setStatus('ready')
    }).catch((error: unknown) => {
      if (!active || requestVersion.current !== version) return
      setProduct(null)
      setStatus(isApiError(error) && error.status === 404 ? 'empty' : 'error')
    })

    return () => {
      active = false
      requestVersion.current += 1
    }
  }, [reloadKey, requestedId, validId])

  const goBack = () => {
    void Taro.navigateBack().catch(() => Taro.redirectTo({ url: '/pages/student-secondhand/index' }))
  }

  const openSellerProfile = () => {
    if (!product?.sellerId) {
      showToast('卖家资料暂不可用')
      return
    }
    void Taro.navigateTo({
      url: `/pages/user-profile/index?userId=${encodeURIComponent(product.sellerId)}&tab=secondhand`
    })
  }

  const statusLabel = product?.status === 'selling'
    ? '出售中'
    : product?.status === 'reserved'
      ? '已预订'
      : '已售'
  const tags = product ? [product.category, product.condition].filter(Boolean) : []
  const sellerMeta = product?.campus || '校内'
  const sellerAvatar = product?.sellerName.trim().charAt(0) || '校'
  const firstImage = product?.images[0]

  const handleFavorite = async () => {
    if (!product || favoritePending) return
    setFavoritePending(true)
    try {
      const serverState = favorited === null ? await getTradeViewerState(product.id) : null
      const desired = !(serverState?.favorited ?? favorited)
      if (product.status === 'sold' && desired) {
        showToast('已售商品不能新增收藏')
        return
      }
      const next = await setProductFavorited(product.id, desired)
      setFavorited(next)
      await Taro.showToast({ title: next ? '已收藏' : '已取消收藏', icon: 'success', duration: 1400 })
    } catch (error) {
      try {
        const serverState = await getTradeViewerState(product.id)
        setFavorited(serverState.favorited)
      } catch {
        // 结果未知时保留最后一次已确认状态，不在前端猜测成功。
      }
      await Taro.showToast({ title: writeErrorMessage(error, '收藏'), icon: 'none', duration: 1800 })
    } finally {
      setFavoritePending(false)
    }
  }

  const handleStartChat = async () => {
    if (!product || contactPending) return
    if (product.status === 'sold') {
      showToast('已售商品不能新建私信会话')
      return
    }
    setContactPending(true)
    try {
      await contactSeller(product.id)
      setContactVisible(false)
      await Taro.navigateTo({
        url: `/pages/secondhand-chat/index?productId=${encodeURIComponent(product.id)}`
      })
    } catch (error) {
      await Taro.showToast({ title: writeErrorMessage(error, '联系卖家'), icon: 'none', duration: 1800 })
    } finally {
      setContactPending(false)
    }
  }

  const renderState = () => {
    const stateCopy = status === 'loading'
      ? { title: '正在加载宝贝', description: '请稍候...' }
      : status === 'empty'
        ? { title: '宝贝不存在或已下架', description: '请返回二手集市重新选择。' }
        : { title: '加载失败，点击重试', description: '网络开小差了，请稍后再试。' }
    return (
      <View
        className={`secondhand-detail-info secondhand-detail-info--state ${status === 'error' ? 'secondhand-detail-info--retry' : ''}`}
        onClick={status === 'error' ? retryProduct : undefined}
      >
        <Text className='secondhand-detail-state__title'>{stateCopy.title}</Text>
        <Text className='secondhand-detail-state__desc'>{stateCopy.description}</Text>
      </View>
    )
  }

  return (
    <View className='secondhand-detail-page'>
      <CapsulePageHeader
        title='宝贝详情'
        left={<View className='secondhand-detail-header__back' onClick={goBack} />}
      />

      <ScrollView className='secondhand-detail-scroll' scrollY enhanced showScrollbar={false}>
        {status !== 'ready' || !product ? renderState() : (
          <>
            <View
              className='secondhand-detail-hero'
              onClick={() => {
                if (!firstImage || imageFailed || !product) return
                void Taro.previewImage({ urls: product.images, current: firstImage })
                  .catch(() => showToast('图片预览失败，请稍后重试'))
              }}
            >
              {firstImage && !imageFailed ? (
                <Image
                  className='secondhand-detail-hero__image'
                  src={firstImage}
                  mode='aspectFill'
                  onError={() => setImageFailed(true)}
                />
              ) : (
                <Text>图 · 卖家未上传图片</Text>
              )}
            </View>

            <View className='secondhand-detail-info'>
              <View className='secondhand-detail-info__price-row'>
                <Text className='secondhand-detail-info__price'>
                  <Text className='secondhand-detail-info__currency'>¥</Text>{product.price}
                </Text>
                <View className='secondhand-detail-info__status'>
                  <Text>{statusLabel}</Text>
                </View>
              </View>

              <Text className='secondhand-detail-info__title'>{product.title}</Text>
              <Text className='secondhand-detail-info__desc'>{product.description}</Text>

              <View className='secondhand-detail-tags'>
                {tags.map((tag) => (
                  <View className='secondhand-detail-tag' key={tag}>
                    <Text>{tag}</Text>
                  </View>
                ))}
              </View>

              <Text className='secondhand-detail-info__meta'>
                {product.createdAt ? `${product.createdAt} 发布` : '发布时间未知'}
              </Text>
            </View>

            <View className='secondhand-detail-section'>
              <View className='secondhand-detail-seller'>
                <View className='secondhand-detail-seller__avatar'>
                  <Text>{sellerAvatar}</Text>
                </View>
                <View className='secondhand-detail-seller__main'>
                  <Text className='secondhand-detail-seller__name'>{product.sellerName}</Text>
                  <Text className='secondhand-detail-seller__meta'>{sellerMeta}</Text>
                </View>
                <View className='secondhand-detail-seller__button' onClick={openSellerProfile}>
                  <Text>主页</Text>
                </View>
              </View>

              <View className='secondhand-detail-warning'>
                <Text className='secondhand-detail-warning__icon'>△</Text>
                <Text>平台仅展示信息、不参与交易。请与卖家私下当面交易，验货后再付款，谨防转账诈骗。</Text>
              </View>
            </View>
          </>
        )}
      </ScrollView>

      {status === 'ready' && product && (
        <View className='secondhand-detail-actionbar'>
          <View
            className={`secondhand-detail-favorite ${favorited ? 'secondhand-detail-favorite--active' : ''} ${favoritePending ? 'secondhand-detail-favorite--pending' : ''}`}
            onClick={() => void handleFavorite()}
          >
            <View className='secondhand-detail-favorite__star' />
            <Text>{favoritePending ? '处理中' : favorited ? '已收藏' : '收藏'}</Text>
          </View>
          <View
            className={`secondhand-detail-contact ${contactPending ? 'secondhand-detail-contact--pending' : ''}`}
            onClick={() => !contactPending && setContactVisible(true)}
          >
            <View className='secondhand-detail-contact__icon' />
            <Text>联系卖家</Text>
          </View>
        </View>
      )}

      {contactVisible && product && (
        <View className='secondhand-contact-modal'>
          <View className='secondhand-contact-modal__mask' onClick={() => !contactPending && setContactVisible(false)} />
          <View className='secondhand-contact-sheet'>
            <View className='secondhand-contact-sheet__head'>
              <Text className='secondhand-contact-sheet__title'>联系卖家</Text>
              <View className='secondhand-contact-sheet__close' onClick={() => !contactPending && setContactVisible(false)}>
                <Text>×</Text>
              </View>
            </View>

            <View className='secondhand-contact-option' onClick={() => void handleStartChat()}>
              <View className='secondhand-contact-option__icon secondhand-contact-option__icon--chat' />
              <View className='secondhand-contact-option__main'>
                <Text className='secondhand-contact-option__title'>发起私聊</Text>
                <Text className='secondhand-contact-option__desc'>
                  {contactPending ? '正在建立安全会话' : '通过平台私信沟通'}
                </Text>
              </View>
              <Text className='secondhand-contact-option__arrow'>›</Text>
            </View>

            <View className='secondhand-contact-option' onClick={() => showToast('卖家未提供微信联系方式')}>
              <View className='secondhand-contact-option__icon secondhand-contact-option__icon--wechat' />
              <View className='secondhand-contact-option__main'>
                <Text className='secondhand-contact-option__title'>加微信联系</Text>
                <Text className='secondhand-contact-option__desc'>卖家未提供微信联系方式</Text>
              </View>
              <Text className='secondhand-contact-option__arrow'>›</Text>
            </View>

            <Text className='secondhand-contact-sheet__safe'>交易由买卖双方私下完成，平台不担保、不代收款</Text>
          </View>
        </View>
      )}
    </View>
  )
}
