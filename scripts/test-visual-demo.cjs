const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const root = path.resolve(__dirname, '..')
function load(relative, dependencies = {}) {
  const filename = path.join(root, 'apps/miniapp/src', relative)
  const compiled = ts.transpileModule(fs.readFileSync(filename,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS,esModuleInterop:true}}).outputText
  const unit = new Module(filename,module)
  unit.require = id => { assert.ok(id in dependencies, `Network or unknown dependency: ${id}`); return dependencies[id] }
  unit._compile(compiled,filename)
  return unit.exports
}
async function main() {
  const data=load('demo/data.ts')
  const policy=load('services/feature-policy-model.ts')
  const api=load('demo/api.ts',{'./data':data,'../services/feature-policy-model':policy})
  const {demoRequest:r}=api
  assert.equal(data.demoUsers.length,6)
  assert.equal(new Set(data.demoUsers.map(x=>x.avatar)).size,6)
  assert.equal(new Set(data.demoUsers.map(x=>x.cover)).size,6)
  const photos=JSON.parse(fs.readFileSync(path.join(root,'apps/miniapp/src/demo/media-sources.json'),'utf8'))
  const imagePaths=new Set(photos.map(p=>`/demo-media/${p.id}.jpg`))
  assert.equal(photos.length,17)
  for(const user of data.demoUsers) { assert.ok(user.nickname.endsWith('·演示'));assert.ok(imagePaths.has(user.avatar));assert.ok(imagePaths.has(user.cover)) }
  assert.ok(data.demoPosts.some(p=>p.images.length===0));assert.ok(data.demoPosts.some(p=>p.images.length>1))
  const first=r('/community/posts?page=1&pageSize=20');const second=r('/community/posts?page=2&pageSize=20')
  assert.equal(first.total,24);assert.equal(first.items.length,20);assert.equal(second.items.length,4)
  assert.equal(new Set([...first.items,...second.items].map(p=>p.id)).size,24)
  for(const post of data.demoPosts) {
    const user=r(`/community/users/${post.authorId}/profile`).profile
    assert.equal(post.avatar,user.avatar);assert.equal(post.authorName,user.nickname)
    assert.ok(r(`/community/users/${post.authorId}/posts`).items.every(p=>p.authorId===post.authorId))
    for(const image of post.images) assert.ok(imagePaths.has(image))
  }
  for(const product of data.demoProducts) {
    assert.equal(product.seller.avatar,data.demoUser(product.sellerId).avatar)
    assert.ok(product.price>0);assert.ok(Number.isInteger(product.price))
    for(const image of product.images)assert.ok(imagePaths.has(image.url))
  }
  const p=data.demoPosts[0], key=`/community/posts/${p.id}`
  assert.equal(r(key+'/like','PUT').likedCount,p.likedCount+1)
  assert.equal(r(key+'/like','PUT').likedCount,p.likedCount+1)
  assert.equal(r(key+'/like','DELETE').likedCount,p.likedCount)
  assert.equal(r('/community/searches','POST',{keyword:'',filters:{contentType:'text'}}).items.every(p=>p.images.length===0),true)
  assert.equal(r('/trade/products?category=数码').items.every(p=>p.category==='数码'),true)
  assert.deepEqual(r('/trade/products?category=%E6%95%B0%E7%A0%81'),r('/trade/products?category=数码'))
  const affordable = r('/trade/searches','POST',{keyword:'',filters:{minPrice:1000,maxPrice:5000,sort:'price_asc'}}).items
  assert.ok(affordable.length > 0)
  assert.ok(affordable.every(p=>p.price >= 1000 && p.price <= 5000))
  assert.deepEqual(affordable.map(p=>p.price),affordable.map(p=>p.price).sort((a,b)=>a-b))
  assert.throws(()=>r('/auth/me','PATCH',{nickname:'Overwrite'}),/模拟/)
  assert.throws(()=>r('/delivery/payments','POST'),/模拟/)
  assert.throws(()=>r('/not-implemented'),/模拟/)
  assert.throws(()=>api.demoConversation(data.DEMO_SELF),/自己/)
  assert.throws(()=>api.demoConversation(7),/不存在/)
  const storage=new Map([['domainA.token','PRODUCTION_TOKEN_UNTOUCHED']])
  let version='develop', nativeCalls=0
  const native={ getAccountInfoSync:()=>({miniProgram:{envVersion:version}}),
    request:()=>{nativeCalls++;throw Error('Native network must not run')},login:()=>{nativeCalls++;throw Error('Native login must not run')},
    getStorageSync:key=>storage.get(key),setStorageSync:(key,value)=>storage.set(key,value),removeStorageSync:key=>storage.delete(key),
    showModal:async()=>({confirm:true}), navigateTo:async()=>{nativeCalls++} }
  const runtime=load('demo/runtime.ts',{'@tarojs/taro':native})
  const resource=load('demo/resource-url.ts')
  assert.equal(resource.resolveApiResourceUrl('', ''), '', 'Empty avatar during chat loading must render a fallback')
  assert.equal(resource.resolveApiResourceUrl('/demo-media/ocean.jpg', ''), '/demo-media/ocean.jpg')
  assert.throws(()=>resource.resolveApiResourceUrl('https://example.com/photo.jpg', ''), /阻止/)
  const facade=load('demo/taro.ts',{'@tarojs/taro':native,'./api':api,'./runtime':runtime}).default
  assert.equal((await facade.login()).code,'visual-demo-local-code')
  facade.setStorageSync('domainA.token','demo')
  assert.equal(facade.getStorageSync('domainA.token'),'demo')
  facade.removeStorageSync('domainA.token')
  assert.equal(storage.get('domainA.token'),'PRODUCTION_TOKEN_UNTOUCHED')
  assert.equal((await facade.request({url:'https://visual-demo.invalid/api/v1/auth/me'})).data.data.id,data.DEMO_SELF)
  await assert.rejects(()=>facade.request({url:'https://zhanduiwang.top/api/v1/auth/me'}),/阻止/)
  await assert.rejects(()=>facade.requestPayment({}),/不支持付款/)
  const im=load('demo/im.ts',{'./data':data,'./api':api,'./runtime':runtime})
  for(const user of data.demoUsers.slice(1)) {
    const ctx=await im.contactUser(String(user.id));assert.equal(ctx.peer.avatar,user.avatar)
    assert.deepEqual(await im.getIMConversation(ctx.relationId),ctx)
    assert.equal((await im.loadTencentMessages(ctx)).length,3)
    await im.sendTencentText(ctx,'本地测试')
    assert.equal((await im.loadTencentMessages(ctx)).length,4)
    await assert.rejects(()=>im.sendTencentText({...ctx,peerUserId:'campus_7'},'bad'),/不匹配/)
  }
  await assert.rejects(()=>im.ensureTencentIMReady(),/不连接/)
  version='release'
  await assert.rejects(()=>facade.login(),/不能用于正式版本/)
  await assert.rejects(()=>facade.request({url:'https://visual-demo.invalid/api/v1/auth/me'}),/不能用于正式版本/)
  assert.throws(()=>facade.getStorageSync('domainA.token'),/不能用于正式版本/)
  await assert.rejects(()=>im.contactUser('910002'),/不能用于正式版本/)
  assert.equal(nativeCalls,0)
  console.log('PASS: 6 complete profiles; 24 posts; 18 products; 17 licensed photos; paging, relations, local chat, production isolation, release refusal')
}
main().catch(error=>{console.error(error);process.exitCode=1})
