import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { adminAPI } from './services/f-admin'
import { useAdminStore } from './store'
import { MediaConsole } from './MediaConsole'
import { DeliveryConsole } from './DeliveryConsole'
import { TakeoutConsole } from './TakeoutConsole'
import { ExternalTakeoutMerchants } from './ExternalTakeoutMerchants'
import { IdentityApplications } from './IdentityApplications'
import { legacyTakeoutEnabled } from './feature-flags'
import { Login } from './Login'
import { AccountPanel } from './AccountPanel'
import { adminAuthAPI } from './services/admin-auth'
import { AdminIcon, useAdminDialog, type AdminDialogAPI, type AdminIconName } from './components'
import type { AdminSection, NavigationCategory, Page, PostRow, ProductRow, ReportRow, ReviewTask, Stats, UserRow } from './types'

const nav:Array<{key:AdminSection;label:string;icon:AdminIconName}>=[
	{key:'media',label:'媒体运营',icon:'media'},
	{key:'delivery',label:'配送运营',icon:'delivery'},
	...(legacyTakeoutEnabled?[{key:'takeout' as const,label:'外卖运营',icon:'delivery' as const}]:[]),
	{key:'dashboard',label:'数据统计',icon:'dashboard'},{key:'users',label:'用户管理',icon:'users'},
	{key:'identities',label:'身份审核',icon:'identities'},
	{key:'moderation',label:'统一审核',icon:'moderation'},{key:'community',label:'帖子管理',icon:'community'},
	{key:'trade',label:'商品管理',icon:'trade'},{key:'reports',label:'举报审核',icon:'reports'},
	{key:'navigation',label:'导航管理',icon:'navigation'}
]

function Panel({title,children}:{title:string;children:ReactNode}){return <section className="panel"><header className="panel__header"><h2>{title}</h2></header>{children}</section>}
function ErrorBox({message}:{message:string}){return <div className="placeholder"><strong>{message}</strong><p>请确认管理员会话仍然有效；如果已过期，请重新登录。</p></div>}
function reviewStatusLabel(status:ReviewTask['status']){return({submitted:'待开始',reviewing:'审核中',approved:'已通过',rejected:'已拒绝'})[status]}
function reviewObjectLabel(objectType:ReviewTask['objectType']){return({post:'帖子',comment:'评论',product:'商品',product_comment:'商品留言',report:'举报'})[objectType]}
function reviewDecisionSourceLabel(source:ReviewTask['decisionSource']){return({manual:'人工审核',automatic:'自动审核',manual_override:'人工改判'})[source]}

function Dashboard(){const[data,setData]=useState<Stats|null>(null);const[error,setError]=useState('');useEffect(()=>{adminAPI.stats().then(setData).catch(e=>setError(String(e)))},[]);if(error)return <ErrorBox message={error}/>;if(!data)return <Panel title="数据统计">加载中…</Panel>;const metrics=[['用户',data.totals.users,data.today.users],['帖子',data.totals.posts,data.today.posts],['商品',data.totals.products,data.today.products],['待审核',data.totals.pendingReview,'-']];return <section className="metric-grid">{metrics.map(([label,total,today])=><article className="metric-card" key={label}><div className="metric-card__label">{label}</div><div className="metric-card__value">{total}</div><div className="metric-card__footer"><span>今日</span><span>{today}</span></div></article>)}</section>}

function Reviews(){
	const dialog=useAdminDialog()
	const[data,setData]=useState<Page<ReviewTask>|null>(null)
	const[error,setError]=useState('')
	const[notice,setNotice]=useState('')
	const[busyTaskId,setBusyTaskId]=useState<number|null>(null)
	const[selectedTaskId,setSelectedTaskId]=useState<number|null>(null)
	const load=useCallback(async()=>{
		try{
			const value=await adminAPI.reviews()
			setData(value)
			setError('')
		}catch(e){setError(String(e))}
	},[])
	useEffect(()=>{void load()},[load])
	const selected=(data?.items||[]).find(item=>item.id===selectedTaskId)||null
	const selectTask=(item:ReviewTask)=>{setSelectedTaskId(item.id);setError('');setNotice('')}
	const act=async(item:ReviewTask,action:'start'|'approve'|'reject')=>{
		let reason=''
		if(action==='approve'&&!await dialog.confirm({title:'通过并公开',description:`确认通过${reviewObjectLabel(item.objectType)} #${item.objectId} 吗？通过后将按业务规则公开展示。`,confirmLabel:'通过并公开'}))return
		if(action==='reject'){
			reason=await dialog.prompt({title:'拒绝审核',description:`拒绝${reviewObjectLabel(item.objectType)} #${item.objectId} 后，内容不会公开展示。`,inputLabel:'拒绝原因',placeholder:'请说明具体原因',requiredMessage:'拒绝原因不能为空',confirmLabel:'确认拒绝',tone:'danger'})||''
			if(!reason)return
		}
		setBusyTaskId(item.id)
		setError('')
		setNotice('')
		try{
			if(action==='start')await adminAPI.startReview(item.id)
			else await adminAPI.decideReview(item.id,action,reason)
			setNotice(action==='start'?'已进入审核，请继续选择“通过”或“拒绝”。':action==='approve'?'审核已通过，内容将按业务规则公开展示。':'审核已拒绝。')
			await load()
		}catch(e){setError(String(e))}
		finally{setBusyTaskId(null)}
	}
	const override=async(item:ReviewTask,status:'approved'|'rejected')=>{
		const actionLabel=status==='approved'?'通过':'拒绝'
		const reason=await dialog.prompt({title:`改判为${actionLabel}`,description:`确认将任务 #${item.id} 的${reviewObjectLabel(item.objectType)} #${item.objectId} 改判为“${actionLabel}”。`,inputLabel:'改判原因',placeholder:'请输入可追溯的改判依据',requiredMessage:'改判原因不能为空',confirmLabel:`确认${actionLabel}`,tone:status==='rejected'?'danger':'default'})||''
		if(!reason)return
		setBusyTaskId(item.id)
		setError('')
		setNotice('')
		try{
			await adminAPI.overrideReview(item.id,status,reason)
			setNotice(`任务 #${item.id} 已由管理员改判为“${actionLabel}”。`)
			await load()
		}catch(e){setError(String(e))}
		finally{setBusyTaskId(null)}
	}
	return <Panel title="提交 → 审核 → 通过/拒绝 → 展示">
		<p className="review-guide">点击任务行或“处理审核”打开操作区。商品管理页只展示状态，不直接执行审核。</p>
		{error&&<div className="review-feedback review-feedback--error" role="alert">{error}</div>}
		{notice&&<div className="review-feedback review-feedback--success" role="status">{notice}</div>}
		<div className="audit-list">
			{(data?.items||[]).map(item=><div
				className={`audit-row audit-row--selectable ${selectedTaskId===item.id?'audit-row--selected':''}`}
				key={item.id}
				role="button"
				tabIndex={0}
				onClick={()=>selectTask(item)}
				onKeyDown={event=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();selectTask(item)}}}
			>
				<div>
					<div className="audit-row__title"><span className="type-chip">{reviewObjectLabel(item.objectType)}</span>{reviewObjectLabel(item.objectType)} #{item.objectId}<small>任务 #{item.id}</small></div>
					<p><span className={`decision-source decision-source--${item.decisionSource}`}>{reviewDecisionSourceLabel(item.decisionSource)}</span>{item.risk||'无风险备注'} · {item.createdAt}</p>
				</div>
				<div className="audit-row__actions">
					<span className={`status status--${item.status}`}>{reviewStatusLabel(item.status)}</span>
					<button type="button" onClick={event=>{event.stopPropagation();selectTask(item)}}>{selectedTaskId===item.id?'已展开':'处理审核'}</button>
				</div>
			</div>)}
			{data&&data.items.length===0&&<div className="placeholder">当前没有审核任务</div>}
		</div>
		{selected&&<section className="review-detail" aria-label={`审核任务 ${selected.id}`}>
			<header><div><span>当前任务</span><strong>{reviewObjectLabel(selected.objectType)} #{selected.objectId}</strong></div><span className={`status status--${selected.status}`}>{reviewStatusLabel(selected.status)}</span></header>
			<div className="review-detail__facts"><span>任务编号：#{selected.id}</span><span>决策来源：{reviewDecisionSourceLabel(selected.decisionSource)}</span><span>风险备注：{selected.risk||'无'}</span><span>决策原因：{selected.reason||'无'}</span><span>提交时间：{selected.createdAt}</span></div>
			<div className="review-detail__actions">
				{selected.status==='submitted'&&<button type="button" disabled={busyTaskId===selected.id} onClick={()=>void act(selected,'start')}>{busyTaskId===selected.id?'处理中…':'开始审核'}</button>}
				{selected.status==='reviewing'&&<><button type="button" disabled={busyTaskId===selected.id} onClick={()=>void act(selected,'approve')}>{busyTaskId===selected.id?'处理中…':'通过并公开'}</button><button type="button" className="danger" disabled={busyTaskId===selected.id} onClick={()=>void act(selected,'reject')}>拒绝</button></>}
				{(selected.status==='approved'||selected.status==='rejected')&&(selected.decisionSource==='automatic'||selected.decisionSource==='manual_override')&&<><button type="button" disabled={busyTaskId===selected.id||selected.status==='approved'} onClick={()=>void override(selected,'approved')}>改判为通过</button><button type="button" className="danger" disabled={busyTaskId===selected.id||selected.status==='rejected'} onClick={()=>void override(selected,'rejected')}>改判为拒绝</button></>}
				{(selected.status==='approved'||selected.status==='rejected')&&selected.decisionSource==='manual'&&<span>该任务已由人工完成，无需再次操作。</span>}
			</div>
		</section>}
	</Panel>
}

function Users(){const[data,setData]=useState<Page<UserRow>|null>(null);const[error,setError]=useState('');const load=()=>adminAPI.users().then(setData).catch(e=>setError(String(e)));const act=async(run:()=>Promise<unknown>)=>{try{setError('');await run();await load()}catch(e){setError(String(e))}};useEffect(()=>{void load()},[]);return <Panel title="用户与管理员权限"><>{error&&<ErrorBox message={error}/>}<table><thead><tr><th>用户</th><th>学校/校区</th><th>角色</th><th>状态</th><th>状态操作</th><th>角色操作（仅超管）</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.nickname}</strong><span>#{row.id}</span></td><td>{row.school} / {row.campus}</td><td>{row.role}</td><td>{row.status}</td><td><button onClick={()=>act(()=>adminAPI.updateUserStatus(row.id,'active'))}>启用</button><button onClick={()=>act(()=>adminAPI.updateUserStatus(row.id,'muted'))}>禁言</button><button onClick={()=>act(()=>adminAPI.updateUserStatus(row.id,'banned'))}>封禁</button></td><td><button onClick={()=>act(()=>adminAPI.updateUserRole(row.id,'user'))}>用户</button><button onClick={()=>act(()=>adminAPI.updateUserRole(row.id,'admin'))}>管理员</button><button onClick={()=>act(()=>adminAPI.updateUserRole(row.id,'superadmin'))}>超级管理员</button></td></tr>)}</tbody></table></></Panel>}

function Posts(){const[data,setData]=useState<Page<PostRow>|null>(null);const[error,setError]=useState('');useEffect(()=>{adminAPI.posts().then(setData).catch(e=>setError(String(e)))},[]);if(error)return <ErrorBox message={error}/>;return <Panel title="帖子管理"><table><thead><tr><th>帖子</th><th>作者</th><th>话题</th><th>审核状态</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.title}</strong><span>#{row.id}</span></td><td>{row.author.nickname}</td><td>{row.topic}</td><td>{row.reviewStatus}</td></tr>)}</tbody></table></Panel>}

function Products(){const[data,setData]=useState<Page<ProductRow>|null>(null);const[error,setError]=useState('');useEffect(()=>{adminAPI.products().then(setData).catch(e=>setError(String(e)))},[]);if(error)return <ErrorBox message={error}/>;return <Panel title="商品管理"><table><thead><tr><th>商品</th><th>卖家</th><th>价格</th><th>业务状态</th><th>审核状态</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.title}</strong><span>#{row.id}</span></td><td>{row.seller.nickname}</td><td>¥{(row.price/100).toFixed(2)}</td><td>{row.status}</td><td>{row.reviewStatus}</td></tr>)}</tbody></table></Panel>}

function Reports(){const[data,setData]=useState<Page<ReportRow>|null>(null);const[error,setError]=useState('');useEffect(()=>{adminAPI.reports().then(setData).catch(e=>setError(String(e)))},[]);if(error)return <ErrorBox message={error}/>;return <Panel title="举报管理"><table><thead><tr><th>目标</th><th>原因</th><th>审核状态</th><th>提交时间</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td>{row.targetType} #{row.targetId}</td><td>{row.reason}</td><td>{row.reviewStatus}</td><td>{row.createdAt}</td></tr>)}</tbody></table></Panel>}

function Navigation(){const[data,setData]=useState<NavigationCategory[]>([]);const[error,setError]=useState('');useEffect(()=>{adminAPI.navigation().then(setData).catch(e=>setError(String(e)))},[]);return <div className="navigation-console"><ExternalTakeoutMerchants/>{error?<ErrorBox message={error}/>:<Panel title="普通导航入口（仅 AppID + Path）">{data.map(category=><div key={category.id}><h3>{category.name}</h3><table><thead><tr><th>名称</th><th>AppID</th><th>Path</th><th>状态</th></tr></thead><tbody>{category.items.map(item=><tr key={item.id}><td>{item.title}</td><td>{item.appId}</td><td>{item.path}</td><td>{item.enabled?'enabled':'disabled'}</td></tr>)}</tbody></table></div>)}</Panel>}</div>}

async function takedownReason(dialog:AdminDialogAPI,kind:string,title:string){return dialog.prompt({title:`下架${kind}`,description:`确认下架“${title}”。下架后该内容将从用户端消失，恢复需重新提交审核。`,inputLabel:'下架原因',placeholder:'请输入本次操作原因',requiredMessage:'下架原因不能为空',confirmLabel:'确认下架',tone:'danger'})}

function TakedownPosts(){const dialog=useAdminDialog();const[data,setData]=useState<Page<PostRow>|null>(null);const[error,setError]=useState('');const[busy,setBusy]=useState<number|null>(null);const load=useCallback(()=>adminAPI.posts().then(setData).catch(e=>setError(String(e))),[]);useEffect(()=>{void load()},[load]);const takedown=async(row:PostRow)=>{const reason=await takedownReason(dialog,'帖子',row.title);if(reason===null)return;setBusy(row.id);setError('');try{await adminAPI.takedownPost(row.id,reason);await load()}catch(e){setError(String(e))}finally{setBusy(null)}};return <Panel title="帖子管理"><>{error&&<div className="content-takedown-error" role="alert">{error}</div>}<table><thead><tr><th>帖子</th><th>作者</th><th>话题</th><th>审核状态</th><th>操作</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.title}</strong><span>#{row.id}</span></td><td>{row.author.nickname}</td><td>{row.topic}</td><td>{row.reviewStatus}</td><td>{row.reviewStatus==='approved'?<button className="content-takedown-button" disabled={busy===row.id} onClick={()=>void takedown(row)}>{busy===row.id?'下架中…':'下架'}</button>:<span>不可下架</span>}</td></tr>)}</tbody></table></></Panel>}

function TakedownProducts(){const dialog=useAdminDialog();const[data,setData]=useState<Page<ProductRow>|null>(null);const[error,setError]=useState('');const[busy,setBusy]=useState<number|null>(null);const setSection=useAdminStore(s=>s.setSection);const load=useCallback(()=>adminAPI.products().then(setData).catch(e=>setError(String(e))),[]);useEffect(()=>{void load()},[load]);const takedown=async(row:ProductRow)=>{const reason=await takedownReason(dialog,'商品',row.title);if(reason===null)return;setBusy(row.id);setError('');try{await adminAPI.takedownProduct(row.id,reason);await load()}catch(e){setError(String(e))}finally{setBusy(null)}};return <Panel title="商品管理"><>{error&&<div className="content-takedown-error" role="alert">{error}</div>}<table><thead><tr><th>商品</th><th>卖家</th><th>价格</th><th>业务状态</th><th>审核状态</th><th>操作</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.title}</strong><span>#{row.id}</span></td><td>{row.seller.nickname}</td><td>¥{(row.price/100).toFixed(2)}</td><td>{row.status}</td><td>{row.reviewStatus}</td><td>{row.reviewStatus==='approved'?<button className="content-takedown-button" disabled={busy===row.id} onClick={()=>void takedown(row)}>{busy===row.id?'下架中…':'下架'}</button>:row.reviewStatus==='submitted'||row.reviewStatus==='reviewing'?<button type="button" className="review-link-button" onClick={()=>setSection('moderation')}>前往审核</button>:<span>不可下架</span>}</td></tr>)}</tbody></table></></Panel>}

function Content(){const section=useAdminStore(s=>s.section);if(section==='dashboard')return <Dashboard/>;if(section==='users')return <Users/>;if(section==='identities')return <IdentityApplications/>;if(section==='moderation')return <Reviews/>;if(section==='media')return <MediaConsole/>;if(section==='delivery')return <DeliveryConsole/>;if(section==='takeout'&&legacyTakeoutEnabled)return <TakeoutConsole/>;if(section==='community')return <TakedownPosts/>;if(section==='trade')return <TakedownProducts/>;if(section==='reports')return <Reports/>;return <Navigation/>}

function AdminShell(){const section=useAdminStore(s=>s.section);const setSection=useAdminStore(s=>s.setSection);const active=nav.find(item=>item.key===section);return <div className="app-shell"><aside className="sidebar"><div className="brand"><div className="brand__logo">湛</div><div><strong>湛科生活圈</strong><span>运营管理后台</span></div></div><nav>{nav.map(item=><button key={item.key} type="button" className={section===item.key?'active':''} onClick={()=>setSection(item.key)}><span><AdminIcon name={item.icon}/></span>{item.label}</button>)}</nav></aside><main className="main"><header className="topbar"><div><span className="eyebrow">{section==='takeout'?'外卖业务':section==='delivery'?'配送业务':'校园业务'}</span><h1>{active?.label}</h1></div><AccountPanel/></header><Content/></main></div>}

export default function App(){
	const authStatus=useAdminStore(state=>state.authStatus)
	const setAuthenticated=useAdminStore(state=>state.setAuthenticated)
	const setUnauthenticated=useAdminStore(state=>state.setUnauthenticated)
	useEffect(()=>{
		if(authStatus!=='checking')return
		let active=true
		adminAuthAPI.session().then(session=>{if(active)setAuthenticated(session)}).catch(()=>{if(active)setUnauthenticated()})
		return()=>{active=false}
	},[authStatus,setAuthenticated,setUnauthenticated])
	if(authStatus==='checking')return <main className="auth-loading"><div className="auth-loading__mark">湛</div><p>正在验证管理员会话…</p></main>
	if(authStatus==='unauthenticated')return <Login/>
	return <AdminShell/>
}
