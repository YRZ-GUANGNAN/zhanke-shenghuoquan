import { useCallback, useEffect, useState } from 'react'
import { adminAPI } from './services/f-admin'
import type { MediaAssetDetail, MediaAssetRow, MediaDeadletters, MediaDecisionRow, MediaEventRow, MediaGCJobRow, MediaQuery, MediaSummary, Page } from './types'

type View = 'assets' | 'decisions' | 'events' | 'gc' | 'deadletters'
const emptyPage=<T,>():Page<T>=>({items:[],page:1,pageSize:20,total:0})
const displayTime=(value?:string)=>value?new Date(value).toLocaleString('zh-CN'):'—'
const short=(value:string)=>value.length>18?`${value.slice(0,8)}…${value.slice(-6)}`:value
const purposeLabels:Record<string,string>={post:'帖子图片',product:'二手商品',avatar:'头像',profile_cover:'个人封面',identity_application:'身份申请材料',dish:'菜品图片',merchant_logo:'商家标志',delivery_proof:'配送凭证',delivery_im:'配送沟通图片',feedback:'反馈截图'}
const reviewLabels:Record<string,string>={pending:'待处理',submitted:'已提交',reviewing:'自动检测中',approved:'已通过',rejected:'未通过',review_failed:'检测异常',not_required:'无需单独图片审核'}

export function MediaConsole(){
	const[view,setView]=useState<View>('assets')
	const[summary,setSummary]=useState<MediaSummary|null>(null)
	const[assets,setAssets]=useState<Page<MediaAssetRow>>(emptyPage())
	const[decisions,setDecisions]=useState<Page<MediaDecisionRow>>(emptyPage())
	const[events,setEvents]=useState<Page<MediaEventRow>>(emptyPage())
	const[gc,setGC]=useState<Page<MediaGCJobRow>>(emptyPage())
	const[deadletters,setDeadletters]=useState<MediaDeadletters|null>(null)
	const[detail,setDetail]=useState<MediaAssetDetail|null>(null)
	const[assetId,setAssetId]=useState('')
	const[status,setStatus]=useState('')
	const[purpose,setPurpose]=useState('')
	const[reviewStatus,setReviewStatus]=useState('')
	const[taskId,setTaskId]=useState('')
	const[from,setFrom]=useState('')
	const[to,setTo]=useState('')
	const[page,setPage]=useState(1)
	const[filters,setFilters]=useState<MediaQuery>({})
	const[error,setError]=useState('')
	const[loading,setLoading]=useState(false)

	const load=useCallback(async()=>{
		setLoading(true);setError('')
		const query:MediaQuery={...filters,page,pageSize:50}
		if(view!=='assets'){delete query.purpose;delete query.reviewStatus}
		try{
			setSummary(await adminAPI.mediaSummary())
			if(view==='assets')setAssets(await adminAPI.mediaAssets(query))
			if(view==='decisions')setDecisions(await adminAPI.mediaDecisions(query))
			if(view==='events')setEvents(await adminAPI.mediaEvents(query))
			if(view==='gc')setGC(await adminAPI.mediaGCJobs(query))
			if(view==='deadletters')setDeadletters(await adminAPI.mediaDeadletters(query))
		}catch(reason){setError(reason instanceof Error?reason.message:String(reason))}finally{setLoading(false)}
	},[filters,page,view])

	useEffect(()=>{void load()},[load])
	const inspect=async(id:string)=>{setError('');try{setDetail(await adminAPI.mediaAsset(id))}catch(reason){setError(reason instanceof Error?reason.message:String(reason))}}
	const applyFilters=()=>{setPage(1);setFilters({assetId:assetId.trim(),status,purpose,reviewStatus,taskId:taskId.trim(),from:from?new Date(from).toISOString():'',to:to?new Date(to).toISOString():''})}
	const clearFilters=()=>{setAssetId('');setTaskId('');setStatus('');setPurpose('');setReviewStatus('');setFrom('');setTo('');setPage(1);setFilters({})}
	return <div className="media-console">
		<section className="media-summary">{[['媒体资产',summary?.assets],['待审核',summary?.pendingReview],['消费死信',summary?.consumerDeadletters],['GC 死信',summary?.gcDeadletters]].map(([label,value])=><article key={label}><span>{label}</span><strong>{value??'—'}</strong></article>)}</section>
		<section className="panel media-query">
			<div className="media-tabs">{([['assets','资产'],['decisions','决策历史'],['events','事件'],['gc','GC 历史'],['deadletters','死信']] as Array<[View,string]>).map(([key,label])=><button className={view===key?'active':''} key={key} onClick={()=>{setStatus('');setFilters(current=>({...current,status:''}));setPage(1);setView(key)}}>{label}</button>)}</div>
			<div className="media-filters"><label>资产号<input value={assetId} onChange={event=>setAssetId(event.target.value.toUpperCase())} placeholder="26 位 ULID"/></label><label>任务号<input value={taskId} onChange={event=>setTaskId(event.target.value)} inputMode="numeric" placeholder="Review Task ID"/></label><label>状态<input value={status} onChange={event=>setStatus(event.target.value)} placeholder="当前视图状态"/></label><label>开始时间<input type="datetime-local" value={from} onChange={event=>setFrom(event.target.value)}/></label><label>结束时间<input type="datetime-local" value={to} onChange={event=>setTo(event.target.value)}/></label><button onClick={applyFilters} disabled={loading}>{loading?'查询中…':'查询'}</button><button className="secondary" onClick={clearFilters}>清空</button></div>
			{error&&<div className="media-error">{error}</div>}
			{view==='assets'&&<div className="media-filters"><label>图片用途<select value={purpose} onChange={event=>setPurpose(event.target.value)}><option value=''>全部用途</option>{Object.entries(purposeLabels).map(([value,label])=><option key={value} value={value}>{label}</option>)}</select></label><label>图片审核<select value={reviewStatus} onChange={event=>setReviewStatus(event.target.value)}><option value=''>全部状态</option>{Object.entries(reviewLabels).map(([value,label])=><option key={value} value={value}>{label}</option>)}</select></label><span>头像、封面和身份材料不走云图片审核；商家 Logo、菜品图随商家/菜品人工审核决定展示。帖子、二手商品仍走自动审核。</span></div>}
			{view==='assets'&&<AssetTable page={assets} inspect={inspect}/>} {view==='decisions'&&<DecisionTable page={decisions}/>} {view==='events'&&<EventTable page={events}/>} {view==='gc'&&<GCTable page={gc}/>} {view==='deadletters'&&<DeadletterTables value={deadletters}/>} 
			{view!=='deadletters'&&<Pagination page={page} total={view==='assets'?assets.total:view==='decisions'?decisions.total:view==='events'?events.total:gc.total} setPage={setPage}/>} 
		</section>
		{detail&&<Detail value={detail} close={()=>setDetail(null)}/>} 
	</div>
}

function AssetTable({page,inspect}:{page:Page<MediaAssetRow>;inspect:(id:string)=>void}){return <><div className="media-result">共 {page.total} 条</div><div className="table-scroll"><table><thead><tr><th>资产</th><th>用途</th><th>技术状态</th><th>审核</th><th>上传链路</th><th>引用</th><th>规格</th><th>更新时间</th></tr></thead><tbody>{page.items.map(row=><tr key={row.assetId}><td><button className="link-button" onClick={()=>inspect(row.assetId)}>{short(row.assetId)}</button><span>Owner #{row.ownerId}</span></td><td>{row.purpose}</td><td><Badge value={row.technicalStatus}/><span>{row.storageStage}</span></td><td><Badge value={row.reviewStatus}/><span>{row.reviewTaskId?`Task #${row.reviewTaskId}`:'无任务'}</span></td><td>{row.ticketStatus||'—'}<span>{row.completionPhase||'—'}</span></td><td>{row.referenceCount}<span>阻断 {row.blockingReferences}</span></td><td>{row.width}×{row.height}<span>{formatBytes(row.byteSize)}</span></td><td>{displayTime(row.updatedAt)}</td></tr>)}</tbody></table></div></>}
function DecisionTable({page}:{page:Page<MediaDecisionRow>}){return <><div className="media-result">共 {page.total} 条</div><div className="table-scroll"><table><thead><tr><th>资产</th><th>任务</th><th>版本</th><th>决策</th><th>结果码</th><th>时间</th></tr></thead><tbody>{page.items.map(row=><tr key={`${row.projectionId}-${row.decisionVersion}`}><td>{short(row.assetId)}</td><td>#{row.reviewTaskId}</td><td>v{row.decisionVersion}</td><td><Badge value={row.decision}/></td><td>{row.resultCode||'—'}</td><td>{displayTime(row.receivedAt)}</td></tr>)}</tbody></table></div></>}
function EventTable({page}:{page:Page<MediaEventRow>}){return <><div className="media-result">共 {page.total} 条</div><div className="table-scroll"><table><thead><tr><th>事件</th><th>聚合</th><th>操作</th><th>消费状态</th><th>尝试</th><th>错误</th><th>时间</th></tr></thead><tbody>{page.items.map(row=><tr key={row.eventId}><td>{short(row.eventId)}<span>v{row.aggregateVersion}</span></td><td>{short(row.aggregateId)}</td><td>{row.operation}</td><td><Badge value={row.consumerStatus}/></td><td>{row.attemptCount}</td><td>{row.lastErrorCode||'—'}</td><td>{displayTime(row.createdAt)}</td></tr>)}</tbody></table></div></>}
function GCTable({page}:{page:Page<MediaGCJobRow>}){return <><div className="media-result">共 {page.total} 条</div><div className="table-scroll"><table><thead><tr><th>Job</th><th>资产</th><th>类型</th><th>状态</th><th>尝试</th><th>错误</th><th>完成时间</th></tr></thead><tbody>{page.items.map(row=><tr key={`${row.kind}-${row.jobId}`}><td>{short(row.jobId)}</td><td>{short(row.assetId)}</td><td>{row.kind}{row.objectStage?` / ${row.objectStage}`:''}</td><td><Badge value={row.status}/></td><td>{row.attemptCount}</td><td>{row.lastErrorCode||'—'}</td><td>{displayTime(row.deletedAt)}</td></tr>)}</tbody></table></div></>}
function DeadletterTables({value}:{value:MediaDeadletters|null}){if(!value)return null;return <><h3>消费死信</h3><EventTable page={value.consumer}/><h3 className="media-subtitle">GC 死信</h3><GCTable page={value.gc}/></>}
function Detail({value,close}:{value:MediaAssetDetail;close:()=>void}){return <section className="panel media-detail"><header><div><span>媒体资产详情</span><h2>{value.asset.assetId}</h2></div><button onClick={close}>关闭</button></header><div className="detail-grid"><Fact label="技术状态" value={value.asset.technicalStatus}/><Fact label="审核状态" value={value.projection?.status||'unavailable'}/><Fact label="Ticket" value={value.ticket?`${value.ticket.status} / ${short(value.ticket.ticketId)}`:'—'}/><Fact label="Completion" value={value.completion?.phase||'—'}/><Fact label="引用事实" value={`${value.references.length} 条，阻断 ${value.asset.blockingReferences}`}/><Fact label="GC Job" value={`${value.gcJobs.length} 条`}/></div>{value.decisions.length>0&&<><h3>审核决策历史</h3><DecisionTable page={{items:value.decisions,page:1,pageSize:value.decisions.length,total:value.decisions.length}}/></>}{value.events.length>0&&<><h3>事件历史</h3><EventTable page={{items:value.events,page:1,pageSize:value.events.length,total:value.events.length}}/></>}</section>}
function Fact({label,value}:{label:string;value:string}){return <div><span>{label}</span><strong>{value}</strong></div>}
function Badge({value}:{value:string}){return <span className={`media-badge media-badge--${value}`}>{reviewLabels[value]||value}</span>}
function Pagination({page,total,setPage}:{page:number;total:number;setPage:(value:number)=>void}){const pages=Math.max(1,Math.ceil(total/50));return <div className="media-pagination"><button disabled={page<=1} onClick={()=>setPage(page-1)}>上一页</button><span>第 {page} / {pages} 页</span><button disabled={page>=pages} onClick={()=>setPage(page+1)}>下一页</button></div>}
function formatBytes(value:number){if(value<1024)return `${value} B`;if(value<1024*1024)return `${(value/1024).toFixed(1)} KB`;return `${(value/1024/1024).toFixed(1)} MB`}
