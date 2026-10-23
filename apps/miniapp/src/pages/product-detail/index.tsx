import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useEffect, useState } from 'react'
import { Button, Image, Text, View } from '@tarojs/components'
import { AvatarMark } from '../../components'
import { changeProductStatus, contactSeller, getProduct, toggleProductFavorite } from '../../services/c-trade'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { Product } from '../../types'
import './index.scss'

const statusLabels:Record<Product['status'],string>={selling:'出售中',reserved:'已预定',sold:'已售出'}

function ProductDetailPage(){
  const params=Taro.getCurrentInstance().router?.params||{}
  const profile=useSessionStore(state=>state.profile)
  const[product,setProduct]=useState<Product|null>(null)
  const[favorited,setFavorited]=useState(false)
  const load=async()=>{if(params.id)setProduct(await getProduct(params.id))}
  useEffect(()=>{void load().catch(showError)},[params.id])

  const isSeller=Boolean(product&&profile.id&&product.sellerId===profile.id)
  const startChat=async()=>{if(!product)return;try{const relation=await contactSeller(product.id);await Taro.navigateTo({url:`/pages/secondhand-chat/index?relationId=${encodeURIComponent(String(relation.relationId))}`})}catch(error){showError(error)}}
  const updateStatus=async(status:Product['status'])=>{if(!product)return;try{await changeProductStatus(product.id,status);await load();await Taro.showToast({title:'商品状态已更新',icon:'success'})}catch(error){showError(error)}}

  if(!product)return <View className='product-detail-page'><View className='card'>商品加载中</View></View>
  return <View className='product-detail-page'>
    {product.images[0]?<Image className='product-cover' src={product.images[0]} mode='aspectFill'/>:<View className='cc-placeholder product-cover'>暂无图片</View>}
    <View className='product-detail-body'>
      <View className='product-info card'><View className='between'><Text className='product-info__price'>¥{product.price}</Text><Text className={`cc-tag status-${product.status}`}>{statusLabels[product.status]}</Text></View>
        <Text className='product-info__title'>{product.title}</Text><Text className='product-info__meta'>{product.category} · {product.campus} · {product.createdAt}</Text><Text className='product-info__description'>{product.description}</Text>
      </View>
      <View className='seller-card card'><View className='row seller-card__profile'><AvatarMark label={product.sellerName} src={product.sellerAvatar}/><View><Text className='seller-card__name'>{product.sellerName}</Text><Text className='seller-card__meta'>校内卖家 · 腾讯云 IM 沟通</Text></View></View><Text className='seller-card__tip'>平台后端不保存聊天消息；消息由腾讯云 IM 承载。</Text></View>
      {isSeller&&product.reviewStatus==='approved'&&<View className='card seller-status'><Text>卖家状态管理</Text><View className='row'>{product.status==='selling'&&<><Button className='cc-button' onClick={()=>void updateStatus('reserved')}>标记预定</Button><Button className='cc-button' onClick={()=>void updateStatus('sold')}>标记售出</Button></>}{product.status==='reserved'&&<><Button className='cc-button' onClick={()=>void updateStatus('selling')}>恢复出售</Button><Button className='cc-button' onClick={()=>void updateStatus('sold')}>标记售出</Button></>}{product.status==='sold'&&<Text>已售出为终态</Text>}</View></View>}
    </View>
    {!isSeller&&<View className='product-actions'><Button className='cc-button' disabled={product.status==='sold'} onClick={()=>void toggleProductFavorite(product.id).then(value=>{setFavorited(value);return Taro.showToast({title:value?'已收藏':'已取消收藏',icon:'success'})}).catch(showError)}>{favorited?'已收藏':'收藏'}</Button><Button className='cc-button cc-button--primary' disabled={product.status==='sold'} onClick={()=>void startChat()}>{product.status==='sold'?'商品已售出':'腾讯云 IM 联系卖家'}</Button></View>}
  </View>
}

function showError(error:unknown){void Taro.showToast({title:error instanceof Error?error.message:'操作失败',icon:'none'})}

export default withFeaturePage(ProductDetailPage)
