import React from 'react'
import { DeliveryMessagesContent } from './content'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { useIdentityGuard } from '../../hooks/use-identity-guard'

function MerchantMessagesContent() {
  const allowed = useIdentityGuard('merchant')
  return <DeliveryMessagesContent role='merchant' identityAllowed={allowed} />
}

export default function MerchantMessagesPage() {
  return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><MerchantMessagesContent /></RuntimeSessionGate>
}
