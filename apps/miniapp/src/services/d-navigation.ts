// Domain D client: category + appid/path jump data. No business actions live here.
import type { ToolLink } from '../types'
import { request } from './api'
interface BackendItem{id:number|string;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string}
interface BackendCategory{items:BackendItem[]}
export async function listNavigationItems():Promise<ToolLink[]>{const categories=await request<BackendCategory[]>('/navigation/categories');return categories.reduce<ToolLink[]>((all,category)=>all.concat(category.items.map(item=>({id:String(item.id),title:item.title,subtitle:item.subtitle,iconText:item.iconText,appId:item.appId,path:item.path,color:item.color}))),[])}
