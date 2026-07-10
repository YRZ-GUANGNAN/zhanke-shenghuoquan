import React, { useState } from 'react'
import { Input, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type CommentTone = 'sky' | 'peach' | 'green' | 'mint'

type CommentItem = {
  id: string
  avatar: string
  author: string
  content: string
  time: string
  likes: number
  tone: CommentTone
}

type PostCopy = {
  authorId: string
  author: string
  avatar: string
  authorMeta: string
  hero: string
  title: string
  body: string
  tags: string[]
  meta: string
  likes: number
}

const postCopies: Record<string, PostCopy> = {
  'milk-tea': {
    authorId: 'milk-tea',
    author: '奶茶脑袋',
    avatar: '奶',
    authorMeta: '湛江科技学院 · 大三',
    hero: '图 · 用户上传（奶茶实拍）',
    title: '三食堂新出的芋泥波波真的绝，一天一杯根本停不下来',
    body: '中杯 8 块，加一份波波才 1 块，芋泥是真材实料那种，甜度选三分刚刚好。下午四点后人少不用排队，学生证还能再减 5 毛。',
    tags: ['#奶茶推荐', '#三食堂', '#湛科日常'],
    meta: '今天 12:32 发布 · 湛江科技学院',
    likes: 128
  },
  sea: {
    authorId: 'deer',
    author: '小鹿乱撞',
    avatar: '鹿',
    authorMeta: '湛江科技学院 · 大二',
    hero: '图 · 用户上传（金沙湾海景）',
    title: '有没有周末去金沙湾的搭子？拍照吹海风',
    body: '计划周末下午四点从学校出发，想拍日落也想顺路吃饭。有相机的同学欢迎一起，集合地点可以私信再定。',
    tags: ['#搭子', '#金沙湾', '#湛科日常'],
    meta: '今天 10:18 发布 · 湛江科技学院',
    likes: 87
  },
  library: {
    authorId: 'cabbage',
    author: '卷心菜',
    avatar: '卷',
    authorMeta: '湛江科技学院 · 大三',
    hero: '图 · 用户上传（图书馆座位）',
    title: '期末周图书馆占座攻略｜亲测有效的三个时间点',
    body: '早上开馆前、午饭交接和晚课结束后是三个相对容易找到位置的时间点。三楼靠窗区安静，但记得不要长期占座。',
    tags: ['#图书馆', '#期末周', '#学习攻略'],
    meta: '昨天 18:40 发布 · 湛江科技学院',
    likes: 214
  },
  express: {
    authorId: 'ali',
    author: '阿黎',
    avatar: '黎',
    authorMeta: '湛江科技学院 · 大二',
    hero: '图 · 用户上传（北门快递点）',
    title: '求助：明天上午有人能帮拿北门快递吗，有偿',
    body: '上午有课走不开，快递不重，送到宿舍楼下即可。有空的同学可以私信我时间和跑腿费。',
    tags: ['#求助', '#快递', '#有偿'],
    meta: '今天 11:02 发布 · 湛江科技学院',
    likes: 12
  },
  'part-time': {
    authorId: 'yeye',
    author: '椰椰',
    avatar: '椰',
    authorMeta: '湛江科技学院 · 大四',
    hero: '图 · 用户上传（咖啡店招聘）',
    title: '周末兼职招募，咖啡店需要两名同学帮忙',
    body: '主要负责简单打包和引导，周六日任选一天，具体时段和报酬请私信确认。',
    tags: ['#兼职', '#周末', '#校园招聘'],
    meta: '周二 16:10 发布 · 湛江科技学院',
    likes: 68
  },
  sunset: {
    authorId: 'deer',
    author: '小鹿乱撞',
    avatar: '鹿',
    authorMeta: '湛江科技学院 · 大二',
    hero: '图 · 用户上传（操场晚霞）',
    title: '今天的晚霞，操场随手拍的，太治愈了',
    body: '下课路过操场刚好遇到整片粉蓝色天空，随手拍了几张。希望大家今天也有一个轻松的夜晚。',
    tags: ['#晚霞', '#操场', '#校园日常'],
    meta: '今天 19:02 发布 · 湛江科技学院',
    likes: 156
  },
  'dorm-makeover': {
    authorId: 'pomelo',
    author: '柚子茶',
    avatar: '柚',
    authorMeta: '湛江科技学院 · 大三',
    hero: '图 · 用户上传（宿舍改造）',
    title: '宿舍改造第二弹｜300 块把上床下桌改成 ins 风',
    body: '灯带、桌垫和收纳架都是平价款，安装不打孔，退宿时也方便恢复。清单整理好后放在评论区。',
    tags: ['#宿舍改造', '#收纳', '#好物分享'],
    meta: '6月30日发布 · 湛江科技学院',
    likes: 342
  },
  badminton: {
    authorId: 'current-user',
    author: '阿泽',
    avatar: '泽',
    authorMeta: '湛江科技学院 · 大三',
    hero: '图 · 用户上传（体育馆）',
    title: '周末羽毛球局召集，体育馆 3 号场还差两人',
    body: '周六晚上七点到九点，水平不限，球和场地已经准备好。想参加的同学直接私信即可。',
    tags: ['#羽毛球', '#运动搭子', '#周末'],
    meta: '6月29日发布 · 湛江科技学院',
    likes: 31
  },
  'aze-library': {
    authorId: 'current-user',
    author: '阿泽',
    avatar: '泽',
    authorMeta: '湛江科技学院 · 大二',
    hero: '图 · 用户上传（图书馆安静角落）',
    title: '期末周图书馆安静角落分享',
    body: '三楼东侧靠窗的位置下午光线很好，人也相对少。复习时记得带一件薄外套，空调温度会有点低。',
    tags: ['#图书馆', '#期末周', '#校园日常'],
    meta: '今天 09:26 发布 · 湛江科技学院',
    likes: 96
  },
  'aze-sunset': {
    authorId: 'current-user',
    author: '阿泽',
    avatar: '泽',
    authorMeta: '湛江科技学院 · 大二',
    hero: '图 · 用户上传（蓝调晚霞）',
    title: '下课路上遇到的蓝调晚霞',
    body: '从教学楼回宿舍时刚好遇到蓝调时刻，远处的云层带着一点粉色。随手记录下来，也算给今天留个纪念。',
    tags: ['#晚霞', '#随手拍', '#湛科日常'],
    meta: '昨天 19:12 发布 · 湛江科技学院',
    likes: 73
  },
  'aze-food': {
    authorId: 'current-user',
    author: '阿泽',
    avatar: '泽',
    authorMeta: '湛江科技学院 · 大二',
    hero: '图 · 用户上传（北门砂锅粉）',
    title: '北门新开的砂锅粉，学生价很实在',
    body: '基础套餐分量够，汤底偏清爽，学生证还能送一份配菜。中午高峰人比较多，建议十一点半前去。',
    tags: ['#校园美食', '#北门', '#学生价'],
    meta: '6月30日发布 · 湛江科技学院',
    likes: 52
  },
  'milk-tea-review': {
    authorId: 'milk-tea',
    author: '奶茶脑袋',
    avatar: '奶',
    authorMeta: '湛江科技学院 · 大三',
    hero: '图 · 用户上传（奶茶测评）',
    title: '大学城奶茶测评｜十家喝下来这三家最能打',
    body: '从茶底、甜度到小料口感都记了分，前三名分别适合清爽党、奶香党和小料党。完整点单建议放在评论区。',
    tags: ['#奶茶测评', '#大学城', '#点单攻略'],
    meta: '昨天 16:42 发布 · 湛江科技学院',
    likes: 256
  },
  'milk-tea-menu': {
    authorId: 'milk-tea',
    author: '奶茶脑袋',
    avatar: '奶',
    authorMeta: '湛江科技学院 · 大三',
    hero: '图 · 用户上传（隐藏菜单）',
    title: '隐藏喝法：这家的四季春加波波椰果绝配',
    body: '四季春三分糖、少冰，再加波波和椰果，茶味和口感都很平衡。怕甜的同学可以直接点无糖。',
    tags: ['#隐藏菜单', '#奶茶推荐', '#四季春'],
    meta: '6月29日发布 · 湛江科技学院',
    likes: 89
  },
  'north-graduation': {
    authorId: 'north',
    author: '向北',
    avatar: '向',
    authorMeta: '湛江科技学院 · 大四',
    hero: '图 · 用户上传（毕业季校园骑行）',
    title: '毕业季最后一次绕学校骑行',
    body: '从宿舍出发绕教学楼、图书馆和操场走了一圈，把熟悉的路都再看一遍。自行车也准备转给下一位同学。',
    tags: ['#毕业季', '#校园骑行', '#告别'],
    meta: '6月28日发布 · 湛江科技学院',
    likes: 64
  }
}

const comments: CommentItem[] = [
  {
    id: 'north',
    avatar: '向',
    author: '向北',
    content: '昨天刚喝，波波确实弹，就是下午去要排一会儿',
    time: '1 小时前',
    likes: 6,
    tone: 'sky'
  },
  {
    id: 'pomelo',
    avatar: '柚',
    author: '柚子茶',
    content: '甜度三分党+1，五分有点腻',
    time: '58 分钟前',
    likes: 3,
    tone: 'peach'
  },
  {
    id: 'morning',
    avatar: '早',
    author: '早八战士',
    content: '学生证减 5 毛是真的吗？？明天就去',
    time: '40 分钟前',
    likes: 2,
    tone: 'green'
  },
  {
    id: 'deer',
    avatar: '鹿',
    author: '小鹿乱撞',
    content: '蹲一个代喝，今天出不了门，有人帮带吗（跑腿费好说）',
    time: '12 分钟前',
    likes: 1,
    tone: 'mint'
  }
]

const missingPostCopy: PostCopy = {
  authorId: 'current-user',
  author: '内容待接入',
  avatar: '?',
  authorMeta: '静态壳提示',
  hero: '图 · 未找到对应帖子',
  title: '未找到对应的静态帖子内容',
  body: '该链接没有匹配到已登记的静态帖子，请返回上一页重新选择。',
  tags: [],
  meta: '静态内容待接入',
  likes: 0
}

export default function PostDetailPage() {
  const requestedId = Taro.getCurrentInstance().router?.params?.id || 'milk-tea'
  const postFound = Boolean(postCopies[requestedId])
  const post = postCopies[requestedId] || missingPostCopy
  const [followed, setFollowed] = useState(false)
  const [liked, setLiked] = useState(false)
  const [favorited, setFavorited] = useState(false)
  const [draft, setDraft] = useState('')

  const likeCount = post.likes + (liked ? 1 : 0)
  const favoriteCount = 23 + (favorited ? 1 : 0)

  const goBack = () => {
    void Taro.navigateBack().catch(() => {
      void Taro.redirectTo({ url: '/pages/home/index' })
    })
  }

  const showToast = (title: string) => {
    void Taro.showToast({ title, icon: 'none', duration: 1200 })
  }

  const openTagSearch = (tag: string) => {
    const keyword = tag.replace(/^#/, '')
    void Taro.navigateTo({ url: `/pages/search/index?keyword=${encodeURIComponent(keyword)}` })
  }

  const submitComment = () => {
    if (!draft.trim()) {
      showToast('先写点什么吧')
      return
    }
    setDraft('')
    showToast('评论已记录为本地预览')
  }

  return (
    <View className='post-detail-page'>
      <View className='post-detail-nav'>
        <View className='post-detail-nav__back' onClick={goBack} />
        <View className='post-detail-author' onClick={() => postFound ? void Taro.navigateTo({ url: `/pages/user-profile/index?userId=${encodeURIComponent(post.authorId)}&source=post` }) : showToast('该帖子静态内容未登记')}>
          <View className='post-detail-author__avatar'>
            <Text>{post.avatar}</Text>
          </View>
          <View className='post-detail-author__main'>
            <Text className='post-detail-author__name'>{post.author}</Text>
            <Text className='post-detail-author__meta'>{post.authorMeta}</Text>
          </View>
        </View>
        <View
          className={`post-detail-follow ${followed ? 'post-detail-follow--active' : ''}`}
          onClick={() => setFollowed((value) => !value)}
        >
          <Text>{followed ? '已关注' : '关注'}</Text>
        </View>
      </View>

      <View className='post-detail-scroll'>
        <View className='post-detail-hero' onClick={() => showToast('图片预览下一页接入')}>
          <Text>{post.hero}</Text>
        </View>

        <View className='post-detail-content'>
          <Text className='post-detail-content__title'>{post.title}</Text>
          <Text className='post-detail-content__body'>{post.body}</Text>
          <View className='post-detail-tags'>
            {post.tags.map((tag) => (
              <View key={tag} className='post-detail-tag' onClick={() => openTagSearch(tag)}>
                <Text>{tag}</Text>
              </View>
            ))}
          </View>
          <Text className='post-detail-content__meta'>{post.meta}</Text>
        </View>

        <View className='post-detail-comments'>
          <Text className='post-detail-comments__title'>共 {postFound ? 12 : 0} 条评论</Text>
          {(postFound ? comments : []).map((item) => (
            <View key={item.id} className='post-detail-comment'>
              <View className={`post-detail-comment__avatar post-detail-comment__avatar--${item.tone}`}>
                <Text>{item.avatar}</Text>
              </View>
              <View className='post-detail-comment__main'>
                <Text className='post-detail-comment__author'>{item.author}</Text>
                <Text className='post-detail-comment__content'>{item.content}</Text>
                <View className='post-detail-comment__meta'>
                  <Text>{item.time}</Text>
                  <Text onClick={() => showToast('回复下一页接入')}>回复</Text>
                </View>
              </View>
              <View className='post-detail-comment__like' onClick={() => showToast('评论点赞本地预览')}>
                <Text>♡</Text>
                <Text>{item.likes}</Text>
              </View>
            </View>
          ))}
          <Text className='post-detail-comments__bottom'>— 到底了 —</Text>
        </View>
      </View>

      <View className='post-detail-actionbar'>
        <View className='post-detail-input'>
          <Input
            value={draft}
            placeholder='说点什么...'
            confirmType='send'
            onInput={(event) => setDraft(String(event.detail.value))}
            onConfirm={submitComment}
          />
        </View>
        <View className={`post-detail-action ${liked ? 'post-detail-action--active' : ''}`} onClick={() => setLiked((value) => !value)}>
          <Text className='post-detail-action__icon'>{liked ? '♥' : '♡'}</Text>
          <Text>{likeCount}</Text>
        </View>
        <View className={`post-detail-action ${favorited ? 'post-detail-action--active' : ''}`} onClick={() => setFavorited((value) => !value)}>
          <Text className='post-detail-action__icon'>{favorited ? '★' : '☆'}</Text>
          <Text>{favoriteCount}</Text>
        </View>
        <View className='post-detail-action' onClick={() => showToast('已在评论区')}>
          <Text className='post-detail-action__icon'>▱</Text>
          <Text>12</Text>
        </View>
      </View>
    </View>
  )
}
