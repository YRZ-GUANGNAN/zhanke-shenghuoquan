// Domain F presentation contracts. Admin never queries domain databases directly.
export type AdminSection = 'dashboard' | 'users' | 'identities' | 'moderation' | 'media' | 'delivery' | 'takeout' | 'community' | 'trade' | 'reports' | 'feedbacks' | 'navigation'
export interface AdminUser{id:number;nickname:string;role:'admin'|'superadmin'}
export interface AdminSession{user:AdminUser;csrfToken:string;expiresAt:string}
export interface Page<T>{items:T[];page:number;pageSize:number;total:number}
export interface PageQuery{page?:number;pageSize?:number}
export type ReviewDecisionSource='manual'|'automatic'|'manual_override'
export interface ReviewTask{id:number;objectType:'post'|'comment'|'product'|'product_comment'|'report';objectId:number;risk:string;status:'submitted'|'reviewing'|'approved'|'rejected';decisionSource:ReviewDecisionSource;reason:string;createdAt:string}
export interface UserRow{id:number;nickname:string;school:string;campus:string;role:'user'|'admin'|'superadmin';status:'active'|'muted'|'banned'}
export interface Author{nickname:string;school:string;campus:string}
export interface PostRow{id:number;title:string;topic:string;reviewStatus:string;author:Author;createdAt:string}
export interface ProductRow{id:number;title:string;price:number;category:string;status:string;reviewStatus:string;seller:Author;createdAt:string}
export interface ReportRow{id:number;targetType:string;targetId:number;reason:string;reviewStatus:string;createdAt:string}
export interface FeedbackImage{assetId:string;slot:number}
export interface FeedbackRow{id?:number;userId:number;category:string;content:string;status:string;images?:FeedbackImage[];createdAt:string;updatedAt:string}
export interface NavigationItem{id:number;categoryId:number;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;enabled:boolean;operatingStatus?:ExternalTakeoutOperatingStatus}
export interface NavigationCategory{id:number;name:string;systemKey?:string;sort:number;enabled:boolean;items:NavigationItem[]}
export interface NavigationCategoryInput{name:string;sort:number;enabled:boolean}
export interface NavigationItemInput{categoryId:number;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;enabled:boolean;operatingStatus?:ExternalTakeoutOperatingStatus}
export type ExternalTakeoutOperatingStatus='open'|'resting'
export interface ExternalTakeoutMerchant{id:number;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;enabled:boolean;operatingStatus:ExternalTakeoutOperatingStatus}
export interface ExternalTakeoutMerchantDirectory{categoryEnabled:boolean;items:ExternalTakeoutMerchant[]}
export interface ExternalTakeoutMerchantInput{title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number}
export interface Stats{totals:{users:number;posts:number;products:number;pendingReview:number};today:{users:number;posts:number;products:number}}

export type IdentityApplicationRole='rider'|'merchant'
export type IdentityApplicationStatus='pending'|'approved'|'rejected'
export type IdentityApplicationEventType='submitted'|'resubmitted'|'approved'|'rejected'
export interface IdentityApplicationProof{assetId:string;url:string;expiresAt:string}
export interface IdentityApplicationEvent{eventId:string;eventType:IdentityApplicationEventType;actorUserId:string;status:IdentityApplicationStatus;reason:string;createdAt:string}
export interface IdentityApplicationRow{applicationId:string;userId:string;role:IdentityApplicationRole;status:IdentityApplicationStatus;realName:string;phone:string;merchantName:string;merchantAddress:string;assetIds:string[];proofs?:IdentityApplicationProof[];reviewReason:string;reviewerUserId?:string;submittedAt:string;reviewedAt?:string;rowVersion:number}
export interface IdentityApplicationDetail extends IdentityApplicationRow{events:IdentityApplicationEvent[]}
export interface IdentityApplicationQuery{page?:number;pageSize?:number;role?:IdentityApplicationRole;status?:IdentityApplicationStatus}
export interface IdentityApplicationAdminList{items:IdentityApplicationRow[];page:number;pageSize:number;total:number}

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

export type MerchantStatus='pending_review'|'active'|'resting'|'rejected'|'suspended'
export type DishStatus='draft'|'on_shelf'|'sold_out'|'off_shelf'|'removed'
export type OrderStatus='pending_payment'|'paid'|'accepted'|'preparing'|'ready_for_pickup'|'delivering'|'completed'|'cancelled'|'refunded'
export type RiderStatus='active'|'resting'|'suspended'
export type WithdrawStatus='pending'|'paid'|'rejected'

export interface TakeoutQuery{page?:number;pageSize?:number;status?:string;merchantId?:string;categoryId?:string}
export interface MerchantRow{merchantPublicId:string;ownerUserId:number;name:string;contactPhone:string;location:string;businessHours:string;logoAssetPublicId?:string;status:MerchantStatus;rowVersion:number;createdAt:string;updatedAt:string}
export interface DishCategoryRow{categoryPublicId:string;merchantPublicId:string;name:string;sortOrder:number;rowVersion:number;createdAt:string;updatedAt:string}
export interface DishRow{dishPublicId:string;merchantPublicId:string;categoryPublicId?:string;name:string;description:string;priceCents:number;imageAssetPublicId?:string;status:DishStatus;sortOrder:number;rowVersion:number;createdAt:string;updatedAt:string}
export interface OrderRow{orderPublicId:string;buyerUserId:number;merchantPublicId:string;riderUserId?:number;status:OrderStatus;itemsAmountCents:number;deliveryFeeCents:number;totalAmountCents:number;contactName:string;contactPhone:string;addressBuilding:string;addressRoom:string;remark:string;payNo?:string;paidAt?:string;acceptedAt?:string;preparingAt?:string;readyAt?:string;deliveringAt?:string;completedAt?:string;cancelledAt?:string;refundedAt?:string;rowVersion:number;createdAt:string;updatedAt:string}
export interface OrderItemRow{dishPublicId:string;dishNameSnapshot:string;unitPriceCentsSnapshot:number;quantity:number;lineAmountCents:number}
export interface OrderDetail{order:OrderRow;items:OrderItemRow[]}
export interface RiderRow{riderUserId:number;status:RiderStatus;realName:string;phone:string;rowVersion:number;createdAt:string;updatedAt:string}
export interface FeeConfigRow{riderFeeCents:number;platformCutCents:number;rowVersion:number;updatedAt:string}
export interface RiderConfigRow{maxInFlightPerRider:number;rowVersion:number;updatedAt:string}
export interface SettlementRow{riderUserId:number;pendingCount:number;riderFeeCents:number;platformCutCents:number}
export interface WithdrawRow{withdrawPublicId:string;riderUserId:number;amountCents:number;status:WithdrawStatus;note:string;paidAt?:string;rowVersion:number;createdAt:string;updatedAt:string}
