import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
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
import { AdminButton, AdminField, AdminIcon, AdminNotice, AdminPageHeader, AdminPagination, AdminPanel, AdminRowActions, AdminSelect, AdminState, AdminStatusBadge, AdminTableFrame, AdminToolbar, useAdminDialog, type AdminDialogAPI } from './components'
import { adminNavigation, adminNavigationGroups, type AdminNavigationItem } from './admin-navigation'
import type { AdminSection, FeedbackRow, NavigationCategory, NavigationItem, NavigationItemInput, Page, PageQuery, PostRow, ProductRow, ReportRow, ReviewTask, Stats, UserRow } from './types'

const takeoutNavigation:AdminNavigationItem={key:'takeout',label:'外卖运营',description:'处理商家、菜品、订单与结算',icon:'delivery',group:'运营中心'}
const visibleNavigation=legacyTakeoutEnabled?[...adminNavigation.slice(0,2),takeoutNavigation,...adminNavigation.slice(2)]:adminNavigation

function Panel({title,children}:{title:string;children:ReactNode}){return <AdminPanel title={title}>{children}</AdminPanel>}
function ErrorBox({message}:{message:string}){return <AdminState kind="error" title="暂时无法读取数据" description={`${message} 请确认管理员会话仍然有效；如果已过期，请重新登录。`}/>}
function displayTime(value:string){if(!value)return '—';const date=new Date(value);return Number.isNaN(date.getTime())?value:new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(date)}
const ADMIN_PAGE_SIZE=20
function reviewStatusLabel(status:ReviewTask['status']){return({submitted:'待开始',reviewing:'审核中',approved:'已通过',rejected:'已拒绝'})[status]}
function reviewObjectLabel(objectType:ReviewTask['objectType']){return({post:'帖子',comment:'评论',product:'商品',product_comment:'商品留言',report:'举报'})[objectType]}
function reviewDecisionSourceLabel(source:ReviewTask['decisionSource']){return({manual:'人工审核',automatic:'自动审核',manual_override:'人工改判'})[source]}

const REVIEW_PAGE_SIZE=20
type ReviewStatusFilter=ReviewTask['status']|''

function Dashboard(){
	const[data,setData]=useState<Stats|null>(null)
	const[error,setError]=useState('')
	const setSection=useAdminStore(state=>state.setSection)
	useEffect(()=>{adminAPI.stats().then(setData).catch(e=>setError(String(e)))},[])
	if(error)return <ErrorBox message={error}/>
	if(!data)return <AdminState kind="loading" title="正在汇总平台数据" description="统计数据加载完成后会自动更新。"/>
	const metrics=[
		{label:'注册用户',total:data.totals.users,today:data.today.users},
		{label:'社区帖子',total:data.totals.posts,today:data.today.posts},
		{label:'二手商品',total:data.totals.products,today:data.today.products},
		{label:'待处理审核',total:data.totals.pendingReview,today:null},
	]
	return <div className="dashboard-layout">
		<section className="metric-grid" aria-label="平台核心指标">{metrics.map(metric=><article className="metric-card" key={metric.label}><div className="metric-card__label">{metric.label}</div><div className="metric-card__value">{metric.total}</div><div className="metric-card__footer"><span>{metric.today===null?'工作状态':'今日新增'}</span><strong>{metric.today===null?`${metric.total} 条待处理`:`+${metric.today}`}</strong></div></article>)}</section>
		<AdminPanel title="今日运营工作" description="快捷入口只基于现有真实指标与管理模块。">
			<div className="dashboard-actions">
				<button type="button" onClick={()=>setSection('moderation')}><span>内容审核</span><strong>{data.totals.pendingReview}</strong><small>处理待审内容与人工改判</small></button>
				<button type="button" onClick={()=>setSection('users')}><span>用户与权限</span><strong>{data.totals.users}</strong><small>查看用户状态和管理员角色</small></button>
				<button type="button" onClick={()=>setSection('navigation')}><span>服务导航</span><strong>配置</strong><small>维护校内服务入口与商家目录</small></button>
			</div>
		</AdminPanel>
	</div>
}

function Reviews(){
	const dialog=useAdminDialog()
	const[data,setData]=useState<Page<ReviewTask>|null>(null)
	const[error,setError]=useState('')
	const[notice,setNotice]=useState('')
	const[loading,setLoading]=useState(false)
	const[busyTaskId,setBusyTaskId]=useState<number|null>(null)
	const[selectedTaskId,setSelectedTaskId]=useState<number|null>(null)
	const[reviewStatus,setReviewStatus]=useState<ReviewStatusFilter>('submitted')
	const[reviewPage,setReviewPage]=useState(1)
	const requestSequence=useRef(0)
	const load=useCallback(async()=>{
		const requestId=++requestSequence.current
		setLoading(true)
		try{
			const value=await adminAPI.reviews({status:reviewStatus,page:reviewPage,pageSize:REVIEW_PAGE_SIZE})
			if(requestId!==requestSequence.current)return
			if(value.items.length===0&&reviewPage>1){setReviewPage(current=>Math.max(1,current-1));return}
			setData(value)
			setSelectedTaskId(current=>current!==null&&value.items.some(item=>item.id===current)?current:null)
			setError('')
		}catch(e){if(requestId===requestSequence.current)setError(String(e))}
		finally{if(requestId===requestSequence.current)setLoading(false)}
	},[reviewPage,reviewStatus])
	useEffect(()=>{void load()},[load])
	const selected=(data?.items||[]).find(item=>item.id===selectedTaskId)||null
	const totalPages=Math.max(1,Math.ceil((data?.total||0)/(data?.pageSize||REVIEW_PAGE_SIZE)))
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
			setNotice(action==='start'?'已进入审核，可在“审核中”筛选继续选择“通过”或“拒绝”。':action==='approve'?'审核已通过，内容将按业务规则公开展示。':'审核已拒绝。')
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
		<div className="review-toolbar">
			<label className="review-filter">
				<span>审核状态</span>
				<select value={reviewStatus} onChange={event=>{setReviewStatus(event.target.value as ReviewStatusFilter);setReviewPage(1);setSelectedTaskId(null);setData(null);setNotice('');setError('')}}>
					<option value="submitted">待审核</option>
					<option value="reviewing">审核中</option>
					<option value="approved">已通过</option>
					<option value="rejected">已拒绝</option>
					<option value="">全部</option>
				</select>
			</label>
			<div className="review-toolbar__summary" role="status">
				<strong>{data?.total??0}</strong><span>条任务</span><span>第 {data?.page??reviewPage} / {totalPages} 页</span>{loading&&<span>刷新中…</span>}
			</div>
		</div>
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
		<div className="review-pagination" aria-label="审核任务分页">
			<button type="button" disabled={loading||reviewPage<=1} onClick={()=>setReviewPage(current=>Math.max(1,current-1))}>上一页</button>
			<span>共 {data?.total??0} 条 · 每页 {data?.pageSize??REVIEW_PAGE_SIZE} 条</span>
			<button type="button" disabled={loading||reviewPage>=totalPages||(data?.total??0)===0} onClick={()=>setReviewPage(current=>current+1)}>下一页</button>
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

function usePagedAdminData<T>(loader:(query:PageQuery)=>Promise<Page<T>>){
	const[page,setPage]=useState(1)
	const[data,setData]=useState<Page<T>|null>(null)
	const[loading,setLoading]=useState(false)
	const[error,setError]=useState('')
	const load=useCallback(async()=>{
		setLoading(true)
		try{
			const value=await loader({page,pageSize:ADMIN_PAGE_SIZE})
			if(value.items.length===0&&page>1){setPage(current=>Math.max(1,current-1));return}
			setData(value);setError('')
		}catch(e){setError(String(e))}
		finally{setLoading(false)}
	},[loader,page])
	useEffect(()=>{void load()},[load])
	return{page,setPage,data,loading,error,setError,load}
}

function Users(){
	const dialog=useAdminDialog()
	const sessionUser=useAdminStore(state=>state.user)
	const{page,setPage,data,loading,error,setError,load}=usePagedAdminData<UserRow>(adminAPI.users)
	const[busy,setBusy]=useState<number|null>(null)
	const[notice,setNotice]=useState('')
	const changeStatus=async(row:UserRow,status:UserRow['status'])=>{
		if(status===row.status)return
		const label={active:'启用',muted:'禁言',banned:'封禁'}[status]
		if(!await dialog.confirm({title:`${label}用户`,description:`确认将“${row.nickname}”的账号状态调整为“${label}”吗？`,confirmLabel:`确认${label}`,tone:status==='banned'?'danger':'default'}))return
		setBusy(row.id);setError('');setNotice('')
		try{await adminAPI.updateUserStatus(row.id,status);setNotice(`已更新“${row.nickname}”的账号状态。`);await load()}catch(e){setError(String(e))}finally{setBusy(null)}
	}
	const changeRole=async(row:UserRow,role:UserRow['role'])=>{
		if(role===row.role)return
		const roleName={user:'普通用户',admin:'管理员',superadmin:'超级管理员'}[role]
		if(!await dialog.confirm({title:'调整管理员角色',description:`确认将“${row.nickname}”调整为“${roleName}”吗？系统仍会保护当前账号和最后一名启用超级管理员。`,confirmLabel:'确认调整',tone:role==='user'?'danger':'default'}))return
		setBusy(row.id);setError('');setNotice('')
		try{await adminAPI.updateUserRole(row.id,role);setNotice(`已更新“${row.nickname}”的管理员角色。`);await load()}catch(e){setError(String(e))}finally{setBusy(null)}
	}
	return <AdminPanel title="用户与管理员权限" description="状态和角色变更均受后端权限与最后超级管理员保护。">
		<AdminToolbar summary={data?`共 ${data.total} 名用户`:'正在读取用户'} actions={<AdminButton tone="ghost" loading={loading} onClick={()=>void load()}>刷新</AdminButton>}/>
		{error&&<AdminNotice tone="danger" title="无法更新用户">{error}</AdminNotice>}
		{notice&&<AdminNotice tone="success">{notice}</AdminNotice>}
		{!data&&loading?<AdminState kind="loading" title="正在读取用户" description="用户与权限数据加载完成后会自动显示。"/>:data?.items.length===0?<AdminState kind="empty" title="暂无用户记录" description="当前页没有可展示的用户。"/>:<AdminTableFrame label="用户管理列表"><table className="admin-data-table"><thead><tr><th>用户</th><th>学校 / 校区</th><th>角色</th><th>状态</th><th className="admin-table-actions">管理操作</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.nickname||'未设置昵称'}</strong><span>#{row.id}{sessionUser?.id===row.id?' · 当前账号':''}</span></td><td>{row.school||'未设置'}<span>{row.campus||'未设置'}</span></td><td><AdminStatusBadge value={row.role}/></td><td><AdminStatusBadge value={row.status}/></td><td className="admin-table-actions"><AdminRowActions><select className="admin-inline-select" aria-label={`调整 ${row.nickname} 的状态`} value={row.status} disabled={busy===row.id} onChange={event=>void changeStatus(row,event.target.value as UserRow['status'])}><option value="active">正常</option><option value="muted">禁言</option><option value="banned">封禁</option></select><select className="admin-inline-select" aria-label={`调整 ${row.nickname} 的角色`} value={row.role} disabled={busy===row.id||sessionUser?.role!=='superadmin'} onChange={event=>void changeRole(row,event.target.value as UserRow['role'])}><option value="user">普通用户</option><option value="admin">管理员</option><option value="superadmin">超级管理员</option></select></AdminRowActions></td></tr>)}</tbody></table></AdminTableFrame>}
		{data&&<AdminPagination page={page} pageSize={data.pageSize} total={data.total} loading={loading} onPageChange={setPage}/>}
	</AdminPanel>
}

function Posts(){
	const dialog=useAdminDialog()
	const{page,setPage,data,loading,error,setError,load}=usePagedAdminData<PostRow>(adminAPI.posts)
	const[busy,setBusy]=useState<number|null>(null)
	const[notice,setNotice]=useState('')
	const takedown=async(row:PostRow)=>{const reason=await takedownReason(dialog,'帖子',row.title);if(reason===null)return;setBusy(row.id);setError('');setNotice('');try{await adminAPI.takedownPost(row.id,reason);setNotice(`帖子“${row.title}”已下架。`);await load()}catch(e){setError(String(e))}finally{setBusy(null)}}
	return <AdminPanel title="帖子管理" description="查看社区帖子审核状态，并对已公开内容执行合规下架。">
		<AdminToolbar summary={data?`共 ${data.total} 条帖子`:'正在读取帖子'} actions={<AdminButton tone="ghost" loading={loading} onClick={()=>void load()}>刷新</AdminButton>}/>
		{error&&<AdminNotice tone="danger" title="帖子操作失败">{error}</AdminNotice>}{notice&&<AdminNotice tone="success">{notice}</AdminNotice>}
		{!data&&loading?<AdminState kind="loading" title="正在读取帖子" description="帖子数据加载完成后会自动显示。"/>:data?.items.length===0?<AdminState kind="empty" title="暂无帖子" description="当前页没有可展示的帖子。"/>:<AdminTableFrame label="帖子管理列表"><table className="admin-data-table"><thead><tr><th>帖子</th><th>作者</th><th>话题</th><th>审核状态</th><th>提交时间</th><th className="admin-table-actions">操作</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.title||'无标题帖子'}</strong><span>#{row.id}</span></td><td>{row.author.nickname||'未知用户'}<span>{[row.author.school,row.author.campus].filter(Boolean).join(' / ')||'学校未设置'}</span></td><td>{row.topic||'未分类'}</td><td><AdminStatusBadge value={row.reviewStatus}/></td><td>{displayTime(row.createdAt)}</td><td className="admin-table-actions">{row.reviewStatus==='approved'?<AdminButton tone="text" loading={busy===row.id} onClick={()=>void takedown(row)}>下架</AdminButton>:<span className="admin-unavailable">当前不可下架</span>}</td></tr>)}</tbody></table></AdminTableFrame>}
		{data&&<AdminPagination page={page} pageSize={data.pageSize} total={data.total} loading={loading} onPageChange={setPage}/>}
	</AdminPanel>
}

function Products(){
	const dialog=useAdminDialog()
	const setSection=useAdminStore(state=>state.setSection)
	const{page,setPage,data,loading,error,setError,load}=usePagedAdminData<ProductRow>(adminAPI.products)
	const[busy,setBusy]=useState<number|null>(null)
	const[notice,setNotice]=useState('')
	const takedown=async(row:ProductRow)=>{const reason=await takedownReason(dialog,'商品',row.title);if(reason===null)return;setBusy(row.id);setError('');setNotice('');try{await adminAPI.takedownProduct(row.id,reason);setNotice(`商品“${row.title}”已下架。`);await load()}catch(e){setError(String(e))}finally{setBusy(null)}}
	return <AdminPanel title="商品管理" description="商品审核由统一审核处理，本页负责业务状态查看和合规下架。">
		<AdminToolbar summary={data?`共 ${data.total} 件商品`:'正在读取商品'} actions={<AdminButton tone="ghost" loading={loading} onClick={()=>void load()}>刷新</AdminButton>}/>
		{error&&<AdminNotice tone="danger" title="商品操作失败">{error}</AdminNotice>}{notice&&<AdminNotice tone="success">{notice}</AdminNotice>}
		{!data&&loading?<AdminState kind="loading" title="正在读取商品" description="商品数据加载完成后会自动显示。"/>:data?.items.length===0?<AdminState kind="empty" title="暂无商品" description="当前页没有可展示的商品。"/>:<AdminTableFrame label="商品管理列表"><table className="admin-data-table"><thead><tr><th>商品</th><th>卖家</th><th>价格</th><th>业务状态</th><th>审核状态</th><th className="admin-table-actions">操作</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{row.title||'无标题商品'}</strong><span>#{row.id} · {row.category||'未分类'}</span></td><td>{row.seller.nickname||'未知用户'}</td><td className="admin-price">¥{(row.price/100).toFixed(2)}</td><td><AdminStatusBadge value={row.status}/></td><td><AdminStatusBadge value={row.reviewStatus}/></td><td className="admin-table-actions"><AdminRowActions>{row.reviewStatus==='approved'?<AdminButton tone="text" loading={busy===row.id} onClick={()=>void takedown(row)}>下架</AdminButton>:row.reviewStatus==='submitted'||row.reviewStatus==='reviewing'?<AdminButton tone="text" onClick={()=>setSection('moderation')}>前往审核</AdminButton>:<span className="admin-unavailable">当前不可操作</span>}</AdminRowActions></td></tr>)}</tbody></table></AdminTableFrame>}
		{data&&<AdminPagination page={page} pageSize={data.pageSize} total={data.total} loading={loading} onPageChange={setPage}/>}
	</AdminPanel>
}

function Reports(){
	const setSection=useAdminStore(state=>state.setSection)
	const{page,setPage,data,loading,error,load}=usePagedAdminData<ReportRow>(adminAPI.reports)
	const targetLabel=(value:string)=>({post:'帖子',comment:'评论',product:'商品',product_comment:'商品留言',user:'用户'}[value]||value)
	return <AdminPanel title="举报审核" description="举报证据在此集中查看，实际处置沿用统一审核流程。">
		<AdminToolbar summary={data?`共 ${data.total} 条举报`:'正在读取举报'} actions={<><AdminButton tone="ghost" loading={loading} onClick={()=>void load()}>刷新</AdminButton><AdminButton tone="primary" onClick={()=>setSection('moderation')}>进入统一审核</AdminButton></>}/>
		{error&&<AdminNotice tone="danger" title="无法读取举报">{error}</AdminNotice>}
		{!data&&loading?<AdminState kind="loading" title="正在读取举报" description="举报数据加载完成后会自动显示。"/>:data?.items.length===0?<AdminState kind="empty" title="暂无举报" description="当前页没有待展示的举报。"/>:<AdminTableFrame label="举报审核列表"><table className="admin-data-table"><thead><tr><th>举报目标</th><th>原因</th><th>审核状态</th><th>提交时间</th><th className="admin-table-actions">操作</th></tr></thead><tbody>{(data?.items||[]).map(row=><tr key={row.id}><td><strong>{targetLabel(row.targetType)} #{row.targetId}</strong><span>举报 #{row.id}</span></td><td className="admin-cell-wrap">{row.reason||'未填写原因'}</td><td><AdminStatusBadge value={row.reviewStatus}/></td><td>{displayTime(row.createdAt)}</td><td className="admin-table-actions"><AdminButton tone="text" onClick={()=>setSection('moderation')}>前往审核</AdminButton></td></tr>)}</tbody></table></AdminTableFrame>}
		{data&&<AdminPagination page={page} pageSize={data.pageSize} total={data.total} loading={loading} onPageChange={setPage}/>}
	</AdminPanel>
}

function Feedbacks(){
	const{page,setPage,data,loading,error,load}=usePagedAdminData<FeedbackRow>(adminAPI.feedbacks)
	const categoryLabel=(value:string)=>({bug:'功能异常',suggestion:'产品建议',content:'内容问题',account:'账号问题',other:'其他'}[value]||value||'未分类')
	return <AdminPanel title="用户反馈" description="此处仅展示用户提交的真实反馈；当前后端未提供处理状态写入接口。">
		<AdminToolbar summary={data?`共 ${data.total} 条反馈`:'正在读取反馈'} actions={<AdminButton tone="ghost" loading={loading} onClick={()=>void load()}>刷新</AdminButton>}/>
		{error&&<AdminNotice tone="danger" title="无法读取反馈">{error}</AdminNotice>}
		{!data&&loading?<AdminState kind="loading" title="正在读取反馈" description="反馈数据加载完成后会自动显示。"/>:data?.items.length===0?<AdminState kind="empty" title="暂无用户反馈" description="当前页没有可展示的反馈。"/>:<AdminTableFrame label="用户反馈列表"><table className="admin-data-table"><thead><tr><th>提交用户</th><th>分类</th><th>反馈内容</th><th>附件</th><th>状态</th><th>提交时间</th></tr></thead><tbody>{(data?.items||[]).map((row,index)=><tr key={row.id||`${row.userId}-${row.createdAt}-${index}`}><td><strong>用户 #{row.userId}</strong></td><td>{categoryLabel(row.category)}</td><td className="admin-cell-wrap admin-feedback-content">{row.content}</td><td>{row.images?.length||0} 张</td><td><AdminStatusBadge value={row.status}/></td><td>{displayTime(row.createdAt)}</td></tr>)}</tbody></table></AdminTableFrame>}
		{data&&<AdminPagination page={page} pageSize={data.pageSize} total={data.total} loading={loading} onPageChange={setPage}/>}
	</AdminPanel>
}

type NavigationItemDraft=NavigationItemInput&{id?:number}
const emptyNavigationItem=(categoryId:number):NavigationItemDraft=>({categoryId,title:'',subtitle:'',iconText:'',appId:'',path:'',color:'',sort:0,enabled:true,operatingStatus:'open'})

function Navigation(){
	const dialog=useAdminDialog()
	const[data,setData]=useState<NavigationCategory[]>([])
	const[error,setError]=useState('')
	const[notice,setNotice]=useState('')
	const[loading,setLoading]=useState(false)
	const[busy,setBusy]=useState<string>('')
	const[itemDraft,setItemDraft]=useState<NavigationItemDraft|null>(null)
	const load=useCallback(async()=>{setLoading(true);try{setData(await adminAPI.navigation());setError('')}catch(e){setError(String(e))}finally{setLoading(false)}},[])
	useEffect(()=>{void load()},[load])
	const createCategory=async()=>{
		const name=await dialog.prompt({title:'新增导航分类',description:'分类将显示在普通校园服务导航中。',inputLabel:'分类名称',placeholder:'例如：校内服务',requiredMessage:'分类名称不能为空',confirmLabel:'创建分类'})
		if(!name)return
		setBusy('category-new');setError('');setNotice('')
		try{await adminAPI.createNavigationCategory({name,sort:(data.length+1)*10,enabled:true});setNotice(`已创建分类“${name}”。`);await load()}catch(e){setError(String(e))}finally{setBusy('')}
	}
	const editCategory=async(category:NavigationCategory)=>{
		const name=await dialog.prompt({title:'编辑导航分类',description:'修改分类名称不会改变其现有入口。',inputLabel:'分类名称',initialValue:category.name,requiredMessage:'分类名称不能为空',confirmLabel:'保存分类'})
		if(!name)return
		setBusy(`category-${category.id}`);setError('');setNotice('')
		try{await adminAPI.updateNavigationCategory(category.id,{name,sort:category.sort,enabled:category.enabled});setNotice(`已更新分类“${name}”。`);await load()}catch(e){setError(String(e))}finally{setBusy('')}
	}
	const toggleCategory=async(category:NavigationCategory)=>{
		setBusy(`category-${category.id}`);setError('');setNotice('')
		try{await adminAPI.updateNavigationCategory(category.id,{name:category.name,sort:category.sort,enabled:!category.enabled});setNotice(`分类“${category.name}”已${category.enabled?'停用':'启用'}。`);await load()}catch(e){setError(String(e))}finally{setBusy('')}
	}
	const deleteCategory=async(category:NavigationCategory)=>{
		if(!await dialog.confirm({title:'删除导航分类',description:`确认删除“${category.name}”吗？请先确认该分类下 ${category.items.length} 个入口均可移除。`,confirmLabel:'确认删除',tone:'danger'}))return
		setBusy(`category-${category.id}`);setError('');setNotice('')
		try{await adminAPI.deleteNavigationCategory(category.id);setNotice(`分类“${category.name}”已删除。`);await load()}catch(e){setError(String(e))}finally{setBusy('')}
	}
	const editItem=(item:NavigationItem)=>setItemDraft({id:item.id,categoryId:item.categoryId,title:item.title,subtitle:item.subtitle||'',iconText:item.iconText||'',appId:item.appId,path:item.path,color:item.color||'',sort:item.sort,enabled:item.enabled,operatingStatus:item.operatingStatus||'open'})
	const saveItem=async()=>{
		if(!itemDraft)return
		if(!itemDraft.title.trim()||!itemDraft.appId.trim()||!itemDraft.path.trim()){setError('入口名称、AppID 和 Path 均不能为空。');return}
		setBusy(itemDraft.id?`item-${itemDraft.id}`:'item-new');setError('');setNotice('')
		const{id,...draft}=itemDraft
		const input={...draft,title:draft.title.trim(),appId:draft.appId.trim(),path:draft.path.trim()}
		try{if(id)await adminAPI.updateNavigationItem(id,input);else await adminAPI.createNavigationItem(input);setNotice(`入口“${input.title}”已${id?'更新':'创建'}。`);setItemDraft(null);await load()}catch(e){setError(String(e))}finally{setBusy('')}
	}
	const deleteItem=async(item:NavigationItem)=>{
		if(!await dialog.confirm({title:'删除导航入口',description:`确认删除“${item.title}”吗？删除后用户端将无法再从该入口跳转。`,confirmLabel:'确认删除',tone:'danger'}))return
		setBusy(`item-${item.id}`);setError('');setNotice('')
		try{await adminAPI.deleteNavigationItem(item.id);setNotice(`入口“${item.title}”已删除。`);await load()}catch(e){setError(String(e))}finally{setBusy('')}
	}
	return <div className="navigation-console">
		<ExternalTakeoutMerchants/>
		<AdminPanel title="普通导航入口" description="仅维护后端已经支持的 AppID、Path 与展示顺序。" actions={<><AdminButton tone="ghost" loading={loading} onClick={()=>void load()}>刷新</AdminButton><AdminButton tone="primary" loading={busy==='category-new'} onClick={()=>void createCategory()}>新增分类</AdminButton></>}>
			{error&&<AdminNotice tone="danger" title="导航操作失败">{error}</AdminNotice>}{notice&&<AdminNotice tone="success">{notice}</AdminNotice>}
			{itemDraft&&<form className="navigation-item-form" onSubmit={event=>{event.preventDefault();void saveItem()}}>
				<div className="navigation-item-form__title"><div><strong>{itemDraft.id?'编辑导航入口':'新增导航入口'}</strong><span>只填写真实可用的小程序跳转信息</span></div><AdminButton tone="text" onClick={()=>setItemDraft(null)}>取消</AdminButton></div>
				<div className="navigation-item-form__grid"><AdminSelect label="所属分类" value={itemDraft.categoryId} onChange={event=>setItemDraft(current=>current?{...current,categoryId:Number(event.target.value)}:current)}>{data.filter(category=>!category.systemKey).map(category=><option value={category.id} key={category.id}>{category.name}</option>)}</AdminSelect><AdminField label="入口名称" required value={itemDraft.title} onChange={event=>setItemDraft(current=>current?{...current,title:event.target.value}:current)}/><AdminField label="AppID" required value={itemDraft.appId} onChange={event=>setItemDraft(current=>current?{...current,appId:event.target.value}:current)}/><AdminField label="Path" required value={itemDraft.path} onChange={event=>setItemDraft(current=>current?{...current,path:event.target.value}:current)}/><AdminField label="排序" type="number" value={itemDraft.sort} onChange={event=>setItemDraft(current=>current?{...current,sort:Number(event.target.value)}:current)}/><label className="admin-check"><input type="checkbox" checked={itemDraft.enabled} onChange={event=>setItemDraft(current=>current?{...current,enabled:event.target.checked}:current)}/><span>启用入口</span></label></div>
				<div className="navigation-item-form__footer"><AdminButton tone="primary" type="submit" loading={busy==='item-new'||busy===`item-${itemDraft.id}`}>保存入口</AdminButton></div>
			</form>}
			{!loading&&data.length===0?<AdminState kind="empty" title="暂无普通导航分类" description="创建分类后即可添加真实服务入口。"/>:<div className="navigation-category-list">{data.map(category=><section className="navigation-category" key={category.id}><header><div><h3>{category.name}</h3><span>排序 {category.sort} · {category.items.length} 个入口</span></div><div className="navigation-category__actions"><AdminStatusBadge value={category.enabled?'enabled':'disabled'}/>{!category.systemKey&&<><AdminButton tone="text" disabled={Boolean(busy)} onClick={()=>void editCategory(category)}>编辑</AdminButton><AdminButton tone="text" disabled={Boolean(busy)} onClick={()=>void toggleCategory(category)}>{category.enabled?'停用':'启用'}</AdminButton><AdminButton tone="danger" disabled={Boolean(busy)} onClick={()=>void deleteCategory(category)}>删除</AdminButton><AdminButton tone="primary" disabled={Boolean(busy)} onClick={()=>setItemDraft(emptyNavigationItem(category.id))}>新增入口</AdminButton></>}</div></header><AdminTableFrame label={`${category.name}导航入口`}><table className="admin-data-table"><thead><tr><th>名称</th><th>AppID</th><th>Path</th><th>排序</th><th>状态</th><th className="admin-table-actions">操作</th></tr></thead><tbody>{category.items.map(item=><tr key={item.id}><td><strong>{item.title}</strong><span>#{item.id}</span></td><td><code>{item.appId}</code></td><td className="admin-cell-wrap"><code>{item.path}</code></td><td>{item.sort}</td><td><AdminStatusBadge value={item.enabled?'enabled':'disabled'}/></td><td className="admin-table-actions"><AdminRowActions><AdminButton tone="text" disabled={Boolean(busy)||Boolean(category.systemKey)} onClick={()=>editItem(item)}>编辑</AdminButton><AdminButton tone="danger" disabled={Boolean(busy)||Boolean(category.systemKey)} onClick={()=>void deleteItem(item)}>删除</AdminButton></AdminRowActions></td></tr>)}{category.items.length===0&&<tr><td colSpan={6}><div className="admin-inline-empty">该分类暂无入口</div></td></tr>}</tbody></table></AdminTableFrame></section>)}</div>}
		</AdminPanel>
	</div>
}

async function takedownReason(dialog:AdminDialogAPI,kind:string,title:string){return dialog.prompt({title:`下架${kind}`,description:`确认下架“${title}”。下架后该内容将从用户端消失，恢复需重新提交审核。`,inputLabel:'下架原因',placeholder:'请输入本次操作原因',requiredMessage:'下架原因不能为空',confirmLabel:'确认下架',tone:'danger'})}

function Content(){const section=useAdminStore(s=>s.section);if(section==='dashboard')return <Dashboard/>;if(section==='users')return <Users/>;if(section==='identities')return <IdentityApplications/>;if(section==='moderation')return <Reviews/>;if(section==='media')return <MediaConsole/>;if(section==='delivery')return <DeliveryConsole/>;if(section==='takeout'&&legacyTakeoutEnabled)return <TakeoutConsole/>;if(section==='community')return <Posts/>;if(section==='trade')return <Products/>;if(section==='reports')return <Reports/>;if(section==='feedbacks')return <Feedbacks/>;return <Navigation/>}

function AdminShell(){const section=useAdminStore(s=>s.section);const setSection=useAdminStore(s=>s.setSection);const active=visibleNavigation.find(item=>item.key===section)||adminNavigation[0];return <div className="app-shell"><aside className="sidebar"><div className="brand"><div className="brand__logo">湛</div><div><strong>湛科生活圈</strong><span>运营管理后台</span></div></div><nav className="admin-navigation" aria-label="后台主导航">{adminNavigationGroups.map(group=><section className="admin-navigation__group" key={group}><h2>{group}</h2>{visibleNavigation.filter(item=>item.group===group).map(item=><button key={item.key} type="button" className={section===item.key?'active':''} aria-current={section===item.key?'page':undefined} onClick={()=>setSection(item.key)}><span className="admin-navigation__icon"><AdminIcon name={item.icon}/></span><span className="admin-navigation__label">{item.label}</span></button>)}</section>)}</nav><footer className="sidebar__footer"><span className="sidebar__status"/>内部运营系统</footer></aside><main className="main"><div className="topbar"><AdminPageHeader eyebrow={active.group} title={active.label} description={active.description}/><AccountPanel/></div><div className="main__content"><Content/></div></main></div>}

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
