import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { TakeoutAPIError, takeoutAdminAPI } from './services/takeout-admin'
import { useAdminDialog } from './components'
import type { DishCategoryRow, DishRow, FeeConfigRow, MerchantRow, OrderDetail, OrderRow, Page, RiderConfigRow, RiderRow, RiderStatus, SettlementRow, TakeoutQuery, WithdrawRow, WithdrawStatus } from './types'

type View='merchants'|'catalog'|'orders'|'riders'|'fee'|'rider-config'|'settlements'|'withdrawals'
const pageSize=20
const emptyPage=<T,>():Page<T>=>({items:[],page:1,pageSize,total:0})
const money=(value:number)=>`¥${(value/100).toFixed(2)}`
const time=(value?:string)=>value?new Date(value).toLocaleString('zh-CN'):'—'
const errorMessage=(reason:unknown)=>reason instanceof Error?reason.message:String(reason)
const conflictMessage='数据已被修改，请刷新后重试'

export function TakeoutConsole(){
	const[view,setView]=useState<View>('merchants')
	const tabs:Array<[View,string]>=[['merchants','商家管理'],['catalog','菜品/分类'],['orders','订单管理'],['riders','骑手管理'],['fee','费率配置'],['rider-config','在途上限'],['settlements','结算报表'],['withdrawals','提现处理']]
	return <div className="takeout-console">
		<section className="panel takeout-tabs" aria-label="外卖后台模块">{tabs.map(([key,label])=><button key={key} className={view===key?'active':''} onClick={()=>setView(key)}>{label}</button>)}</section>
		{view==='merchants'&&<Merchants/>}
		{view==='catalog'&&<Catalog/>}
		{view==='orders'&&<Orders/>}
		{view==='riders'&&<Riders/>}
		{view==='fee'&&<FeeConfig/>}
		{view==='rider-config'&&<RiderConfig/>}
		{view==='settlements'&&<Settlements/>}
		{view==='withdrawals'&&<Withdrawals/>}
	</div>
}

function Panel({title,actions,children}:{title:string;actions?:ReactNode;children:ReactNode}){return <section className="panel takeout-panel"><header className="takeout-header"><h2>{title}</h2>{actions}</header>{children}</section>}
function Badge({value}:{value:string}){return <span className={`takeout-badge takeout-badge--${value}`}>{value}</span>}
function ErrorNotice({error,conflict,refresh}:{error:string;conflict:boolean;refresh:()=>void}){if(!error)return null;return <div className={`takeout-notice ${conflict?'takeout-notice--conflict':''}`} role="alert"><strong>{conflict?conflictMessage:error}</strong>{conflict&&<button onClick={refresh}>刷新最新数据</button>}</div>}
function Pagination({value,setPage}:{value:Page<unknown>;setPage:(page:number)=>void}){const pages=Math.max(1,Math.ceil(value.total/value.pageSize));return <div className="takeout-pagination"><button disabled={value.page<=1} onClick={()=>setPage(value.page-1)}>上一页</button><span>第 {value.page} / {pages} 页，共 {value.total} 条</span><button disabled={value.page>=pages} onClick={()=>setPage(value.page+1)}>下一页</button></div>}
function Detail({title,close,children}:{title:string;close:()=>void;children:ReactNode}){return <aside className="takeout-detail"><header><h3>{title}</h3><button onClick={close}>关闭</button></header>{children}</aside>}
function Fact({label,value}:{label:string;value:ReactNode}){return <div className="takeout-fact"><span>{label}</span><strong>{value}</strong></div>}

function Merchants(){
	const dialog=useAdminDialog()
	const[data,setData]=useState<Page<MerchantRow>>(emptyPage())
	const[status,setStatus]=useState('')
	const[page,setPage]=useState(1)
	const[detail,setDetail]=useState<MerchantRow|null>(null)
	const[error,setError]=useState('')
	const[conflict,setConflict]=useState(false)
	const[loading,setLoading]=useState(false)
	const[busy,setBusy]=useState('')
	const load=useCallback(async()=>{setLoading(true);setError('');setConflict(false);try{setData(await takeoutAdminAPI.merchants({status,page,pageSize}))}catch(reason){setError(errorMessage(reason))}finally{setLoading(false)}},[status,page])
	useEffect(()=>{void load()},[load])
	const inspect=async(id:string)=>{setError('');try{setDetail(await takeoutAdminAPI.merchant(id))}catch(reason){setError(errorMessage(reason))}}
	const act=async(action:'suspend'|'unsuspend')=>{
		if(!detail)return
		const label=action==='suspend'?'封禁':'解封'
		if(!await dialog.confirm({title:`${label}商家`,description:`确认${label}商家“${detail.name}”？该操作会立即影响商家工作台状态。`,confirmLabel:`确认${label}`,tone:action==='suspend'?'danger':'default'}))return
		setBusy(action);setError('');setConflict(false)
		try{
			const updated=action==='suspend'?await takeoutAdminAPI.suspendMerchant(detail.merchantPublicId,detail.status,detail.rowVersion):await takeoutAdminAPI.unsuspendMerchant(detail.merchantPublicId,detail.rowVersion)
			setDetail(updated);await load()
		}catch(reason){if(reason instanceof TakeoutAPIError&&reason.status===409){setConflict(true);setError(conflictMessage)}else setError(errorMessage(reason))}finally{setBusy('')}
	}
	const refresh=async()=>{await load();if(detail)await inspect(detail.merchantPublicId)}
	return <Panel title="商家管理" actions={<div className="takeout-controls"><label>G 域状态（F 审核结果）<select value={status} onChange={event=>{setPage(1);setStatus(event.target.value)}}><option value="">全部</option>{['pending_review','active','resting','suspended','rejected'].map(value=><option key={value}>{value}</option>)}</select></label><button onClick={()=>void load()} disabled={loading}>{loading?'加载中…':'刷新'}</button></div>}>
		<ErrorNotice error={error} conflict={conflict} refresh={()=>void refresh()}/><div className="table-scroll"><table><thead><tr><th>商家</th><th>Owner</th><th>位置/营业时间</th><th>状态</th><th>版本</th></tr></thead><tbody>{data.items.map(row=><tr key={row.merchantPublicId}><td><button className="link-button" onClick={()=>void inspect(row.merchantPublicId)}>{row.name}</button><span>{row.merchantPublicId}</span></td><td>#{row.ownerUserId}</td><td>{row.location||'—'}<span>{row.businessHours||'—'}</span></td><td><Badge value={row.status}/></td><td>v{row.rowVersion}</td></tr>)}</tbody></table></div><Pagination value={data} setPage={setPage}/>
		{detail&&<Detail title={detail.name} close={()=>setDetail(null)}><div className="detail-grid"><Fact label="业务状态" value={<Badge value={detail.status}/>}/><Fact label="联系电话" value={detail.contactPhone||'—'}/><Fact label="位置" value={detail.location||'—'}/><Fact label="营业时间" value={detail.businessHours||'—'}/><Fact label="Row Version" value={`v${detail.rowVersion}`}/><Fact label="更新时间" value={time(detail.updatedAt)}/></div><div className="takeout-actions">{(detail.status==='active'||detail.status==='resting')&&<button className="danger" disabled={Boolean(busy)} onClick={()=>void act('suspend')}>{busy==='suspend'?'处理中…':'封禁商家'}</button>}{detail.status==='suspended'&&<button disabled={Boolean(busy)} onClick={()=>void act('unsuspend')}>{busy==='unsuspend'?'处理中…':'解除封禁'}</button>}</div></Detail>}
	</Panel>
}

function Catalog(){
	const[kind,setKind]=useState<'dishes'|'categories'>('dishes')
	const[data,setData]=useState<Page<DishRow|DishCategoryRow>>(emptyPage())
	const[detail,setDetail]=useState<DishRow|DishCategoryRow|null>(null)
	const[merchantInput,setMerchantInput]=useState('')
	const[categoryInput,setCategoryInput]=useState('')
	const[statusInput,setStatusInput]=useState('')
	const[filters,setFilters]=useState<TakeoutQuery>({})
	const[page,setPage]=useState(1)
	const[error,setError]=useState('')
	const[loading,setLoading]=useState(false)
	const load=useCallback(async()=>{setLoading(true);setError('');try{const query={...filters,page,pageSize};setData(kind==='dishes'?await takeoutAdminAPI.dishes(query):await takeoutAdminAPI.categories(query))}catch(reason){setError(errorMessage(reason))}finally{setLoading(false)}},[filters,kind,page])
	useEffect(()=>{void load()},[load])
	const apply=()=>{setPage(1);setDetail(null);setFilters({merchantId:merchantInput.trim(),categoryId:kind==='dishes'?categoryInput.trim():'',status:kind==='dishes'?statusInput:''})}
	const inspect=async(id:string)=>{setError('');try{setDetail(kind==='dishes'?await takeoutAdminAPI.dish(id):await takeoutAdminAPI.category(id))}catch(reason){setError(errorMessage(reason))}}
	const items=data.items as Array<DishRow|DishCategoryRow>
	return <Panel title="菜品 / 分类管理" actions={<div className="takeout-subtabs"><button className={kind==='dishes'?'active':''} onClick={()=>{setKind('dishes');setPage(1);setDetail(null)}}>菜品</button><button className={kind==='categories'?'active':''} onClick={()=>{setKind('categories');setPage(1);setDetail(null)}}>分类</button></div>}>
		<div className="takeout-controls"><label>商家 ID<input value={merchantInput} onChange={event=>setMerchantInput(event.target.value.toUpperCase())}/></label>{kind==='dishes'&&<><label>分类 ID<input value={categoryInput} onChange={event=>setCategoryInput(event.target.value.toUpperCase())}/></label><label>菜品状态<select value={statusInput} onChange={event=>setStatusInput(event.target.value)}><option value="">全部</option>{['draft','on_shelf','sold_out','off_shelf','removed'].map(value=><option key={value}>{value}</option>)}</select></label></>}<button onClick={apply} disabled={loading}>{loading?'查询中…':'查询'}</button><button className="secondary" onClick={()=>{setMerchantInput('');setCategoryInput('');setStatusInput('');setPage(1);setFilters({})}}>清空</button></div>
		<ErrorNotice error={error} conflict={false} refresh={()=>void load()}/><div className="table-scroll"><table><thead>{kind==='dishes'?<tr><th>菜品</th><th>商家/分类</th><th>价格</th><th>状态</th><th>排序</th></tr>:<tr><th>分类</th><th>商家</th><th>排序</th><th>版本</th></tr>}</thead><tbody>{items.map(row=>kind==='dishes'?<DishTableRow key={(row as DishRow).dishPublicId} value={row as DishRow} inspect={inspect}/>:<CategoryTableRow key={(row as DishCategoryRow).categoryPublicId} value={row as DishCategoryRow} inspect={inspect}/>)}</tbody></table></div><Pagination value={data as Page<unknown>} setPage={setPage}/>
		{detail&&('dishPublicId' in detail?<Detail title={detail.name} close={()=>setDetail(null)}><div className="detail-grid"><Fact label="菜品 ID" value={detail.dishPublicId}/><Fact label="商家" value={detail.merchantPublicId}/><Fact label="分类" value={detail.categoryPublicId||'—'}/><Fact label="价格" value={money(detail.priceCents)}/><Fact label="状态" value={<Badge value={detail.status}/>}/><Fact label="Row Version" value={`v${detail.rowVersion}`}/></div><p className="takeout-description">{detail.description||'无描述'}</p></Detail>:<Detail title={detail.name} close={()=>setDetail(null)}><div className="detail-grid"><Fact label="分类 ID" value={detail.categoryPublicId}/><Fact label="商家" value={detail.merchantPublicId}/><Fact label="排序" value={detail.sortOrder}/><Fact label="Row Version" value={`v${detail.rowVersion}`}/></div></Detail>)}
	</Panel>
}

function DishTableRow({value,inspect}:{value:DishRow;inspect:(id:string)=>void}){return <tr><td><button className="link-button" onClick={()=>void inspect(value.dishPublicId)}>{value.name}</button><span>{value.dishPublicId}</span></td><td>{value.merchantPublicId}<span>{value.categoryPublicId||'未分类'}</span></td><td>{money(value.priceCents)}</td><td><Badge value={value.status}/></td><td>{value.sortOrder}</td></tr>}
function CategoryTableRow({value,inspect}:{value:DishCategoryRow;inspect:(id:string)=>void}){return <tr><td><button className="link-button" onClick={()=>void inspect(value.categoryPublicId)}>{value.name}</button><span>{value.categoryPublicId}</span></td><td>{value.merchantPublicId}</td><td>{value.sortOrder}</td><td>v{value.rowVersion}</td></tr>}

function Orders(){
	const[data,setData]=useState<Page<OrderRow>>(emptyPage())
	const[detail,setDetail]=useState<OrderDetail|null>(null)
	const[status,setStatus]=useState('')
	const[page,setPage]=useState(1)
	const[error,setError]=useState('')
	const[loading,setLoading]=useState(false)
	const load=useCallback(async()=>{setLoading(true);setError('');try{setData(await takeoutAdminAPI.orders({status,page,pageSize}))}catch(reason){setError(errorMessage(reason))}finally{setLoading(false)}},[status,page])
	useEffect(()=>{void load()},[load])
	const inspect=async(id:string)=>{setError('');try{setDetail(await takeoutAdminAPI.order(id))}catch(reason){setError(errorMessage(reason))}}
	return <Panel title="订单管理" actions={<div className="takeout-controls"><label>状态<select value={status} onChange={event=>{setPage(1);setStatus(event.target.value)}}><option value="">全部</option>{['pending_payment','paid','accepted','preparing','ready_for_pickup','delivering','completed','cancelled','refunded'].map(value=><option key={value}>{value}</option>)}</select></label><button disabled={loading} onClick={()=>void load()}>{loading?'加载中…':'刷新'}</button></div>}>
		<ErrorNotice error={error} conflict={false} refresh={()=>void load()}/><div className="table-scroll"><table><thead><tr><th>订单</th><th>买家/商家</th><th>金额</th><th>状态</th><th>创建时间</th></tr></thead><tbody>{data.items.map(row=><tr key={row.orderPublicId}><td><button className="link-button" onClick={()=>void inspect(row.orderPublicId)}>{row.orderPublicId}</button></td><td>买家 #{row.buyerUserId}<span>{row.merchantPublicId}</span></td><td>{money(row.totalAmountCents)}</td><td><Badge value={row.status}/></td><td>{time(row.createdAt)}</td></tr>)}</tbody></table></div><Pagination value={data} setPage={setPage}/>
		{detail&&<OrderDetailPanel value={detail} close={()=>setDetail(null)}/>}
	</Panel>
}

function OrderDetailPanel({value,close}:{value:OrderDetail;close:()=>void}){const order=value.order;const stages:Array<[string,string|undefined]>=[['已支付',order.paidAt],['已接单',order.acceptedAt],['备餐中',order.preparingAt],['待取餐',order.readyAt],['配送中',order.deliveringAt],['已完成',order.completedAt],['已取消',order.cancelledAt],['已退款',order.refundedAt]];return <Detail title={`订单 ${order.orderPublicId}`} close={close}><div className="detail-grid"><Fact label="状态" value={<Badge value={order.status}/>}/><Fact label="商品金额" value={money(order.itemsAmountCents)}/><Fact label="配送费" value={money(order.deliveryFeeCents)}/><Fact label="总额" value={money(order.totalAmountCents)}/><Fact label="联系人" value={`${order.contactName} / ${order.contactPhone}`}/><Fact label="地址" value={`${order.addressBuilding} ${order.addressRoom}`}/></div><h4>订单明细</h4><table><thead><tr><th>菜品</th><th>单价</th><th>数量</th><th>小计</th></tr></thead><tbody>{value.items.map((item,index)=><tr key={`${item.dishPublicId}-${index}`}><td>{item.dishNameSnapshot}</td><td>{money(item.unitPriceCentsSnapshot)}</td><td>{item.quantity}</td><td>{money(item.lineAmountCents)}</td></tr>)}</tbody></table><h4>阶段时间</h4><div className="takeout-timeline">{stages.map(([label,value])=><div key={label}><span>{label}</span><strong>{time(value)}</strong></div>)}</div></Detail>}

function Riders(){
	const dialog=useAdminDialog()
	const[data,setData]=useState<Page<RiderRow>>(emptyPage())
	const[status,setStatus]=useState('')
	const[page,setPage]=useState(1)
	const[error,setError]=useState('')
	const[conflict,setConflict]=useState(false)
	const[busy,setBusy]=useState<number|null>(null)
	const load=useCallback(async()=>{setError('');setConflict(false);try{setData(await takeoutAdminAPI.riders({status,page,pageSize}))}catch(reason){setError(errorMessage(reason))}},[status,page])
	useEffect(()=>{void load()},[load])
	const act=async(row:RiderRow,next:RiderStatus)=>{if(!await dialog.confirm({title:'更新骑手状态',description:`确认将骑手 #${row.riderUserId} 的状态从 ${row.status} 改为 ${next}？`,confirmLabel:'确认更新',tone:next==='suspended'?'danger':'default'}))return;setBusy(row.riderUserId);setError('');setConflict(false);try{await takeoutAdminAPI.updateRiderStatus(row.riderUserId,next,row.status,row.rowVersion);await load()}catch(reason){if(reason instanceof TakeoutAPIError&&reason.status===409){setConflict(true);setError(conflictMessage)}else setError(errorMessage(reason))}finally{setBusy(null)}}
	return <Panel title="骑手管理" actions={<div className="takeout-controls"><label>状态<select value={status} onChange={event=>{setPage(1);setStatus(event.target.value)}}><option value="">全部</option>{['active','resting','suspended'].map(value=><option key={value}>{value}</option>)}</select></label><button onClick={()=>void load()}>刷新</button></div>}><ErrorNotice error={error} conflict={conflict} refresh={()=>void load()}/><div className="table-scroll"><table><thead><tr><th>骑手</th><th>联系方式</th><th>状态</th><th>版本</th><th>操作</th></tr></thead><tbody>{data.items.map(row=><tr key={row.riderUserId}><td>{row.realName}<span>#{row.riderUserId}</span></td><td>{row.phone}</td><td><Badge value={row.status}/></td><td>v{row.rowVersion}</td><td><div className="takeout-row-actions">{row.status==='active'&&<button disabled={busy===row.riderUserId} onClick={()=>void act(row,'resting')}>设为休息</button>}{row.status==='resting'&&<button disabled={busy===row.riderUserId} onClick={()=>void act(row,'active')}>恢复接单</button>}{row.status!=='suspended'&&<button className="danger" disabled={busy===row.riderUserId} onClick={()=>void act(row,'suspended')}>封禁</button>}{row.status==='suspended'&&<button disabled={busy===row.riderUserId} onClick={()=>void act(row,'active')}>解封</button>}</div></td></tr>)}</tbody></table></div><Pagination value={data} setPage={setPage}/></Panel>
}

function FeeConfig(){
	const dialog=useAdminDialog()
	const[value,setValue]=useState<FeeConfigRow|null>(null)
	const[riderFee,setRiderFee]=useState('')
	const[platformCut,setPlatformCut]=useState('')
	const[error,setError]=useState('')
	const[conflict,setConflict]=useState(false)
	const[loading,setLoading]=useState(false)
	const load=useCallback(async()=>{setError('');setConflict(false);try{const latest=await takeoutAdminAPI.feeConfig();setValue(latest);setRiderFee(String(latest.riderFeeCents));setPlatformCut(String(latest.platformCutCents))}catch(reason){setError(errorMessage(reason))}},[])
	useEffect(()=>{void load()},[load])
	const submit=async()=>{if(!value)return;if(!await dialog.confirm({title:'更新费率配置',description:`新配置：骑手配送费 ${money(Number(riderFee))}，平台抽成 ${money(Number(platformCut))}。仅影响后续订单。`,confirmLabel:'保存配置'}))return;setLoading(true);setError('');setConflict(false);try{await takeoutAdminAPI.updateFeeConfig(Number(riderFee),Number(platformCut),value.rowVersion);await load()}catch(reason){if(reason instanceof TakeoutAPIError&&reason.status===409){setConflict(true);setError(conflictMessage)}else setError(errorMessage(reason))}finally{setLoading(false)}}
	return <Panel title="费率配置"><ErrorNotice error={error} conflict={conflict} refresh={()=>void load()}/>{value?<div className="takeout-form"><label>骑手配送费（分）<input type="number" min="0" value={riderFee} onChange={event=>setRiderFee(event.target.value)}/></label><label>平台抽成（分）<input type="number" min="0" value={platformCut} onChange={event=>setPlatformCut(event.target.value)}/></label><Fact label="当前 Row Version" value={`v${value.rowVersion}`}/><Fact label="服务端更新时间" value={time(value.updatedAt)}/><button disabled={loading} onClick={()=>void submit()}>{loading?'保存中…':'保存配置'}</button><button className="secondary" disabled={loading} onClick={()=>void load()}>刷新</button></div>:<p>加载中…</p>}</Panel>
}

function RiderConfig(){
	const dialog=useAdminDialog()
	const[value,setValue]=useState<RiderConfigRow|null>(null)
	const[max,setMax]=useState('')
	const[error,setError]=useState('')
	const[conflict,setConflict]=useState(false)
	const[loading,setLoading]=useState(false)
	const load=useCallback(async()=>{setError('');setConflict(false);try{const latest=await takeoutAdminAPI.riderConfig();setValue(latest);setMax(String(latest.maxInFlightPerRider))}catch(reason){setError(errorMessage(reason))}},[])
	useEffect(()=>{void load()},[load])
	const submit=async()=>{if(!value)return;if(!await dialog.confirm({title:'更新在途订单上限',description:`每名骑手的最大在途订单将调整为 ${max} 单。`,confirmLabel:'保存配置'}))return;setLoading(true);setError('');setConflict(false);try{await takeoutAdminAPI.updateRiderConfig(Number(max),value.rowVersion);await load()}catch(reason){if(reason instanceof TakeoutAPIError&&reason.status===409){setConflict(true);setError(conflictMessage)}else setError(errorMessage(reason))}finally{setLoading(false)}}
	return <Panel title="骑手在途上限配置"><ErrorNotice error={error} conflict={conflict} refresh={()=>void load()}/>{value?<div className="takeout-form"><label>每名骑手最大在途单数<input type="number" min="1" value={max} onChange={event=>setMax(event.target.value)}/></label><Fact label="当前 Row Version" value={`v${value.rowVersion}`}/><Fact label="服务端更新时间" value={time(value.updatedAt)}/><button disabled={loading} onClick={()=>void submit()}>{loading?'保存中…':'保存配置'}</button><button className="secondary" disabled={loading} onClick={()=>void load()}>刷新</button></div>:<p>加载中…</p>}</Panel>
}

function Settlements(){
	const[data,setData]=useState<Page<SettlementRow>>(emptyPage())
	const[page,setPage]=useState(1)
	const[error,setError]=useState('')
	const load=useCallback(async()=>{setError('');try{setData(await takeoutAdminAPI.settlements({page,pageSize}))}catch(reason){setError(errorMessage(reason))}},[page])
	useEffect(()=>{void load()},[load])
	return <Panel title="待结算汇总" actions={<button onClick={()=>void load()}>刷新</button>}><ErrorNotice error={error} conflict={false} refresh={()=>void load()}/><div className="table-scroll"><table><thead><tr><th>骑手</th><th>待结算笔数</th><th>骑手应结</th><th>平台抽成</th></tr></thead><tbody>{data.items.map(row=><tr key={row.riderUserId}><td>#{row.riderUserId}</td><td>{row.pendingCount}</td><td>{money(row.riderFeeCents)}</td><td>{money(row.platformCutCents)}</td></tr>)}</tbody></table></div><Pagination value={data} setPage={setPage}/></Panel>
}

function Withdrawals(){
	const dialog=useAdminDialog()
	const[data,setData]=useState<Page<WithdrawRow>>(emptyPage())
	const[status,setStatus]=useState<WithdrawStatus|''>('pending')
	const[page,setPage]=useState(1)
	const[detail,setDetail]=useState<WithdrawRow|null>(null)
	const[error,setError]=useState('')
	const[conflict,setConflict]=useState(false)
	const[busy,setBusy]=useState('')
	const load=useCallback(async()=>{setError('');setConflict(false);try{setData(await takeoutAdminAPI.withdrawals({status,page,pageSize}))}catch(reason){setError(errorMessage(reason))}},[status,page])
	useEffect(()=>{void load()},[load])
	const inspect=async(id:string)=>{setError('');try{setDetail(await takeoutAdminAPI.withdrawal(id))}catch(reason){setError(errorMessage(reason))}}
	const run=async(action:'pay'|'reject')=>{if(!detail)return;let reason='';if(action==='pay'){if(!await dialog.confirm({title:'登记线下打款',description:`请先确认已完成线下打款，再将提现 ${detail.withdrawPublicId}（${money(detail.amountCents)}）标记为已打款。本操作不会发起真实转账。`,confirmLabel:'确认已打款'}))return}else{reason=await dialog.prompt({title:'拒绝提现申请',description:`确认拒绝提现 ${detail.withdrawPublicId}（${money(detail.amountCents)}）。`,inputLabel:'拒绝原因',placeholder:'请输入具体原因',requiredMessage:'拒绝原因不能为空',confirmLabel:'确认拒绝',tone:'danger'})||'';if(!reason)return}setBusy(action);setError('');setConflict(false);try{const updated=action==='pay'?await takeoutAdminAPI.payWithdrawal(detail.withdrawPublicId,detail.rowVersion):await takeoutAdminAPI.rejectWithdrawal(detail.withdrawPublicId,detail.rowVersion,reason);setDetail(updated);await load()}catch(cause){if(cause instanceof TakeoutAPIError&&cause.status===409){setConflict(true);setError(conflictMessage)}else setError(errorMessage(cause))}finally{setBusy('')}}
	const refresh=async()=>{await load();if(detail)await inspect(detail.withdrawPublicId)}
	return <Panel title="提现处理" actions={<div className="takeout-controls"><label>状态<select value={status} onChange={event=>{setPage(1);setStatus(event.target.value as WithdrawStatus|'')}}><option value="">全部</option><option value="pending">pending</option><option value="paid">paid</option><option value="rejected">rejected</option></select></label><button onClick={()=>void load()}>刷新</button></div>}><ErrorNotice error={error} conflict={conflict} refresh={()=>void refresh()}/><div className="table-scroll"><table><thead><tr><th>提现申请</th><th>骑手</th><th>金额</th><th>状态</th><th>版本</th><th>创建时间</th></tr></thead><tbody>{data.items.map(row=><tr key={row.withdrawPublicId}><td><button className="link-button" onClick={()=>void inspect(row.withdrawPublicId)}>{row.withdrawPublicId}</button></td><td>#{row.riderUserId}</td><td>{money(row.amountCents)}</td><td><Badge value={row.status}/></td><td>v{row.rowVersion}</td><td>{time(row.createdAt)}</td></tr>)}</tbody></table></div><Pagination value={data} setPage={setPage}/>{detail&&<Detail title={`提现 ${detail.withdrawPublicId}`} close={()=>setDetail(null)}><div className="detail-grid"><Fact label="骑手" value={`#${detail.riderUserId}`}/><Fact label="金额" value={money(detail.amountCents)}/><Fact label="状态" value={<Badge value={detail.status}/>}/><Fact label="Row Version" value={`v${detail.rowVersion}`}/><Fact label="备注" value={detail.note||'—'}/><Fact label="打款时间" value={time(detail.paidAt)}/></div>{detail.status==='pending'&&<div className="takeout-actions"><button disabled={Boolean(busy)} onClick={()=>void run('pay')}>{busy==='pay'?'处理中…':'标记已打款'}</button><button className="danger" disabled={Boolean(busy)} onClick={()=>void run('reject')}>{busy==='reject'?'处理中…':'拒绝申请'}</button></div>}</Detail>}</Panel>
}
