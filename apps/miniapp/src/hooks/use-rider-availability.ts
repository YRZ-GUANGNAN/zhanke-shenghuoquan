import { useCallback, useEffect, useRef, useState } from 'react'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { changeRiderAvailability, getRiderAvailability, riderAvailabilityCanClaim, type RiderAvailability } from '../services/delivery-rider'

// Duty is a server fact, not the identity's active flag. Hiding or switching
// accounts invalidates local callbacks without implicitly stopping a shift.
export function useRiderAvailability(accountId: string | number, allowed: boolean, sessionToken = '') {
  const [value, setValue] = useState<RiderAvailability | null>(null)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const owner = useRef({ id: accountId, allowed, token: sessionToken })
  owner.current = { id: accountId, allowed, token: sessionToken }
  const valueOwner = useRef('')
  const visible = useRef(true)
  const mounted = useRef(true)
  const generation = useRef(0)
  const pending = useRef(false)
  const fact = useRef<RiderAvailability | null>(null)

  const refresh = useCallback(async (): Promise<boolean> => {
    if (!owner.current.allowed || !visible.current || !mounted.current || pending.current) return false
    const account = owner.current.id
    const token = owner.current.token
    const epoch = ++generation.current
    const current = () => mounted.current && visible.current && owner.current.allowed
      && owner.current.id === account && owner.current.token === token && generation.current === epoch
    setBusy(true)
    fact.current = null
    try {
      const result = await getRiderAvailability()
      if (!current()) return false
      if (result.enabled && String(result.riderUserId) !== String(account)) throw new Error('接单状态账户不匹配')
      fact.current = result
      valueOwner.current = `${account}:${token}`
      setValue(result)
      setMessage('')
      return true
    } catch (error) {
      if (current()) { setValue(null); setMessage(error instanceof Error ? error.message : '接单状态同步失败，请刷新') }
      return false
    } finally {
      if (current()) setBusy(false)
    }
  }, [])

  const change = useCallback(async (): Promise<boolean> => {
    const previous = fact.current
    if (!previous?.enabled || !owner.current.allowed || !visible.current || !mounted.current || pending.current) return false
    const account = owner.current.id
    const token = owner.current.token
    if (String(previous.riderUserId) !== String(account) || valueOwner.current !== `${account}:${token}`) return false
    const epoch = ++generation.current
    const current = () => mounted.current && visible.current && owner.current.allowed
      && owner.current.id === account && owner.current.token === token && generation.current === epoch
    pending.current = true
    setBusy(true)
    fact.current = null
    setMessage('')
    let changed = false
    try {
      const result = await changeRiderAvailability(previous.state === 'on_duty' ? 'stop' : 'start', previous.rowVersion)
      if (!current()) return false
      if (result.enabled && String(result.riderUserId) !== String(account)) throw new Error('接单状态账户不匹配')
      fact.current = result
      valueOwner.current = `${account}:${token}`
      setValue(result)
      changed = true
    } catch {
      if (!current()) return false
      // A lost POST response cannot prove failure. Only GET the original fact;
      // never retry or invert the command automatically.
      try {
        const result = await getRiderAvailability()
        if (!current()) return false
        if (result.enabled && String(result.riderUserId) !== String(account)) throw new Error('账户不匹配')
        fact.current = result
        valueOwner.current = `${account}:${token}`
        setValue(result)
        changed = result.enabled && result.state !== previous.state
        setMessage('已同步接单状态，请以当前状态为准')
      } catch {
        if (current()) { setValue(null); setMessage('操作结果待核实，请刷新接单状态；不会自动重试') }
      }
    } finally {
      if (current()) { pending.current = false; setBusy(false) }
    }
    return changed && current()
  }, [])

  useEffect(() => {
    mounted.current = true
    generation.current++
    pending.current = false
    fact.current = null
    setValue(null)
    setBusy(false)
    setMessage('')
    if (allowed) void refresh()
    return () => { generation.current++; fact.current = null; pending.current = false }
  }, [accountId, allowed, sessionToken, refresh])

  useEffect(() => () => { mounted.current = false; generation.current++ }, [])
  useDidHide(() => {
    visible.current = false
    generation.current++
    pending.current = false
    fact.current = null
    setValue(null)
    setBusy(false)
  })
  useDidShow(() => { visible.current = true; void refresh() })

  const ownValue = valueOwner.current === `${accountId}:${sessionToken}` && allowed && visible.current ? value : null
  return { value: ownValue, busy, message, refresh, change, canClaim: !busy && riderAvailabilityCanClaim(ownValue) }
}
