const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')
const types = read('apps', 'miniapp', 'src', 'types.ts')
const trade = read('apps', 'miniapp', 'src', 'services', 'c-trade.ts')
const community = read('apps', 'miniapp', 'src', 'services', 'b-community.ts')
const detail = read('apps', 'miniapp', 'src', 'pages', 'secondhand-detail', 'index.tsx')
const detailStyle = read('apps', 'miniapp', 'src', 'pages', 'secondhand-detail', 'index.scss')
const comments = read('apps', 'miniapp', 'src', 'pages', 'secondhand-detail', 'product-comments.tsx')
const notifications = read('apps', 'miniapp', 'src', 'pages', 'notifications', 'index.tsx')

const requiredTypeFragments = [
  'export interface ProductComment',
  'reviewStatus: ProductCommentReviewStatus',
  'isSeller: boolean',
  'replies: ProductComment[]',
  "'product_commented'",
  "'product_comment_replied'",
  'productId?: string',
  'productCommentId?: string',
]

for (const fragment of requiredTypeFragments) {
  if (!types.includes(fragment)) {
    throw new Error(`second-hand comment type contract missing: ${fragment}`)
  }
}

const requiredClientFragments = [
  'export async function listProductComments(',
  'export async function createProductComment(',
  'export async function replyProductComment(',
  '/comments?page=${page}&pageSize=${pageSize}',
  '/comments/${encodeURIComponent(commentId)}/replies',
  "data:{content,clientRequestId}",
  "header:{'Idempotency-Key':clientRequestId}",
]

for (const fragment of requiredClientFragments) {
  if (!trade.includes(fragment)) {
    throw new Error(`second-hand comment client contract missing: ${fragment}`)
  }
}

if (!/clientRequestId=createClientRequestId\(\)/.test(trade)) {
  throw new Error('comment create and reply must default to createClientRequestId()')
}

for (const forbidden of ['data:{productId', 'data:{parentCommentId', 'data:{replyToUserId', 'data:{isSeller']) {
  if (trade.includes(forbidden)) {
    throw new Error(`comment request body must not trust client coordinate: ${forbidden}`)
  }
}

for (const type of ['product_commented', 'product_comment_replied']) {
  if (!community.includes(type)) {
    throw new Error(`notification normalization missing: ${type}`)
  }
}

for (const fragment of [
  'useSessionStore((state) => state.profile.id)',
  'currentUserId === product.sellerId',
  'changeProductStatus',
  '<ProductComments',
  'targetCommentId={targetCommentId}',
  "{favorited ? '★' : '☆'}",
]) {
  if (!detail.includes(fragment)) {
    throw new Error(`second-hand detail contract missing: ${fragment}`)
  }
}

if (/secondhand-detail-favorite__star::(?:before|after)/.test(detailStyle)) {
  throw new Error('favorite star must not be drawn with rotated pseudo-elements')
}

for (const fragment of [
  'type CommentLoadState',
  'type ReplyTarget',
  'pendingSubmission',
  'commentId: comment.id',
  'parentCommentId: comment.parentCommentId || comment.id',
  'created.parentCommentId || replyTarget.parentCommentId',
  'targetCommentId && !containsTarget(loadedItems)',
  '商品已售出，不能继续留言',
  'id={`product-comment-${item.id}`}',
]) {
  if (!comments.includes(fragment)) {
    throw new Error(`product comment UI contract missing: ${fragment}`)
  }
}

for (const fragment of [
  'parentCommentId:optionalID(item.parentCommentId)',
  'replyToUserId:optionalID(item.replyToUserId)',
  "value===0||value==='0'",
]) {
  if (!trade.includes(fragment)) {
    throw new Error(`product comment optional ID normalization missing: ${fragment}`)
  }
}

if (!notifications.includes("/pages/secondhand-detail/index?id=${encodeURIComponent(item.productId)}&commentId=${encodeURIComponent(item.productCommentId||'')}")) {
  throw new Error('product notification must navigate to its product and comment coordinate')
}

console.log('second-hand comments client contract: PASS')
