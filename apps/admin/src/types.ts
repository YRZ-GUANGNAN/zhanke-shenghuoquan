// Domain F presentation contracts. Admin never queries domain databases directly.
export type AdminSection = 'dashboard' | 'users' | 'moderation' | 'media' | 'community' | 'trade' | 'reports' | 'navigation'
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

export interface MediaQuery{page?:number;pageSize?:number;assetId?:string;status?:string;reviewStatus?:string;purpose?:string;taskId?:string;operation?:string;kind?:string;from?:string;to?:string}
export interface MediaSummary{assets:number;pendingReview:number;consumerDeadletters:number;gcDeadletters:number}
export interface MediaAssetRow{assetId:string;ownerId:number;purpose:string;storageStage:string;technicalStatus:string;reviewStatus:string;reviewTaskId?:number;ticketId?:string;ticketStatus?:string;completionPhase?:string;referenceCount:number;blockingReferences:number;mimeType:string;byteSize:number;width:number;height:number;createdAt:string;updatedAt:string;deleteRequestedAt?:string;deletedAt?:string}
export interface MediaReferenceRow{id:number;referenceType:string;referenceId:string;slot:number;status:string;parentVisibility:string;aggregateVersion:number;fenceExpiresAt?:string;updatedAt:string}
export interface MediaTicketRow{ticketId:string;status:string;expectedSize:number;expectedContentType:string;credentialIssueCount:number;expiresAt:string;createdAt:string;updatedAt:string}
export interface MediaCompletionRow{ticketId:string;phase:string;validationAttempts:number;promoteAttempts:number;lastErrorCode:string;completedAt?:string;updatedAt:string}
export interface MediaProjectionRow{projectionId:string;reviewTaskId?:number;status:string;appliedDecisionVersion:number;failureCode:string;submittedAt?:string;reviewingAt?:string;decidedAt?:string;updatedAt:string}
export interface MediaDecisionRow{assetId:string;projectionId:string;reviewTaskId:number;decisionVersion:number;decision:string;payloadHash:string;resultCode:string;receivedAt:string}
export interface MediaEventRow{eventId:string;aggregateId:string;aggregateVersion:number;operation:string;consumerStatus:string;attemptCount:number;lastErrorCode:string;nextAttemptAt?:string;processedAt?:string;createdAt:string}
export interface MediaCheckpointRow{aggregateId:string;lastEventId:string;lastAggregateVersion:number;lastOperation:string;activeEventId?:string;leaseExpiresAt?:string;updatedAt:string}
export interface MediaGCJobRow{kind:string;jobId:string;assetId:string;objectStage?:string;status:string;attemptCount:number;lastErrorCode:string;providerRequestId?:string;deletedAt?:string;createdAt:string;updatedAt:string}
export interface MediaAssetDetail{asset:MediaAssetRow;references:MediaReferenceRow[];ticket?:MediaTicketRow;completion?:MediaCompletionRow;projection?:MediaProjectionRow;decisions:MediaDecisionRow[];events:MediaEventRow[];checkpoint?:MediaCheckpointRow;gcJobs:MediaGCJobRow[]}
export interface MediaDeadletters{consumer:Page<MediaEventRow>;gc:Page<MediaGCJobRow>}
