// Domain F client: management and the unified review workflow only.
import type { NavigationCategory, Page, PostRow, ProductRow, ReportRow, ReviewTask, Stats, UserRow } from '../types'
const base='/api/v1'
async function api<T>(path:string,init:RequestInit={}):Promise<T>{const token=localStorage.getItem('domainA.token')||'';const response=await fetch(`${base}${path}`,{...init,headers:{'Content-Type':'application/json',...(token?{Authorization:`Bearer ${token}`}:{})}});const body=await response.json() as {code:number;message:string;data:T};if(!response.ok||body.code!==0)throw new Error(body.message||`HTTP ${response.status}`);return body.data}
export const adminAPI={
	stats:()=>api<Stats>('/admin/stats'),
	reviews:(status='')=>api<Page<ReviewTask>>(`/admin/reviews${status?`?status=${status}`:''}`),
	startReview:(id:number)=>api<ReviewTask>(`/admin/reviews/${id}/start`,{method:'POST'}),
	decideReview:(id:number,decision:'approve'|'reject',reason='')=>api<ReviewTask>(`/admin/reviews/${id}/${decision}`,{method:'POST',body:JSON.stringify({reason})}),
	users:()=>api<Page<UserRow>>('/admin/users'),
	updateUserStatus:(id:number,status:UserRow['status'])=>api(`/admin/users/${id}/status`,{method:'PATCH',body:JSON.stringify({status})}),
	updateUserRole:(id:number,role:UserRow['role'])=>api(`/admin/users/${id}/role`,{method:'PATCH',body:JSON.stringify({role})}),
	posts:()=>api<Page<PostRow>>('/admin/posts'),
	products:()=>api<Page<ProductRow>>('/admin/products'),
	reports:()=>api<Page<ReportRow>>('/admin/reports'),
	navigation:()=>api<NavigationCategory[]>('/admin/navigation')
}
