// Domain F presentation contracts. Admin never queries domain databases directly.
export type AdminSection = 'dashboard' | 'users' | 'moderation' | 'community' | 'trade' | 'reports' | 'navigation'
export interface Page<T>{items:T[];page:number;pageSize:number;total:number}
export interface ReviewTask{id:number;objectType:'post'|'comment'|'product'|'report';objectId:number;risk:string;status:'submitted'|'reviewing'|'approved'|'rejected';reason:string;createdAt:string}
export interface UserRow{id:number;nickname:string;school:string;campus:string;role:'user'|'admin'|'superadmin';status:'active'|'muted'|'banned'}
export interface Author{nickname:string;school:string;campus:string}
export interface PostRow{id:number;title:string;topic:string;reviewStatus:string;author:Author;createdAt:string}
export interface ProductRow{id:number;title:string;price:number;category:string;status:string;reviewStatus:string;seller:Author;createdAt:string}
export interface ReportRow{id:number;targetType:string;targetId:number;reason:string;reviewStatus:string;createdAt:string}
export interface NavigationItem{id:number;title:string;appId:string;path:string;enabled:boolean}
export interface NavigationCategory{id:number;name:string;enabled:boolean;items:NavigationItem[]}
export interface Stats{totals:{users:number;posts:number;products:number;pendingReview:number};today:{users:number;posts:number;products:number}}
