import {
  createUnreadSnapshot,
  mergeUnreadSnapshot,
  nextUnreadGeneration,
  unreadCountForDisplay,
  type NotificationUnreadSnapshot
} from './notification'

function expect(condition: unknown, message: string): void {
  if (!condition) throw new Error(message)
}

function equal<T>(actual: T, expected: T, message: string): void {
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`${message}: ${JSON.stringify(actual)} !== ${JSON.stringify(expected)}`)
  }
}

/**
 * Pure contract checks. This function is exported so a Node/TS harness can run it
 * without a Taro runtime, network, account or WeChat credentials.
 */
export function runNotificationUnreadSnapshotTests(): void {
  const initial = createUnreadSnapshot(7)
  expect(unreadCountForDisplay(initial) === null, 'unconfirmed snapshot must not render a fake zero')

  const confirmed = mergeUnreadSnapshot(initial, {
    generation: 7,
    confirmed: true,
    total: 0,
    conversations: {}
  })
  expect(unreadCountForDisplay(confirmed) === 0, 'confirmed zero must remain a real zero')

  const firstEvent = mergeUnreadSnapshot(confirmed, {
    generation: 7,
    conversationId: 'conversation-a',
    unreadCount: 2
  })
  const duplicateEvent = mergeUnreadSnapshot(firstEvent, {
    generation: 7,
    conversationId: 'conversation-a',
    unreadCount: 2
  })
  equal(duplicateEvent.total, 2, 'repeated absolute events must not accumulate')
  equal(duplicateEvent.conversations, { 'conversation-a': 2 }, 'conversation count must be replaced by the snapshot value')

  const stale = mergeUnreadSnapshot(firstEvent, {
    generation: 6,
    conversationId: 'conversation-a',
    unreadCount: 99
  })
  equal(stale, firstEvent, 'an old account generation must not write into the current snapshot')

  const next = nextUnreadGeneration(7)
  expect(next.generation === 8, 'generation must advance when the account changes')
  expect(unreadCountForDisplay(next) === null, 'a new generation starts with an unknown unread count')
}

/** Explicitly typed fixture for consumers that want to run the checks in a harness. */
export const notificationUnreadSnapshotFixture: NotificationUnreadSnapshot = createUnreadSnapshot(0)
