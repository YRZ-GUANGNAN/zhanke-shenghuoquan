// Domain C client: products, Meilisearch results, favorites, state transitions and Tencent IM entry.
import type { Product } from '../types'
import { request } from './api'

interface BackendImage{url:string}
interface BackendUser{id?:number|string;nickname?:string;avatar?:string;campus?:string}
interface BackendProduct{id:number|string;sellerId?:number|string;seller?:BackendUser;title?:string;description?:string;category?:string;price?:number;originalPrice?:number;condition?:string;campus?:string;images?:BackendImage[];createdAt?:string;status?:Product['status'];reviewStatus?:Product['reviewStatus'];favoriteCount?:number}
interface Page<T>{items:T[];page:number;pageSize:number;total:number}
export interface IMConversationContext{relationId:number;conversationId:string;peerUserId:string;productId:number;buyerId:number;sellerId:number}

export async function listProducts(page=1,keyword='',status:Product['status']='selling'):Promise<{items:Product[];total:number}>{const result=await request<Page<BackendProduct>>(`/trade/products?page=${page}&keyword=${encodeURIComponent(keyword)}&status=${status}`);return{items:result.items.map(normalizeProduct),total:result.total}}
export async function listMyProducts(page=1):Promise<Product[]>{const result=await request<Page<BackendProduct>>(`/trade/my-products?page=${page}`);return result.items.map(normalizeProduct)}
export async function getProduct(id:string):Promise<Product>{return normalizeProduct(await request<BackendProduct>(`/trade/products/${id}`))}
export async function createProduct(input:{title:string;description:string;price:number;category?:string;imageAssetIds?:string[]}):Promise<void>{await request('/trade/products',{url:'',method:'POST',data:input})}
export async function toggleProductFavorite(id:string):Promise<boolean>{return(await request<{favorited:boolean}>(`/trade/products/${id}/favorite`,{url:'',method:'POST'})).favorited}
export async function changeProductStatus(id:string,status:Product['status']):Promise<void>{await request(`/trade/products/${id}/status`,{url:'',method:'PATCH',data:{status}})}
export async function contactSeller(id:string):Promise<IMConversationContext>{return request(`/trade/products/${id}/contact`,{url:'',method:'POST',timeout:10000})}

function normalizeProduct(item:BackendProduct):Product{return{id:String(item.id),sellerId:String(item.sellerId||item.seller?.id||''),title:item.title||'',description:item.description||'',price:yuan(item.price),originalPrice:item.originalPrice?yuan(item.originalPrice):undefined,category:item.category||'',images:(item.images||[]).map(v=>v.url),sellerName:item.seller?.nickname||'校园用户',sellerAvatar:item.seller?.avatar||'',campus:item.campus||item.seller?.campus||'',condition:item.condition||'',createdAt:item.createdAt||'',status:item.status||'selling',reviewStatus:item.reviewStatus||'submitted',favoriteCount:item.favoriteCount||0}}
function yuan(value?:number):number{return Number(((value||0)/100).toFixed(2))}
