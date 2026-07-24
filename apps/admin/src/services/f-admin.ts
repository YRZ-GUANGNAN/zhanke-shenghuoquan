// Domain F client: management and the unified review workflow only.
import type { ExternalTakeoutMerchant, ExternalTakeoutMerchantDirectory, ExternalTakeoutMerchantInput, ExternalTakeoutOperatingStatus, IdentityApplicationAdminList, IdentityApplicationDetail, IdentityApplicationQuery, IdentityApplicationRow, MediaAssetDetail, MediaAssetRow, MediaDeadletters, MediaDecisionRow, MediaEventRow, MediaGCJobRow, MediaQuery, MediaSummary, NavigationCategory, Page, PostRow, ProductRow, ReportRow, ReviewTask, Stats, UserRow } from '../types'
import { adminRequest } from './http'
export class AdminAPIError extends Error {
	constructor(message:string,readonly status:number){super(message);this.name='AdminAPIError'}
}
async function api<T>(path:string,init:RequestInit={}):Promise<T>{
	return adminRequest<T>(path,init,(status,message)=>new AdminAPIError(message,status))
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
