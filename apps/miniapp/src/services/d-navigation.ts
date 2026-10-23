// Domain D client: category + appid/path jump data. No business actions live here.
import type { ToolLink } from '../types'
import { request } from './api'
import { validMiniProgramAppID, validMiniProgramPath } from './d-navigation-validation'
export { validMiniProgramAppID, validMiniProgramPath } from './d-navigation-validation'
export type NavigationOperatingStatus='open'|'resting'
export interface ExternalTakeoutMerchantLink{id:string;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;operatingStatus:NavigationOperatingStatus}
export interface NavigationHotSearchKeyword{keyword:string;count:number;score:number}
export interface ExternalTakeoutSearchPage{items:ExternalTakeoutMerchantLink[];page:number;pageSize:number;total:number}
interface BackendItem{id:number|string;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;operatingStatus:NavigationOperatingStatus}
interface BackendCategory{id:number|string;name:string;systemKey?:string;sort:number;items:BackendItem[]}
const externalTakeoutSystemKey='external_takeout_merchants' as const

function isRealTool(item:BackendItem):boolean{
	return validMiniProgramAppID(item.appId)&&validMiniProgramPath(item.path)
}

export function listNavigationCategories():Promise<BackendCategory[]>{return request<BackendCategory[]>('/navigation/categories')}
export async function listNavigationItems():Promise<ToolLink[]>{
	const categories=await listNavigationCategories()
	return categories
		.filter((category)=>category.systemKey!==externalTakeoutSystemKey)
		.reduce<ToolLink[]>((all,category)=>all.concat(category.items.filter(isRealTool).map((item)=>({id:String(item.id),categoryId:String(category.id),categoryName:category.name,title:item.title,subtitle:item.subtitle,iconText:item.iconText,appId:item.appId.trim(),path:item.path.trim(),color:item.color}))),[])
}
export async function listExternalTakeoutMerchants():Promise<ExternalTakeoutMerchantLink[]>{
	const categories=await listNavigationCategories()
	const category=categories.find((value)=>value.systemKey===externalTakeoutSystemKey)
	return (category?.items||[]).map((item)=>({id:String(item.id),title:item.title,subtitle:item.subtitle,iconText:item.iconText,appId:item.appId,path:item.path,color:item.color,sort:item.sort,operatingStatus:item.operatingStatus}))
}
export async function searchExternalTakeoutMerchants(keyword:string,page=1,pageSize=20):Promise<ExternalTakeoutSearchPage>{
	const result=await request<{items:BackendItem[];page:number;pageSize:number;total:number}>('/navigation/external-takeout/searches',{url:'',method:'POST',data:{keyword,page,pageSize},timeout:10000})
	return{...result,items:result.items.map((item)=>({id:String(item.id),title:item.title,subtitle:item.subtitle,iconText:item.iconText,appId:item.appId,path:item.path,color:item.color,sort:item.sort,operatingStatus:item.operatingStatus}))}
}
export function listExternalTakeoutSearchHistory():Promise<string[]>{return request<string[]>('/navigation/external-takeout/search-history')}
export async function clearExternalTakeoutSearchHistory():Promise<void>{await request('/navigation/external-takeout/search-history',{url:'',method:'DELETE'})}
export function listExternalTakeoutHotSearches():Promise<NavigationHotSearchKeyword[]>{return request<NavigationHotSearchKeyword[]>('/navigation/external-takeout/hot-searches')}
