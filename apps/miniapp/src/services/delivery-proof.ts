import { request } from './api'

export type DeliveryProofType = 'pickup' | 'delivery'
export type DeliveryProofStatus = 'binding' | 'active' | 'deadletter' | 'revoked'

export interface DeliveryProof {
  proofPublicId: string
  proofType: DeliveryProofType
  slot: number
  status: DeliveryProofStatus
  createdAt: string
  activatedAt: string | null
}

export interface DeliveryProofList {
  items: DeliveryProof[]
}

export interface ResolvedDeliveryProof {
  url: string
  expiresInSeconds: number
}

export interface DeliveryProofImage extends DeliveryProof {
  url?: string
  expiresAt?: number
}

const BASE = '/delivery/orders'

export function listDeliveryProofs(deliveryOrderPublicId: string): Promise<DeliveryProofList> {
  return request<DeliveryProofList>(
    `${BASE}/${encodeURIComponent(deliveryOrderPublicId)}/proofs`,
    { url: '', timeout: 10000 }
  )
}

export function resolveDeliveryProof(
  deliveryOrderPublicId: string,
  proofType: DeliveryProofType,
  slot: number
): Promise<ResolvedDeliveryProof> {
  return request<ResolvedDeliveryProof>(
    `${BASE}/${encodeURIComponent(deliveryOrderPublicId)}/proofs/${proofType}/${slot}/resolve`,
    { url: '', timeout: 10000 }
  )
}

// URLs are deliberately resolved afresh for every detail refresh. They are
// never persisted to storage or shared with the generic Media resolve path.
export async function loadPrivateDeliveryProofs(
  deliveryOrderPublicId: string
): Promise<DeliveryProofImage[]> {
  const page = await listDeliveryProofs(deliveryOrderPublicId)
  return Promise.all(page.items.map(async (proof) => {
    if (proof.status !== 'active') return proof
    const resolved = await resolveDeliveryProof(
      deliveryOrderPublicId,
      proof.proofType,
      proof.slot
    )
    return {
      ...proof,
      url: resolved.url,
      expiresAt: Date.now() + resolved.expiresInSeconds * 1000
    }
  }))
}
