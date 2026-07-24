import type { DishCategoryRow, DishRow, FeeConfigRow, MerchantRow, OrderDetail, OrderRow, Page, RiderConfigRow, RiderRow, RiderStatus, SettlementRow, TakeoutQuery, WithdrawRow } from '../types'
import { adminRequest } from './http'

const base='/admin/takeout'

export class TakeoutAPIError extends Error{
	status:number
	constructor(status:number,message:string){super(message);this.name='TakeoutAPIError';this.status=status}
}

async function api<T>(path:string,init:RequestInit={}):Promise<T>{
	return adminRequest<T>(`${base}${path}`,init,(status,message)=>new TakeoutAPIError(status,message))
}

function queryString(query:TakeoutQuery={}){
	const params=new URLSearchParams()
	Object.entries(query).forEach(([key,value])=>{if(value!==undefined&&value!=='')params.set(key,String(value))})
	const encoded=params.toString()
	return encoded?`?${encoded}`:''
}

const body=(value:unknown):RequestInit=>({headers:{'Content-Type':'application/json'},body:JSON.stringify(value)})

export const takeoutAdminAPI={
	merchants:(query:TakeoutQuery={})=>api<Page<MerchantRow>>(`/merchants${queryString(query)}`),
	merchant:(id:string)=>api<MerchantRow>(`/merchants/${encodeURIComponent(id)}`),
	suspendMerchant:(id:string,status:MerchantRow['status'],rowVersion:number)=>api<MerchantRow>(`/merchants/${encodeURIComponent(id)}/suspend`,{...body({expectedStatus:status,expectedRowVersion:rowVersion}),method:'PATCH'}),
	unsuspendMerchant:(id:string,rowVersion:number)=>api<MerchantRow>(`/merchants/${encodeURIComponent(id)}/unsuspend`,{...body({expectedRowVersion:rowVersion}),method:'PATCH'}),
	categories:(query:TakeoutQuery={})=>api<Page<DishCategoryRow>>(`/categories${queryString(query)}`),
	category:(id:string)=>api<DishCategoryRow>(`/categories/${encodeURIComponent(id)}`),
	dishes:(query:TakeoutQuery={})=>api<Page<DishRow>>(`/dishes${queryString(query)}`),
	dish:(id:string)=>api<DishRow>(`/dishes/${encodeURIComponent(id)}`),
	orders:(query:TakeoutQuery={})=>api<Page<OrderRow>>(`/orders${queryString(query)}`),
	order:(id:string)=>api<OrderDetail>(`/orders/${encodeURIComponent(id)}`),
	riders:(query:TakeoutQuery={})=>api<Page<RiderRow>>(`/riders${queryString(query)}`),
	updateRiderStatus:(id:number,status:RiderStatus,expectedStatus:RiderStatus,rowVersion:number)=>api<RiderRow>(`/riders/${id}/status`,{...body({status,expectedStatus,expectedRowVersion:rowVersion}),method:'PATCH'}),
	feeConfig:()=>api<FeeConfigRow>('/fee-config'),
	updateFeeConfig:(riderFeeCents:number,platformCutCents:number,rowVersion:number)=>api<FeeConfigRow>('/fee-config',{...body({riderFeeCents,platformCutCents,expectedRowVersion:rowVersion}),method:'PUT'}),
	riderConfig:()=>api<RiderConfigRow>('/rider-config'),
	updateRiderConfig:(maxInFlightPerRider:number,rowVersion:number)=>api<RiderConfigRow>('/rider-config',{...body({maxInFlightPerRider,expectedRowVersion:rowVersion}),method:'PUT'}),
	settlements:(query:TakeoutQuery={})=>api<Page<SettlementRow>>(`/settlement-report${queryString(query)}`),
	withdrawals:(query:TakeoutQuery={})=>api<Page<WithdrawRow>>(`/withdraw-requests${queryString(query)}`),
	withdrawal:(id:string)=>api<WithdrawRow>(`/withdraw-requests/${encodeURIComponent(id)}`),
	payWithdrawal:(id:string,rowVersion:number)=>api<WithdrawRow>(`/withdraw-requests/${encodeURIComponent(id)}/pay`,{...body({expectedRowVersion:rowVersion}),method:'PATCH'}),
	rejectWithdrawal:(id:string,rowVersion:number,reason:string)=>api<WithdrawRow>(`/withdraw-requests/${encodeURIComponent(id)}/reject`,{...body({expectedRowVersion:rowVersion,reason}),method:'PATCH'})
}
