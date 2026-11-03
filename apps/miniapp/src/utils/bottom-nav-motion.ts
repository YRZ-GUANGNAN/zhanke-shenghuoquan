/**
 * Compatibility bridge retained for existing ScrollView callers.
 * Bottom navigation is now permanently visible, so scroll position is
 * intentionally ignored and never changes navigation state.
 */
export function useBottomNavScroll(_enabled = true, _ownerKey = '') {
  return (_scrollTop: number) => undefined
}
