import { useEffect, useRef, useState } from 'react'

/** An event counter, never a saved liked/favorited value, drives this feedback. */
export function useConfirmedFeedback(event: number, owner: string) {
  const previous = useRef({ event, owner })
  const [pulse, setPulse] = useState(0)
  useEffect(() => {
    const before = previous.current
    previous.current = { event, owner }
    if (before.owner !== owner || event <= before.event) {
      setPulse(0)
      return
    }
    setPulse(event)
    const timer = setTimeout(() => setPulse(0), 280)
    return () => clearTimeout(timer)
  }, [event, owner])
  return pulse
}
