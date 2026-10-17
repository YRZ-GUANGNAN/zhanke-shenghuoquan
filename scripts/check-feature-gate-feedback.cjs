const assert = require('node:assert/strict'), fs = require('node:fs'), path = require('node:path'), Module = require('node:module')
const ts = require('typescript'), React = require('react'), { createRoot } = require('react-dom/client'), { JSDOM } = require('jsdom')
async function run() {
  const dom = new JSDOM('<main></main>')
  const saved = ['window','document','IS_REACT_ACT_ENVIRONMENT'].map(key => [key,Object.getOwnPropertyDescriptor(globalThis,key)])
  Object.assign(globalThis,{window:dom.window,document:dom.window.document,IS_REACT_ACT_ENVIRONMENT:true})
  let visibility = { available:false, forum:false }, mounts=0
  const pending=[]
  const request=()=>new Promise(resolve=>pending.push(resolve))
  const view=p=>React.createElement('div',{className:p.className},p.children)
  const router={path:'pages/post-detail/index',params:{id:'1'},$taroPath:'post-1'}
  const file=path.resolve(__dirname,'../apps/miniapp/src/components/feature-page-gate.tsx')
  const loaded=new Module(file,module)
  loaded.require=id=>{
    if(id==='react'||id==='react/jsx-runtime')return require(id)
    if(id==='@tarojs/components')return {View:view}
    if(id==='@tarojs/taro')return {useRouter:()=>router,getCurrentInstance:()=>({router}),hideShareMenu:async()=>{},useDidShow:()=>{}}
    if(id==='./ui')return {
      AppState:p=>React.createElement('div',{'data-state':p.kind},p.title,p.actionText&&React.createElement('button',{onClick:p.onAction},p.actionText)),
      AppButton:p=>React.createElement('button',{onClick:p.onClick},p.children)
    }
    if(id.endsWith('use-feature-visibility'))return {useFeatureVisibility:()=>visibility}
    if(id.endsWith('feature-policy-model'))return {featurePageKeys:()=>['forum']}
    if(id.endsWith('feature-visibility'))return {getFeatureVisibility:request}
    if(id.endsWith('stores/session'))return {useSessionStore:()=> 'authenticated'}
    if(id.endsWith('services/taro'))return {default:{reLaunch:async()=>{}}}
    if(id.endsWith('.scss'))return {}
    throw Error(id)
  }
  loaded._compile(ts.transpileModule(fs.readFileSync(file,'utf8'),{fileName:file,compilerOptions:{jsx:ts.JsxEmit.ReactJSX,module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText,file)
  const Page=()=>{React.useEffect(()=>{mounts++},[]);return React.createElement('div',null,'业务页面')}
  const app=createRoot(document.querySelector('main'))
  const render=()=>React.act(async()=>app.render(React.createElement(loaded.exports.FeaturePageGate,null,React.createElement(Page))))
  try{
    await render()
    assert.ok(document.querySelector('[data-state=loading]'),'initial lookup is loading, not an offline flash')
    assert.equal(mounts,0)
    assert.equal(pending.length,1)
    await React.act(async()=>pending[0]({available:false}))
    assert.ok(document.querySelector('[data-state=offline]'))
    assert.equal(pending.length,1,'failure cannot trigger an unbounded auto retry')
    const retry=[...document.querySelectorAll('button')].find(b=>b.textContent==='重新读取')
    await React.act(async()=>{retry.click();retry.click()})
    assert.equal(pending.length,2,'same-frame retries are deduplicated')
    visibility={available:true,forum:false}
    await React.act(async()=>pending[1](visibility))
    await render()
    assert.ok(document.querySelector('[data-state=forbidden]'))
    assert.equal(mounts,0,'closed feature does not mount business page')
    visibility={available:true,forum:true}
    await render()
    assert.equal(mounts,1)
    console.log('feature gate feedback: PASS (pending, failure, deduplication, closed page isolation)')
  }finally{
    await React.act(async()=>app.unmount());dom.window.close()
    saved.forEach(([key,d])=>d?Object.defineProperty(globalThis,key,d):delete globalThis[key])
  }
}
run().catch(error=>{console.error(error);process.exitCode=1})
