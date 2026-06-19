// Domain B client: community content, follow relations and in-app notifications only.
import type { CommunityComment, CommunityNotification, CommunityPost } from '../types'
import { request } from './api'

interface BackendImage { url: string }
interface BackendUser { id?:number|string; nickname?:string; avatar?:string; school?:string; campus?:string }
interface BackendPost { id:number|string; author?:BackendUser; topic?:string; title?:string; content?:string; images?:BackendImage[]; pinned?:boolean; likedCount?:number; commentCount?:number; favoriteCount?:number; createdAt?:string }
interface BackendComment {id:number|string;author?:BackendUser;content:string;createdAt?:string}
interface BackendNotification {id:number|string;type:CommunityNotification['type'];actor?:BackendUser;postId?:number|string;readAt?:string|null;createdAt?:string}
interface Page<T> {items:T[];page:number;pageSize:number;total:number}
interface NotificationPage extends Page<BackendNotification>{unread:number}

export async function listCommunityPosts(page=1,sort:'recent'|'popular'='recent'):Promise<CommunityPost[]>{const result=await request<Page<BackendPost>>(`/community/posts?page=${page}&sort=${sort}`);return result.items.map(normalizePost)}
export async function listFollowingPosts(page=1):Promise<CommunityPost[]>{const result=await request<Page<BackendPost>>(`/community/following-feed?page=${page}`);return result.items.map(normalizePost)}
export async function getCommunityPost(id:string):Promise<CommunityPost>{return normalizePost(await request<BackendPost>(`/community/posts/${id}`))}
export async function createCommunityPost(input:{title:string;content:string;topic?:string;imageAssetIds?:string[]}):Promise<void>{await request('/community/posts',{url:'',method:'POST',data:input})}
export async function listPostComments(id:string,page=1):Promise<CommunityComment[]>{const result=await request<Page<BackendComment>>(`/community/posts/${id}/comments?page=${page}`);return result.items.map(item=>({id:String(item.id),author:item.author?.nickname||'校园用户',avatar:item.author?.avatar||'',content:item.content,createdAt:dateLabel(item.createdAt)}))}
export async function addPostComment(id:string,content:string):Promise<void>{await request(`/community/posts/${id}/comments`,{url:'',method:'POST',data:{content}})}
export async function togglePostLike(id:string):Promise<boolean>{return(await request<{liked:boolean}>(`/community/posts/${id}/like`,{url:'',method:'POST'})).liked}
export async function togglePostFavorite(id:string):Promise<boolean>{return(await request<{favorited:boolean}>(`/community/posts/${id}/favorite`,{url:'',method:'POST'})).favorited}
export async function toggleUserFollow(id:string):Promise<boolean>{return(await request<{followed:boolean}>(`/community/users/${id}/follow`,{url:'',method:'POST'})).followed}
export async function listCommunityNotifications(page=1):Promise<{items:CommunityNotification[];unread:number}>{const result=await request<NotificationPage>(`/community/notifications?page=${page}`);return{unread:result.unread,items:result.items.map(item=>({id:String(item.id),type:item.type,actor:item.actor?.nickname||'校园用户',postId:String(item.postId||''),read:Boolean(item.readAt),createdAt:dateLabel(item.createdAt)}))}}
export async function markCommunityNotificationRead(id:string):Promise<void>{await request(`/community/notifications/${id}/read`,{url:'',method:'POST'})}

function normalizePost(item:BackendPost):CommunityPost{return{id:String(item.id),authorId:String(item.author?.id||''),author:item.author?.nickname||'校园用户',avatar:item.author?.avatar||'',school:item.author?.school||item.author?.campus||'',topic:item.topic||'',title:item.title||'',content:item.content||'',images:(item.images||[]).map(v=>v.url),likedCount:item.likedCount||0,commentCount:item.commentCount||0,favoriteCount:item.favoriteCount||0,createdAt:dateLabel(item.createdAt),pinned:item.pinned}}
function dateLabel(value?:string):string{if(!value)return'';const date=new Date(value);return Number.isNaN(date.getTime())?value:`${String(date.getMonth()+1).padStart(2,'0')}-${String(date.getDate()).padStart(2,'0')} ${String(date.getHours()).padStart(2,'0')}:${String(date.getMinutes()).padStart(2,'0')}`}
