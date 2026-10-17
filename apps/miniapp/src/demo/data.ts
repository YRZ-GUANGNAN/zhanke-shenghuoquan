// Fictional, labelled fixtures. Never seeded into a backend or Tencent IM.
export const DEMO_SELF = 910001
export const photo = (name: string) => `/demo-media/${name}.jpg`
const seeds = [
  ['林间同学', 'cat', 'sunset'], ['小凛今天补番了吗', 'dog', 'ocean'],
  ['柚子是声优控', 'coffee', 'reading'], ['小鹿等安可', 'flowers', 'lightsea'],
  ['耳机里的星光', 'camera', 'concert'], ['慢慢读书', 'books', 'library'],
]
export const demoUsers = seeds.map(([name, avatar, cover], i) => ({
  id: DEMO_SELF + i, nickname: `${name}·演示`, avatar: photo(avatar), cover: photo(cover),
  school: '', campus: '', role: 'user',
}))
export function demoUser(id: number | string) {
  const user = demoUsers.find(value => value.id === Number(id))
  if (!user) throw new Error('该演示用户不存在')
  return user
}
const content: Array<[number, string, string, string[], string]> = [
  [1, '像动画片尾一样的海边', '最近想找一部温柔的日常番。把收藏的海边摄影换成背景，打开主页也能放松一下。', ['ocean'], '二次元日常'],
  [3, '灯海亮起来的那一刻', '翻到这张现场摄影，隔着照片也能感受到大合唱的气氛。下次想认真听完安可再离场。', ['lightsea'], '追星日常'],
  [0, '谁能拒绝认真看镜头的小猫', '今天的治愈份额已到账。', ['cat'], '可爱瞬间'],
  [2, '听到前奏就想起片尾的画面', '循环动画原声带的一天。整理书桌的时候听纯音乐，不知不觉就收拾好了。', ['reading', 'headphones'], '动画音乐'],
  [4, '歌单从日落放到夜晚', '喜欢现场的灯光，也喜欢回到宿舍后戴耳机重听的安静时刻。', ['concert', 'headphones'], '音乐分享'],
  [5, '给书桌留一个安静的角落', '重新整理桌面，书、台灯和一杯水就够了。', ['reading', 'books'], '学习日常'],
  [1, '求推荐一部轻松的日常番', '这周想看不用烧脑、人物相处很舒服的故事。老番也欢迎，先谢谢大家！', [], '番剧交流'],
  [3, '去现场之前的小清单', '充电、耳塞、返程路线提前看好。应援不用太复杂，照顾好自己最重要。', ['lightsticks'], '追星日常'],
  [2, '给补番时间留一点仪式感', '做完作业再看一集，片尾曲也不跳过。', ['coffee', 'books'], '二次元'],
  [0, '傍晚的天空很值得抬头', '把喜欢的颜色留在今天的记录里。', ['sunset', 'ocean'], '日常记录'],
  [4, '你会把哪首歌一直放在收藏里', '不是最近最火的，是无论隔多久都想重新听一遍的那首。', [], '音乐分享'],
  [5, '在书架之间停留一个下午', '喜欢把读到的句子抄下来。这张图是收藏的图书馆摄影，不是校内实拍。', ['library'], '阅读记录'],
  [1, '补番间隙看一眼小狗', '暂停一下屏幕，起身走走。今天的小快乐是这只认真看人的狗狗。', ['dog'], '二次元日常'],
  [3, '喜欢这种一片星星的现场', '看舞台摄影总会停留很久。先好好学习，再攒一次有计划的出行。', ['lightsea', 'concert'], '追星日常'],
  [2, '想整理一份睡前动画原声歌单', '人声少一点，钢琴、弦乐都可以。大家有喜欢的作曲家吗？', [], '动画音乐'],
  [4, '耳机一戴，走路都有节奏了', '听歌是平凡日常里最容易得到的小快乐。', ['headphones'], '音乐分享'],
  [5, '分享一个整理笔记的小习惯', '每周只留一页总结：这周学会了什么，下周还想弄明白什么。', [], '学习交流'],
  [0, '周末想骑车兜风', '慢慢骑，不赶时间。', ['bicycle'], '日常记录'],
  [1, '这周的补番进度暂停一下', '先把作业写完，喜欢的故事值得留一整段时间慢慢看。', ['books'], '二次元日常'],
  [3, '散场之后还在脑内循环', '最喜欢的不是最响亮的瞬间，而是大家一起唱完最后一句的感觉。', [], '追星日常'],
  [2, '春天的颜色适合做主页背景', '喜欢清透一点的花和天空，日常的照片放在主页里就很舒服。', ['flowers'], '二次元日常'],
  [4, '最近收藏的现场摄影', '给忙碌的一周留一点期待。图片来自授权摄影，来源见模拟体验说明。', ['concert'], '音乐分享'],
  [5, '一杯咖啡和还没读完的书', '暂时关掉通知，留一小段时间给自己。', ['coffee', 'reading'], '阅读记录'],
  [0, '它在等一个出门散步的信号', '狗狗抬头看人的样子也很治愈。', ['dog'], '可爱瞬间'],
]
export const demoPosts = content.map(([authorIndex, title, text, media, topic], i) => {
  const author = demoUsers[authorIndex]
  return { id: 920001 + i, authorId: author.id, authorName: author.nickname, avatar: author.avatar,
    title, content: `${text}\n\n模拟内容，仅用于体验排版。`, images: media.map(photo), topic, tags: [topic],
    likedCount: [38, 12, 106, 0, 8, 27][i % 6], commentCount: 3, favoriteCount: i % 8,
    reviewStatus: 'approved', createdAt: new Date(Date.UTC(2026, 9, 17, 7) - i * 3600000).toISOString(),
  }
})
const goods: Array<[string, string, number, string]> = [
  ['头戴式耳机，听歌用', 'headphones', 9990, '数码'], ['闲置书籍整理', 'books', 3500, '书籍资料'],
  ['机械键盘，桌面搭配', 'keyboard', 12000, '数码'], ['复古阅读台灯', 'lamp', 6850, '生活用品'],
  ['入门摄影相机', 'camera', 123456, '数码'], ['校内代步自行车', 'bicycle', 15000, '代步'],
  ['闲置练习册，未写字', '', 800, '书籍资料'], ['小巧咖啡杯', 'coffee', 1990, '生活用品'],
  ['闲置小卡收纳册，空册', '', 2800, '生活用品'],
]
export const demoProducts = Array.from({ length: 18 }, (_, i) => {
  const [title, media, price, category] = goods[i % goods.length]
  const seller = demoUsers[(i + 1) % demoUsers.length]
  return { id: 930001 + i, sellerId: seller.id, seller, title, price, originalPrice: price + 2000,
    description: '模拟商品，仅用于查看图文、价格和联系卖家的页面效果，不是真实在售商品。',
    images: media ? [{ url: photo(media) }] : [], category, condition: '有使用痕迹', campus: '',
    status: 'selling', reviewStatus: 'approved', favoriteCount: i % 9,
    createdAt: new Date(Date.UTC(2026, 9, 17, 6) - i * 7200000).toISOString() }
})
export const demoComments = (postId: number) => [1, 2, 3].map((n) => {
  const author = demoUsers[n]
  return { id: postId * 10 + n, postId, userId: author.id, author, authorName: author.nickname,
    avatar: author.avatar, content: ['这个角落看起来很舒服。', '这种记录日常的感觉很好。', '记下了，下次也试试。'][n - 1],
    parentCommentId: 0, replyToCommentId: 0, replyToUserId: 0, liked: false, likedCount: 2 * n,
    replyCount: n === 1 ? 2 : 0, reviewStatus: 'approved', createdAt: '2026-10-17T06:30:00Z' }
})
