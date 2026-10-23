import React from 'react'
import { DeliveryMessagesContent } from '../merchant-messages/content'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { useIdentityGuard } from '../../hooks/use-identity-guard'

function RiderMessagesContent() {
  const allowed = useIdentityGuard('rider')
  return <DeliveryMessagesContent role='rider' identityAllowed={allowed} />
}

export default function RiderMessagesPage() {
  return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><RiderMessagesContent /></RuntimeSessionGate>
}
