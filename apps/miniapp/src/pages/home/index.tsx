import React, { useState } from 'react'
import { Input, Text, View } from '@tarojs/components'
import { ImageTile, PageShell, SectionTitle, SegmentedTabs } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

type HomeTab = 'follow' | 'discover' | 'tools'

const tabs = [
  { key: 'follow' as const, label: '关注' },
  { key: 'discover' as const, label: '发现' },
  { key: 'tools' as const, label: '工具' }
]

const posts = [
  ['三食堂新出的芋泥波波真的绝，一天一杯根本停不下', '奶茶脑袋', '128', '奶茶', 1],
  ['出一辆九成新自行车，毕业急出，150 拿走', '向北', '36', '自行车', 0],
  ['有没有周末去金沙湾的搭子？拍照吹海风', '小鹿乱撞', '87', '海边', 2],
  ['期末周图书馆占座攻略，亲测有效的三个时间点', '卷心菜', '214', '图书馆', 3],
  ['求助：明天上午有人能帮拿北门快递吗，有偿', '阿黎', '12', '快递', 4],
  ['招募羽毛球搭子，体育馆 3 号场还差两人', '球友会', '55', '招聘', 0]
]

const tools = ['校园地图', '课程表', '校园群聊', '二手教材', '兼职信息', '生活服务']

export default function HomePage() {
  const [active, setActive] = useState<HomeTab>('discover')

  return (
    <PageShell active='home' refreshTitle='首页已刷新'>
      <View className='home-top'>
        <View className='home-tabs-inline'>
          {tabs.map((tab) => (
            <Text key={tab.key} className={active === tab.key ? 'home-tabs-inline__active' : ''} onClick={() => setActive(tab.key)}>
              {tab.label}
            </Text>
          ))}
          <Text className='home-search-icon'>⌕</Text>
        </View>
        <View className='home-search'>
          <Text>搜索</Text>
          <Input disabled placeholder='帖子、二手、资料、工具' />
        </View>
      </View>

      {active === 'tools' ? (
        <View className='tool-grid'>
          {tools.map((item, index) => (
            <View key={item} className='tool-card' onClick={() => Taro.showToast({ title: item, icon: 'none' })}>
              <ImageTile tone={index} label={item.slice(0, 1)} />
              <Text>{item}</Text>
            </View>
          ))}
        </View>
      ) : (
        <>
          <SectionTitle title={active === 'follow' ? '关注动态' : '校园发现'} sub='同学正在聊' />
          <View className='masonry-feed'>
            {posts.map((post, index) => (
              <View
                key={post[0] as string}
                className='feed-card'
                onClick={() => Taro.navigateTo({ url: index === 1 ? '/pages/product-detail/index' : '/pages/post-detail/index' })}
              >
                <ImageTile tone={Number(post[4])} label={String(post[3])} />
                <View className='feed-card__body'>
                  <Text className='feed-card__title'>{post[0]}</Text>
                  <View className='feed-card__meta'>
                    <Text>{post[1]}</Text>
                    <Text>♡ {post[2]}</Text>
                  </View>
                </View>
              </View>
            ))}
          </View>
        </>
      )}
    </PageShell>
  )
}
