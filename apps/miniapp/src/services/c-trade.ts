// Domain C client: products, Meilisearch results, favorites, state transitions and Tencent IM entry.
import type { Product, ProductComment, UserProfile } from '../types'
import { API_BASE, request } from './api'
import { createClientRequestId } from './client-request-id'
import { resolveApiResourceUrl } from './resource-url'

interface BackendImage{url:string}
interface BackendUser{id?:number|string;nickname?:string;avatar?:string;campus?:string}
interface BackendProduct{id:number|string;sellerId?:number|string;seller?:BackendUser;title?:string;description?:string;category?:string;price?:number;originalPrice?:number;condition?:string;campus?:string;images?:BackendImage[];createdAt?:string;status?:Product['status'];reviewStatus?:Product['reviewStatus'];favoriteCount?:number}
interface BackendProductComment{id?:number|string;productId?:number|string;parentCommentId?:number|string;replyToUserId?:number|string;content?:string;reviewStatus?:ProductComment['reviewStatus'];author?:BackendUser;replyTo?:BackendUser;isSeller?:boolean;replies?:BackendProductComment[];createdAt?:string}
interface BackendPage<T>{items:T[];page:number;pageSize:number;total:number}
export interface ProductListQuery{page?:number;pageSize?:number;keyword?:string;category?:string;status?:Product['status']}
export interface ProductPageResult{items:Product[];page:number;pageSize:number;total:number}
export interface ProductCommentPage{items:ProductComment[];page:number;pageSize:number;total:number}
export interface TradeHotSearchKeyword{keyword:string;count:number;score:number}
export interface IMConversationContext{relationId:number;conversationId:string;peerUserId:string;productId:number;buyerId:number;sellerId:number}
export interface CreateProductInput{title:string;description:string;price:number;category?:string;imageAssetIds?:string[];clientRequestId?:string}
export interface TradeViewerState{favorited:boolean;contactedSeller:boolean}

export async function getProductsPage(query:ProductListQuery={}):Promise<ProductPageResult>{const page=query.page??1;const pageSize=query.pageSize??20;const status=query.status??'selling';const params=[`page=${page}`,`pageSize=${pageSize}`,`status=${status}`];if(query.keyword?.trim())params.push(`keyword=${encodeURIComponent(query.keyword.trim())}`);if(query.category?.trim())params.push(`category=${encodeURIComponent(query.category.trim())}`);const result=await request<BackendPage<BackendProduct>>(`/trade/products?${params.join('&')}`);return{...result,items:result.items.map(normalizeProduct)}}
export async function searchTradeProducts(keyword:string,page=1,pageSize=20):Promise<ProductPageResult>{const result=await request<BackendPage<BackendProduct>>('/trade/searches',{url:'',method:'POST',data:{keyword,page,pageSize},timeout:10000});return{...result,items:result.items.map(normalizeProduct)}}
export function listTradeSearchHistory():Promise<string[]>{return request<string[]>('/trade/search-history')}
export async function clearTradeSearchHistory():Promise<void>{await request('/trade/search-history',{url:'',method:'DELETE'})}
export function listTradeHotSearches():Promise<TradeHotSearchKeyword[]>{return request<TradeHotSearchKeyword[]>('/trade/hot-searches')}
export async function listProducts(page=1,keyword='',status:Product['status']='selling'):Promise<{items:Product[];total:number}>{const result=await getProductsPage({page,keyword,status});return{items:result.items,total:result.total}}
export async function getMyProductsPage(page=1,pageSize=20):Promise<ProductPageResult>{const result=await request<BackendPage<BackendProduct>>(`/trade/my-products?page=${page}&pageSize=${pageSize}`);return{...result,items:result.items.map(normalizeProduct)}}
export async function getPublicUserProducts(userId:string,page=1,pageSize=20):Promise<ProductPageResult>{const result=await request<BackendPage<BackendProduct>>(`/trade/users/${encodeURIComponent(userId)}/products?page=${page}&pageSize=${pageSize}`);return{...result,items:result.items.map(normalizeProduct)}}
export async function listMyProducts(page=1):Promise<Product[]>{return(await getMyProductsPage(page)).items}
export async function getProduct(id:string):Promise<Product>{return normalizeProduct(await request<BackendProduct>(`/trade/products/${id}`))}
export async function getTradeViewerState(id:string):Promise<TradeViewerState>{return request(`/trade/products/${id}/viewer-state`)}
export async function createProduct(input:CreateProductInput):Promise<void>{const clientRequestId=input.clientRequestId??createClientRequestId();await request('/trade/products',{url:'',method:'POST',data:{...input,clientRequestId},header:{'Idempotency-Key':clientRequestId},timeout:10000})}
export async function toggleProductFavorite(id:string):Promise<boolean>{return(await request<{favorited:boolean}>(`/trade/products/${id}/favorite`,{url:'',method:'POST'})).favorited}
export async function setProductFavorited(id:string,favorited:boolean):Promise<boolean>{return(await request<{favorited:boolean}>(`/trade/products/${id}/favorite`,{url:'',method:favorited?'PUT':'DELETE'})).favorited}
export async function changeProductStatus(id:string,status:Product['status']):Promise<void>{await request(`/trade/products/${id}/status`,{url:'',method:'PATCH',data:{status}})}
export async function contactSeller(id:string):Promise<IMConversationContext>{return request(`/trade/products/${id}/contact`,{url:'',method:'PUT',timeout:10000})}
export async function listProductComments(productId:string,page=1,pageSize=20):Promise<ProductCommentPage>{const result=await request<BackendPage<BackendProductComment>>(`/trade/products/${encodeURIComponent(productId)}/comments?page=${page}&pageSize=${pageSize}`);return{...result,items:result.items.map(normalizeProductComment)}}
export async function createProductComment(productId:string,content:string,clientRequestId=createClientRequestId()):Promise<ProductComment>{const result=await request<BackendProductComment>(`/trade/products/${encodeURIComponent(productId)}/comments`,{url:'',method:'POST',data:{content,clientRequestId},header:{'Idempotency-Key':clientRequestId},timeout:10000});return normalizeProductComment(result)}
export async function replyProductComment(productId:string,commentId:string,content:string,clientRequestId=createClientRequestId()):Promise<ProductComment>{const result=await request<BackendProductComment>(`/trade/products/${encodeURIComponent(productId)}/comments/${encodeURIComponent(commentId)}/replies`,{url:'',method:'POST',data:{content,clientRequestId},header:{'Idempotency-Key':clientRequestId},timeout:10000});return normalizeProductComment(result)}

function normalizeProduct(item:BackendProduct):Product{return{id:String(item.id),sellerId:String(item.sellerId||item.seller?.id||''),title:item.title||'',description:item.description||'',price:yuan(item.price),originalPrice:item.originalPrice?yuan(item.originalPrice):undefined,category:item.category||'',images:(item.images??[]).map(v=>resolveApiResourceUrl(v.url,API_BASE)),sellerName:item.seller?.nickname||'校园用户',sellerAvatar:item.seller?.avatar||'',campus:item.campus||item.seller?.campus||'',condition:item.condition||'',createdAt:item.createdAt||'',status:item.status||'selling',reviewStatus:item.reviewStatus||'submitted',favoriteCount:item.favoriteCount||0}}
function normalizeProductComment(item:BackendProductComment):ProductComment{return{id:String(item.id??''),productId:String(item.productId??''),parentCommentId:optionalID(item.parentCommentId),replyToUserId:optionalID(item.replyToUserId),content:item.content??'',reviewStatus:item.reviewStatus??'submitted',author:normalizeUser(item.author),replyTo:item.replyTo?normalizeUser(item.replyTo):undefined,isSeller:Boolean(item.isSeller),replies:(item.replies??[]).map(normalizeProductComment),createdAt:item.createdAt??''}}
function normalizeUser(item?:BackendUser):UserProfile{return{id:String(item?.id??''),nickname:item?.nickname??'校园用户',avatar:item?.avatar??'',school:'',campus:item?.campus??''}}
function optionalID(value?:number|string):string{if(value===undefined||value===null||value===''||value===0||value==='0')return'';return String(value)}
function yuan(value?:number):number{return Number(((value||0)/100).toFixed(2))}
