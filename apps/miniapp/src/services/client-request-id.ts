const CROCKFORD = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'

export type ClientRequestRandom = () => number

export function createClientRequestId(
  now = Date.now(),
  random: ClientRequestRandom = Math.random
): string {
  let value = Math.max(0, Math.floor(now))
  let timestamp = ''

  for (let index = 0; index < 10; index += 1) {
    timestamp = CROCKFORD[value % CROCKFORD.length] + timestamp
    value = Math.floor(value / CROCKFORD.length)
  }

  let entropy = ''
  for (let index = 0; index < 16; index += 1) {
    const sample = random()
    const normalized = Number.isFinite(sample) ? Math.min(Math.max(sample, 0), 0.9999999999999999) : 0
    entropy += CROCKFORD[Math.floor(normalized * CROCKFORD.length)]
  }

  return `${timestamp}${entropy}`
}
