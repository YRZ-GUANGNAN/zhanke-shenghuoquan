import { useState } from 'react'
import { RootPortal, ScrollView, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import type Facade from '../services/taro'
import { useDidShow } from '@tarojs/taro'
import './indicator.scss'
declare const require: (path: string) => any
const native = Taro as unknown as typeof Facade
const media = require('./media-sources.json') as Array<{id:string;title:string;artist:string;license:string;source:string;licenseUrl:string}>
export default function DemoIndicator() {
  const [open,setOpen]=useState(false)
  useDidShow(()=>{void Taro.hideShareMenu({menus:['shareAppMessage','shareTimeline']}).catch(()=>undefined)})
  return <>
    <View className='visual-demo-badge' onClick={()=>setOpen(true)}><Text>模拟体验 · 说明</Text></View>
    {open && <RootPortal><View className='visual-demo-scrim' onClick={()=>setOpen(false)}>
      <View className='visual-demo-sheet' onClick={event=>event.stopPropagation()}>
        <View className='visual-demo-sheet__header'><Text>模拟数据与照片来源</Text><View onClick={()=>setOpen(false)}>关闭</View></View>
        <ScrollView scrollY className='visual-demo-sheet__body'>
          <Text className='visual-demo-sheet__intro'>本版本包含 6 位虚构用户（二次元、追星与日常兴趣）、24 条帖子、18 件商品，用于体验真实页面排版。图片为可复用摄影，没有 AI 生成图。没有真实在售商品；聊天、点赞、收藏仅在本次体验内生效，不写入生产数据。此包不可送审或发布。每张素材及其裁切版本沿用下列原作许可。</Text>
          {media.map(item=><View className='visual-demo-credit' key={item.id}>
            <Text>{item.title}</Text><Text>{item.artist} · {item.license}</Text>
            <Text>处理：缩小尺寸、JPEG 压缩，页面按比例裁切。</Text>
            <View onClick={()=>void native.setClipboardData({data:item.source+'\n'+item.licenseUrl})}>复制来源与许可链接</View>
          </View>)}
        </ScrollView>
      </View>
    </View></RootPortal>}
  </>
}
