import React from 'react'
import { Button, Input, Text, Textarea, View } from '@tarojs/components'
import { ImageTile, PageHeader, PageShell, SegmentedTabs } from '../../components'
import Taro from '../../services/taro'
import './index.scss'

export default function PublishPage() {
  const params = Taro.getCurrentInstance().router?.params || {}
  const isProduct = params.type === 'product'

  return (
    <PageShell active='publish' refreshTitle='发布页已刷新'>
      <PageHeader title={isProduct ? '发布二手' : '发布帖子'} right={<Text onClick={() => Taro.showToast({ title: '草稿已保存', icon: 'success' })}>草稿</Text>} />
      <SegmentedTabs
        active={isProduct ? 'product' : 'post'}
        items={[
          { key: 'post', label: '帖子' },
          { key: 'product', label: '二手' }
        ]}
        onChange={(key) => Taro.redirectTo({ url: `/pages/publish/index${key === 'product' ? '?type=product' : ''}` })}
      />
      <View className='form-card'>
        <View className='form-row'>
          <Text className='form-row__label'>标题</Text>
          <Input placeholder={isProduct ? '例如：九成新自行车，毕业急出' : '写一个想分享给同学的话题'} />
        </View>
        {isProduct && (
          <View className='form-row'>
            <Text className='form-row__label'>价格</Text>
            <Input type='number' placeholder='¥ 例如 150' />
          </View>
        )}
        <View className='form-row'>
          <Text className='form-row__label'>内容</Text>
          <Textarea placeholder={isProduct ? '描述成色、交易地点和注意事项' : '分享校园生活、求助、拼饭、学习资料……'} />
        </View>
        <View className='publish-images-live'>
          <ImageTile tone={0} label='+' />
          <ImageTile tone={1} label='+' />
          <ImageTile tone={2} label='+' />
        </View>
      </View>
      <Button className='primary-btn publish-live-btn' onClick={() => Taro.showToast({ title: isProduct ? '二手已发布' : '帖子已发布', icon: 'success' })}>
        {isProduct ? '发布二手' : '发布帖子'}
      </Button>
    </PageShell>
  )
}
