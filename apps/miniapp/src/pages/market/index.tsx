import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useEffect, useMemo, useState } from 'react'
import { Button, Image, Input, ScrollView, Text, View } from '@tarojs/components'
import { AvatarMark, BottomNav } from '../../components'
import { listProducts } from '../../services/c-trade'
import Taro from '../../services/taro'
import type { Product } from '../../types'
import './index.scss'

const categories=['全部','电子产品','宿舍必备','零食','书籍','学习']
const statusLabels:Record<Product['status'],string>={selling:'出售中',reserved:'已预定',sold:'已售出'}

function MarketPage(){
  const[category,setCategory]=useState('全部')
  const[status,setStatus]=useState<Product['status']>('selling')
  const[keyword,setKeyword]=useState('')
  const[items,setItems]=useState<Product[]>([])
  const[total,setTotal]=useState(0)
  const filtered=useMemo(()=>category==='全部'?items:items.filter(item=>item.category===category),[category,items])

  const load=async(nextStatus=status,nextKeyword=keyword)=>{try{const result=await listProducts(1,nextKeyword.trim(),nextStatus);setItems(result.items);setTotal(result.total)}catch(error){void Taro.showToast({title:error instanceof Error?error.message:'商品加载失败',icon:'none'})}}
  useEffect(()=>{void load(status,'')},[status])

  return <View className='page market-page'>
    <View className='market-hero'>
      <Text className='market-hero__title'>校园二手</Text><Text className='market-hero__subtitle'>真实商品、校内沟通</Text>
      <View className='cc-search'><Input value={keyword} placeholder='搜索标题和描述' onInput={event=>setKeyword(String(event.detail.value))} onConfirm={()=>void load()}/><Button className='cc-button cc-button--small' onClick={()=>void load()}>搜索</Button></View>
    </View>
    <View className='feed-tabs'>{(['selling','reserved','sold'] as Product['status'][]).map(value=><Text key={value} className={status===value?'feed-tabs__active':''} onClick={()=>{setKeyword('');setStatus(value)}}>{statusLabels[value]}</Text>)}</View>
    <Text className='market-result-count'>共 {total} 件</Text>
    <ScrollView className='category-bar' scrollX enhanced showScrollbar={false}><View className='category-bar__inner'>{categories.map(item=><View key={item} className={`category-bar__item ${category===item?'category-bar__item--active':''}`} onClick={()=>setCategory(item)}>{item}</View>)}</View></ScrollView>
    <View className='product-grid'>{filtered.map(item=><View key={item.id} className='product-card card' onClick={()=>Taro.navigateTo({url:`/pages/product-detail/index?id=${item.id}`})}>
      {item.images[0]?<Image src={item.images[0]} mode='aspectFill'/>:<View className='cc-placeholder product-card__cover'>暂无图片</View>}
      <View className='product-card__body'><View className='between'><Text className='product-card__title'>{item.title}</Text><Text className={`cc-tag status-${item.status}`}>{statusLabels[item.status]}</Text></View>
        <View className='product-card__price-row'><Text className='product-card__price'>¥{item.price}</Text>{item.originalPrice&&<Text className='product-card__origin'>¥{item.originalPrice}</Text>}</View>
        <View className='between product-card__seller'><View className='row'><AvatarMark size='small' label={item.sellerName} src={item.sellerAvatar}/><Text>{item.sellerName}</Text></View><Text>{item.campus}</Text></View>
      </View></View>)}</View>
    <Button className='cc-button cc-button--primary publish-goods' onClick={()=>Taro.navigateTo({url:'/pages/publish/index?type=product'})}>发布二手</Button>
    <BottomNav active='market'/>
  </View>
}

export default withFeaturePage(MarketPage)
