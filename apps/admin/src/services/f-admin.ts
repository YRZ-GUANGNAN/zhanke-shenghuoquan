// Domain F client: management and the unified review workflow only.
import type { ExternalTakeoutMerchant, ExternalTakeoutMerchantDirectory, ExternalTakeoutMerchantInput, ExternalTakeoutOperatingStatus, IdentityApplicationAdminList, IdentityApplicationDetail, IdentityApplicationQuery, IdentityApplicationRow, MediaAssetDetail, MediaAssetRow, MediaDeadletters, MediaDecisionRow, MediaEventRow, MediaGCJobRow, MediaQuery, MediaSummary, NavigationCategory, Page, PostRow, ProductRow, ReportRow, ReviewTask, Stats, UserRow } from '../types'
const base='/api/v1'
export class AdminAPIError extends Error {
	constructor(message:string,readonly status:number){super(message);this.name='AdminAPIError'}
}
type AdminAPIEnvelope<T>={code:number;message:string;data?:T}
function protocolError(response:Response){return new AdminAPIError(response.ok?'Invalid API response':`HTTP ${response.status}`,response.status)}
async function api<T>(path:string,init:RequestInit={}):Promise<T>{
	const token=localStorage.getItem('domainA.token')||''
	let response:Response
	try{
		response=await fetch(`${base}${path}`,{...init,headers:{'Content-Type':'application/json',...(token?{Authorization:`Bearer ${token}`}:{})}})
	}catch{
		throw new AdminAPIError('Network request failed',0)
	}
	let value:unknown
	try{value=await response.json()}catch{throw protocolError(response)}
	if(typeof value!=='object'||value===null||Array.isArray(value))throw protocolError(response)
	const body=value as Partial<AdminAPIEnvelope<T>>
	if(typeof body.code!=='number'||typeof body.message!=='string')throw protocolError(response)
	if(!response.ok||body.code!==0){
		const status=response.ok&&body.code>0?body.code:response.status
		throw new AdminAPIError(body.message||`HTTP ${status}`,status)
	}
	if(!('data' in body))throw protocolError(response)
	return body.data as T
}
function queryString(query:MediaQuery={}){const params=new URLSearchParams();Object.entries(query).forEach(([key,value])=>{if(value!==undefined&&value!=='')params.set(key,String(value))});const value=params.toString();return value?`?${value}`:''}
function identityQueryString(query:IdentityApplicationQuery={}){const params=new URLSearchParams();Object.entries(query).forEach(([key,value])=>{if(value!==undefined&&value!=='')params.set(key,String(value))});const value=params.toString();return value?`?${value}`:''}
export const adminAPI={
	stats:()=>api<Stats>('/admin/stats'),
	reviews:(status='')=>api<Page<ReviewTask>>(`/admin/reviews${status?`?status=${status}`:''}`),
	startReview:(id:number)=>api<ReviewTask>(`/admin/reviews/${id}/start`,{method:'POST'}),
	decideReview:(id:number,decision:'approve'|'reject',reason='')=>api<ReviewTask>(`/admin/reviews/${id}/${decision}`,{method:'POST',body:JSON.stringify({reason})}),
	users:()=>api<Page<UserRow>>('/admin/users'),
	listIdentityApplications:(query:IdentityApplicationQuery={})=>api<IdentityApplicationAdminList>(`/admin/identity-applications${identityQueryString(query)}`),
	identityApplication:(applicationId:string)=>api<IdentityApplicationDetail>(`/admin/identity-applications/${encodeURIComponent(applicationId)}`),
	approveIdentityApplication:(applicationId:string,expectedRowVersion:number)=>api<IdentityApplicationRow>(`/admin/identity-applications/${encodeURIComponent(applicationId)}/approve`,{method:'POST',body:JSON.stringify({expectedRowVersion})}),
	rejectIdentityApplication:(applicationId:string,expectedRowVersion:number,reason:string)=>api<IdentityApplicationRow>(`/admin/identity-applications/${encodeURIComponent(applicationId)}/reject`,{method:'POST',body:JSON.stringify({expectedRowVersion,reason})}),
	updateUserStatus:(id:number,status:UserRow['status'])=>api(`/admin/users/${id}/status`,{method:'PATCH',body:JSON.stringify({status})}),
	updateUserRole:(id:number,role:UserRow['role'])=>api(`/admin/users/${id}/role`,{method:'PATCH',body:JSON.stringify({role})}),
	posts:()=>api<Page<PostRow>>('/admin/posts'),
	takedownPost:(id:number,reason:string)=>api<{updated:boolean}>(`/admin/community/posts/${id}/takedown`,{method:'PATCH',body:JSON.stringify({reason})}),
	products:()=>api<Page<ProductRow>>('/admin/products'),
	takedownProduct:(id:number,reason:string)=>api<{updated:boolean}>(`/admin/trade/products/${id}/takedown`,{method:'PATCH',body:JSON.stringify({reason})}),
	reports:()=>api<Page<ReportRow>>('/admin/reports'),
	navigation:()=>api<NavigationCategory[]>('/admin/navigation'),
	externalTakeoutMerchants:()=>api<ExternalTakeoutMerchantDirectory>('/admin/navigation/external-takeout-merchants'),
	createExternalTakeoutMerchant:(input:ExternalTakeoutMerchantInput)=>api<ExternalTakeoutMerchant>('/admin/navigation/external-takeout-merchants',{method:'POST',body:JSON.stringify(input)}),
	updateExternalTakeoutMerchant:(id:number,input:ExternalTakeoutMerchantInput)=>api<ExternalTakeoutMerchant>(`/admin/navigation/external-takeout-merchants/${id}`,{method:'PUT',body:JSON.stringify(input)}),
	deleteExternalTakeoutMerchant:(id:number)=>api<{deleted:boolean}>(`/admin/navigation/external-takeout-merchants/${id}`,{method:'DELETE'}),
	setExternalTakeoutOperatingStatus:(id:number,operatingStatus:ExternalTakeoutOperatingStatus)=>api<{updated:boolean}>(`/admin/navigation/external-takeout-merchants/${id}/operating-status`,{method:'PATCH',body:JSON.stringify({operatingStatus})}),
	setExternalTakeoutVisibility:(id:number,enabled:boolean)=>api<{updated:boolean}>(`/admin/navigation/external-takeout-merchants/${id}/visibility`,{method:'PATCH',body:JSON.stringify({enabled})}),
	setExternalTakeoutCategoryVisibility:(enabled:boolean)=>api<{updated:boolean}>('/admin/navigation/external-takeout-merchants/category-visibility',{method:'PATCH',body:JSON.stringify({enabled})}),
	mediaSummary:()=>api<MediaSummary>('/admin/media/summary'),
	mediaAssets:(query:MediaQuery={})=>api<Page<MediaAssetRow>>(`/admin/media/assets${queryString(query)}`),
	mediaAsset:(assetId:string)=>api<MediaAssetDetail>(`/admin/media/assets/${encodeURIComponent(assetId)}`),
	mediaDecisions:(query:MediaQuery={})=>api<Page<MediaDecisionRow>>(`/admin/media/review-decisions${queryString(query)}`),
	mediaEvents:(query:MediaQuery={})=>api<Page<MediaEventRow>>(`/admin/media/events${queryString(query)}`),
	mediaGCJobs:(query:MediaQuery={})=>api<Page<MediaGCJobRow>>(`/admin/media/gc-jobs${queryString(query)}`),
	mediaDeadletters:(query:MediaQuery={})=>api<MediaDeadletters>(`/admin/media/deadletters${queryString(query)}`)
}
