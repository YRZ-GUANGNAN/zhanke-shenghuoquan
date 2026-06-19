const http = require('node:http')
const { URL } = require('node:url')

const port = Number(process.env.PORT || 8080)

const users = {
  senior: {
    id: 1,
    nickname: '热心学长',
    avatar: '',
    school: '海大',
    campus: '海大'
  },
  life: {
    id: 2,
    nickname: '校园生活组',
    avatar: '',
    school: '海景校区',
    campus: '海景'
  },
  market: {
    id: 3,
    nickname: '校友集市',
    avatar: '',
    school: '海大',
    campus: '海大'
  }
}

const posts = [
  {
    id: 1,
    userId: users.senior.id,
    user: users.senior,
    topic: '二手避坑',
    title: '二手书不要论斤卖啦',
    content: '教材、考研资料、四六级资料可以挂到二手区，支持私聊和校内自提，卖家要记得标清版本和新旧程度。',
    images: [],
    status: 'approved',
    pinned: true,
    likedCount: 128,
    commentCount: 36,
    createdAt: new Date(Date.now() - 10 * 60 * 1000).toISOString()
  },
  {
    id: 2,
    userId: users.life.id,
    user: users.life,
    topic: '校园问答',
    title: '新生常用工具导航整理',
    content: '把常用的小程序、服务号、地图、失物招领入口放到工具页，大家可以直接跳转，不再到处翻群公告。',
    images: [],
    status: 'approved',
    pinned: false,
    likedCount: 86,
    commentCount: 14,
    createdAt: new Date(Date.now() - 32 * 60 * 1000).toISOString()
  },
  {
    id: 3,
    userId: users.market.id,
    user: users.market,
    topic: '交易提醒',
    title: '面交建议选择教学楼或宿舍门口',
    content: '二手交易建议走平台私聊留痕，贵重物品现场验货，完成交易后及时改为已售。',
    images: [],
    status: 'approved',
    pinned: false,
    likedCount: 41,
    commentCount: 8,
    createdAt: new Date(Date.now() - 60 * 60 * 1000).toISOString()
  }
]

const products = [
  {
    id: 1,
    sellerId: users.senior.id,
    seller: users.senior,
    title: '二手教材一组，市场营销学第六版',
    description: '教材资料流转，校内自提。',
    category: '书籍',
    price: 1000,
    originalPrice: 4280,
    condition: '八成新',
    campus: '海大',
    images: [],
    status: 'on_sale',
    createdAt: new Date().toISOString()
  },
  {
    id: 2,
    sellerId: 4,
    seller: { id: 4, nickname: '未登录用户', avatar: '', school: '海景', campus: '海景' },
    title: '全新白颜料，海景自提',
    description: '全新闲置。',
    category: '学习',
    price: 50,
    originalPrice: 0,
    condition: '全新',
    campus: '海景',
    images: [],
    status: 'on_sale',
    createdAt: new Date(Date.now() - 5 * 60 * 1000).toISOString()
  },
  {
    id: 3,
    sellerId: 5,
    seller: { id: 5, nickname: '阿泽', avatar: '', school: '南区', campus: '南区' },
    title: '人体工学腕托，宿舍楼下自提',
    description: '九成新。',
    category: '宿舍必备',
    price: 1200,
    originalPrice: 0,
    condition: '九成新',
    campus: '南区',
    images: [],
    status: 'on_sale',
    createdAt: new Date(Date.now() - 18 * 60 * 1000).toISOString()
  },
  {
    id: 4,
    sellerId: 6,
    seller: { id: 6, nickname: '小陈', avatar: '', school: '东区', campus: '东区' },
    title: '桌面收纳盒，透明三层',
    description: '轻微使用痕迹。',
    category: '宿舍必备',
    price: 800,
    originalPrice: 0,
    condition: '轻微使用痕迹',
    campus: '东区',
    images: [],
    status: 'on_sale',
    createdAt: new Date(Date.now() - 40 * 60 * 1000).toISOString()
  }
]

const toolLinks = [
  { id: 1, title: '特价影票', subtitle: '校园福利票务', iconText: '票', color: '#ff8a3d', sort: 10, enabled: true },
  { id: 2, title: '学生兼职', subtitle: '校内外岗位', iconText: '兼', color: '#ef6a6a', sort: 20, enabled: true },
  { id: 3, title: '生活服务', subtitle: '维修跑腿打印', iconText: '服', color: '#ffb545', sort: 30, enabled: true },
  { id: 4, title: '校园群聊', subtitle: '社团年级群', iconText: '群', color: '#34a7ff', sort: 40, enabled: true },
  { id: 5, title: '二手书', subtitle: '教材资料流转', iconText: '书', color: '#5db8ff', sort: 50, enabled: true },
  { id: 6, title: '校园地图', subtitle: '楼宇导航', iconText: '图', color: '#4fd1c5', sort: 60, enabled: true },
  { id: 7, title: '学习专区', subtitle: '资料与经验帖', iconText: '学', color: '#f9c74f', sort: 70, enabled: true },
  { id: 8, title: '帖子问答', subtitle: '互助答疑', iconText: '?', color: '#48d597', sort: 80, enabled: true }
]

function sendJson(res, statusCode, body) {
  res.writeHead(statusCode, {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'content-type, authorization',
    'Access-Control-Allow-Methods': 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
    'Content-Type': 'application/json; charset=utf-8'
  })
  res.end(JSON.stringify(body))
}

function ok(res, data) {
  sendJson(res, 200, { code: 0, message: 'ok', data })
}

const server = http.createServer((req, res) => {
  if (req.method === 'OPTIONS') {
    sendJson(res, 204, {})
    return
  }

  const url = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`)
  const path = url.pathname
  console.log(`${new Date().toISOString()} ${req.method} ${path}`)

  if (path === '/healthz') {
    ok(res, { status: 'ok' })
    return
  }

  if (path === '/api/v1/community/posts') {
    ok(res, posts)
    return
  }

  if (path === '/api/v1/market/products') {
    ok(res, products)
    return
  }

  const productMatch = path.match(/^\/api\/v1\/market\/products\/(\d+)$/)
  if (productMatch) {
    const product = products.find((item) => item.id === Number(productMatch[1]))
    if (product) ok(res, product)
    else sendJson(res, 404, { code: 404, message: 'product not found' })
    return
  }

  if (path === '/api/v1/tools/links') {
    ok(res, toolLinks)
    return
  }

  sendJson(res, 404, { code: 404, message: 'not found' })
})

server.listen(port, '0.0.0.0', () => {
  console.log(`campus dev api listening on http://localhost:${port}`)
})
