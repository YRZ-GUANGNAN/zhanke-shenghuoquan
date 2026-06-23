// Domain F client: management and the unified review workflow only.
import type { MediaAssetDetail, MediaAssetRow, MediaDeadletters, MediaDecisionRow, MediaEventRow, MediaGCJobRow, MediaQuery, MediaSummary, NavigationCategory, Page, PostRow, ProductRow, ReportRow, ReviewTask, Stats, UserRow } from '../types'
const base='/api/v1'
async function api<T>(path:string,init:RequestInit={}):Promise<T>{const token=localStorage.getItem('domainA.token')||'';const response=await fetch(`${base}${path}`,{...init,headers:{'Content-Type':'application/json',...(token?{Authorization:`Bearer ${token}`}:{})}});const body=await response.json() as {code:number;message:string;data:T};if(!response.ok||body.code!==0)throw new Error(body.message||`HTTP ${response.status}`);return body.data}
function queryString(query:MediaQuery={}){const params=new URLSearchParams();Object.entries(query).forEach(([key,value])=>{if(value!==undefined&&value!=='')params.set(key,String(value))});const value=params.toString();return value?`?${value}`:''}
export const adminAPI={
	stats:()=>api<Stats>('/admin/stats'),
	reviews:(status='')=>api<Page<ReviewTask>>(`/admin/reviews${status?`?status=${status}`:''}`),
	startReview:(id:number)=>api<ReviewTask>(`/admin/reviews/${id}/start`,{method:'POST'}),
	decideReview:(id:number,decision:'approve'|'reject',reason='')=>api<ReviewTask>(`/admin/reviews/${id}/${decision}`,{method:'POST',body:JSON.stringify({reason})}),
	users:()=>api<Page<UserRow>>('/admin/users'),
	updateUserStatus:(id:number,status:UserRow['status'])=>api(`/admin/users/${id}/status`,{method:'PATCH',body:JSON.stringify({status})}),
	updateUserRole:(id:number,role:UserRow['role'])=>api(`/admin/users/${id}/role`,{method:'PATCH',body:JSON.stringify({role})}),
	posts:()=>api<Page<PostRow>>('/admin/posts'),
	takedownPost:(id:number,reason:string)=>api<{updated:boolean}>(`/admin/community/posts/${id}/takedown`,{method:'PATCH',body:JSON.stringify({reason})}),
	products:()=>api<Page<ProductRow>>('/admin/products'),
	takedownProduct:(id:number,reason:string)=>api<{updated:boolean}>(`/admin/trade/products/${id}/takedown`,{method:'PATCH',body:JSON.stringify({reason})}),
	reports:()=>api<Page<ReportRow>>('/admin/reports'),
	navigation:()=>api<NavigationCategory[]>('/admin/navigation'),
	mediaSummary:()=>api<MediaSummary>('/admin/media/summary'),
	mediaAssets:(query:MediaQuery={})=>api<Page<MediaAssetRow>>(`/admin/media/assets${queryString(query)}`),
	mediaAsset:(assetId:string)=>api<MediaAssetDetail>(`/admin/media/assets/${encodeURIComponent(assetId)}`),
	mediaDecisions:(query:MediaQuery={})=>api<Page<MediaDecisionRow>>(`/admin/media/review-decisions${queryString(query)}`),
	mediaEvents:(query:MediaQuery={})=>api<Page<MediaEventRow>>(`/admin/media/events${queryString(query)}`),
	mediaGCJobs:(query:MediaQuery={})=>api<Page<MediaGCJobRow>>(`/admin/media/gc-jobs${queryString(query)}`),
	mediaDeadletters:(query:MediaQuery={})=>api<MediaDeadletters>(`/admin/media/deadletters${queryString(query)}`)
}
