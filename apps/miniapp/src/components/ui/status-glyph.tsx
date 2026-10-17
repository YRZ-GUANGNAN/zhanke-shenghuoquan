import { Image, View } from '@tarojs/components'
import empty from '../../assets/icons/status-empty.svg'
import error from '../../assets/icons/status-error.svg'
import offline from '../../assets/icons/status-offline.svg'
import forbidden from '../../assets/icons/status-forbidden.svg'
import login from '../../assets/icons/status-login.svg'
import success from '../../assets/icons/status-success.svg'
import pull from '../../assets/icons/status-pull.svg'
import './status-glyph.scss'

const symbols = { empty, error, offline, forbidden, login, success, pull }
export type StatusGlyphKind = keyof typeof symbols | 'loading'

export function StatusGlyph({ kind, small = false }: { kind: StatusGlyphKind; small?: boolean }) {
  return <View className={`status-glyph status-glyph--${kind}${small ? ' status-glyph--small' : ''}`}>
    {(kind === 'loading' || kind === 'login') && <View className='status-glyph__ring' />}
    {kind !== 'loading' && <Image className='status-glyph__image' src={symbols[kind]} mode='aspectFit' />}
  </View>
}
