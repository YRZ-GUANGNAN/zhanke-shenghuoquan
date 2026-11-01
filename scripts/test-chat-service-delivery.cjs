// Actual new handlers/hooks with isolated transports. No login or production IO.
const assert = require('node:assert/strict')
const { loadSource, loadPage, hookRenderer, deferred, nodes, hasClass, text } = require('./test-delivery-stage-workbench.cjs')

async function run() {
  const renderer = hookRenderer()
  const gesture = loadSource('hooks/use-horizontal-navigation.ts', { '@tarojs/taro': { useDidShow: () => {}, useDidHide: () => {} } }, renderer).createHorizontalNavigation
  const point = (x,y,id=1) => ({ clientX:x,clientY:y,identifier:id })
  const event = (points,changed=[]) => ({ touches:points,changedTouches:changed })
  const g = gesture()
  g.start(event([point(220,40)]));g.move(event([point(100,44)]))
  assert.equal(g.canActivate(), false)
  assert.equal(g.end(event([], [point(100,44)])), 1)
  assert.equal(g.canActivate(), false, '滑动后的click不能触发相邻按钮')
  for (const [origin,moved,end] of [[point(10,10),point(110,10),point(110,10)], [point(200,20),point(190,100),point(70,100)], [point(200,20),point(170,21),point(170,21)]]) {
    const x = gesture();x.start(event([origin]));x.move(event([moved]));assert.equal(x.end(event([],[end])),0)
  }
  const multi = gesture();multi.start(event([point(200,20)]));multi.move(event([point(100,20),point(110,25,2)]));assert.equal(multi.end(event([],[point(100,20)])),0)
  const cancelled = gesture();cancelled.start(event([point(220,40)]));cancelled.move(event([point(100,44)]));cancelled.cancel();assert.equal(cancelled.end(event([],[point(100,44)])),0)
  assert.equal(cancelled.canActivate(),false,'取消横滑也不能误点底下的订单操作')
  console.log('PASS 工作区水平导航：纵滚、短滑、多指、取消、系统返回边缘不跳页，滑后防误触')

  const paymentRenderer = hookRenderer(), pending = [], lifecycle = { show: null, hide: null }
  let session = { profile:{id:'A'},token:'token-A' }, capability
  const payment = loadSource('hooks/use-delivery-payment.ts', {
    '@tarojs/taro': { useDidShow: fn => { lifecycle.show = fn }, useDidHide: fn => { lifecycle.hide = fn } },
    '../stores/session': { useSessionStore: fn => fn(session) },
    '../services/api': { request: () => { const task=deferred();pending.push(task);return task.promise } }
  }, paymentRenderer)
  paymentRenderer.setComponent(() => {capability=payment.useDeliveryPayment();return null})
  paymentRenderer.render();lifecycle.show();await paymentRenderer.flush()
  assert.equal(pending.length,1);assert.equal(capability.enabled,false)
  pending[0].resolve({wechatPayment:true});await paymentRenderer.flush();assert.equal(capability.enabled,true)
  const oldRefresh = capability.refresh();session = {profile:{id:'B'},token:'token-B'};paymentRenderer.render()
  assert.equal(capability.enabled,false,'新账号首次渲染也不能沿用上个账号的能力')
  await paymentRenderer.flush()
  pending[1].resolve({wechatPayment:true});await paymentRenderer.flush();assert.equal(capability.enabled,false,'旧账号的可支付响应不能写回')
  pending[2].reject(Error('断网'));await paymentRenderer.flush();assert.equal(capability.enabled,false)
  capability.refresh();await paymentRenderer.flush();pending[3].resolve({wechatPayment:false});await paymentRenderer.flush();assert.equal(capability.enabled,false)
  await oldRefresh;paymentRenderer.unmount()
  console.log('PASS 支付必须由实际服务端能力确认，旧账号/断网/关闭不开放付款')

  const cr = hookRenderer(), requests = [], shopWait = deferred()
  let key = '', catalogAccount = 'A'
  const catalog = loadSource('components/service-merchant-catalog.tsx', {
    '@tarojs/components': {View:'View',Text:'Text'},
    './ui':{AppState:'AppState',AppSkeleton:'AppSkeleton'},'./app-icon':'AppIcon','../media-image':{ResolvedMediaImage:'ResolvedMediaImage'},
    '../services/api':{ request: async url => {requests.push(url);if(url.endsWith('/old'))return shopWait.promise;if(url.includes('/dishes'))return {items:[],total:0};return key ? {name:key} : {items:[{merchantPublicId:'new',name:'真实字段商家'}],total:1}} },
    '../services/g-takeout-buyer':{centsToYuan:n=>String(n/100)},
    '../services/taro':{navigateTo:async()=>{},showToast:async()=>{}},
    '../stores/session':{useSessionStore:fn=>fn({profile:{id:catalogAccount}})},
    './service-merchant-catalog.scss':{}
  }, cr).default
  cr.setComponent(()=>catalog({merchantId:key}));cr.render();await cr.flush()
  assert.ok(requests.every(url=>url.startsWith('/takeout/catalog/merchants')))
  assert.ok(text(cr.tree).includes('真实字段商家'))
  key='old';cr.render();await cr.flush();key='new';catalogAccount='B';cr.render();await cr.flush();shopWait.resolve({name:'旧账号商家'});await cr.flush()
  assert.ok(!text(cr.tree).includes('旧账号商家'));assert.ok(text(cr.tree).includes('new'));cr.unmount()
  console.log('PASS 商品展示只读取目录，不创建订单，账号/商家切换丢弃迟到数据')

  const dr = hookRenderer(), authorization = deferred(), firstSend = deferred()
  let user='7', callback, show, sendCalls=0, stopped=0, subscribed=0
  const conversation={deliveryOrderPublicId:'ORDER',conversationPublicId:'CONV',tencentConversationId:'GROUPtest',assignmentVersion:1,status:'active',canSend:true,imagesEnabled:false}
  const state=()=>({profile:{id:user},identity:{currentIdentity:'rider'},status:'authenticated'})
  const store=fn=>fn(state());store.getState=state
  const reset = ()=>{}
  loadPage('delivery-conversation',{
    '@tarojs/components':{View:'View',Text:'Text',Input:'Input',ScrollView:'ScrollView',Button:'Button',Image:'Image'},
    '@tarojs/taro':{useDidShow:fn=>{show=fn}},
    '../../components/app-icon':'AppIcon','../../services/api':{isApiError:()=>false},
    '../../services/delivery-im':{
      deliveryConversationCanSend:c=>c.canSend&&c.status==='active',getDeliveryOrderConversation:()=>authorization.promise,
      subscribeDeliveryTencentMessages:async(_,fn)=>{subscribed++;callback=fn;return()=>{stopped++}},loadDeliveryTencentMessages:async()=>[],parseDeliveryImagePayload:()=>null,
      sendDeliveryTencentText:async(c,t,id,attempt)=>{sendCalls++;assert.equal(id,`campus_${user}`);if(sendCalls===1)return firstSend.promise;return {ID:'sent',flow:'out',time:1,type:'TIMTextElem',payload:{text:t}}},
    },
    '../../services/media':{},'../../services/taro':{getCurrentInstance:()=>({router:{params:{orderId:'ORDER'}}})},
    '../../stores/session':{useSessionStore:store},'../../hooks/use-keyboard-inset':{useKeyboardInset:()=>({inset:0,reset})},
    '../../hooks/use-visible-conversation-read':{useVisibleConversationRead:()=>{}},'./index.scss':{}
  },dr)
  dr.render();await dr.flush()
  const one=cl=>nodes(dr.tree,n=>hasClass(n,cl))[0]
  assert.equal(one('delivery-chat-input').props.disabled,false)
  one('delivery-chat-input').props.onInput({detail:{value:'先输入'}});dr.render();one('delivery-chat-send').props.onClick();one('delivery-chat-send').props.onClick();await dr.flush()
  assert.equal(sendCalls,0);assert.ok(text(dr.tree).includes('先输入'))
  authorization.resolve(conversation);await dr.flush();assert.equal(sendCalls,1)
  firstSend.reject(Error('网络故障'));await dr.flush();assert.ok(text(dr.tree).includes('发送失败'))
  const retry=one('delivery-chat-retry').props.onClick;retry();retry();await dr.flush();assert.equal(sendCalls,2);assert.ok(!text(dr.tree).includes('发送失败'))
  const previousSubscriptions=subscribed;show();await dr.flush();assert.equal(subscribed,previousSubscriptions+1,'回到前台应绑定当前SDK实例')
  dr.unmount();assert.equal(stopped,subscribed,'每次绑定的监听都必须清理')
  console.log('PASS 配送聊天授权前排队、失败原位提示、同帧重试防重和监听清理')

  const sdk = {getLoginUser:()=> 'campus_7',createTextMessage: opts=>({...opts,ID:'same',from:'campus_7',conversationID:'GROUPtest'}),sendMessage:async()=>{throw Error('传输失败')},resendMessage:async message=>({data:{message}})}
  let allowed=conversation
  const deliveryApi = loadSource('services/delivery-im.ts',{
    '@tencentcloud/chat':{TYPES:{CONV_GROUP:'GROUP'}},'./api':{request:async()=>allowed},
    './e-im':{ensureTencentIMReady:async()=>sdk,withTimeout:async promise=>promise}
  },hookRenderer())
  const attempt={}
  await assert.rejects(deliveryApi.sendDeliveryTencentText(conversation,'同一条','campus_7',attempt),/失败/)
  const retried=await deliveryApi.sendDeliveryTencentText(conversation,'同一条','campus_7',attempt);assert.equal(retried,attempt.message)
  allowed={...conversation,assignmentVersion:2};await assert.rejects(deliveryApi.sendDeliveryTencentText(conversation,'同一条','campus_7',attempt),/变化/)
  allowed={...conversation,status:'read_only'};await assert.rejects(deliveryApi.sendDeliveryTencentText(conversation,'同一条','campus_7',attempt),/变化/)
  console.log('PASS 配送发送前复核服务端订单与分配版本，只读/换骑手停止发送，同ID重试')
}
run().catch(error=>{console.error(error);process.exitCode=1})
