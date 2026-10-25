/* Build-time, color-only adaptation of the real page styles. Never runs on-device. */
const fs = require('node:fs')
const path = require('node:path')
const sass = require('sass')
const postcss = require('postcss')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps/miniapp/src')
const target = path.join(src, 'styles/system-dark-colors.generated.scss')
const config = fs.readFileSync(path.join(src, 'app.config.ts'), 'utf8')
const pages = [...config.matchAll(/'(pages\/[^']+)'/g)].map(match => match[1])
const seen = new Set()
const styles = new Set()

function visit(file) {
  if (seen.has(file)) return
  seen.add(file)
  const source = fs.readFileSync(file, 'utf8')
  for (const match of source.matchAll(/(?:import|export)\s+(?:[^'";]*?\s+from\s*)?['"]([^'"]+)['"]/g)) {
    if (!match[1].startsWith('.')) continue
    const base = path.resolve(path.dirname(file), match[1])
    if (base.endsWith('.scss')) { styles.add(base); continue }
    const child = [base, `${base}.tsx`, `${base}.ts`, `${base}.js`, path.join(base, 'index.tsx'), path.join(base, 'index.ts')].find(candidate => fs.existsSync(candidate) && fs.statSync(candidate).isFile())
    if (child && /\.[jt]sx?$/.test(child)) visit(child)
  }
}
visit(path.join(src, 'app.tsx'))
for (const page of pages) visit(path.join(src, `${page}.tsx`))

const palette = {
  '#ffffff': '#18212c', '#f2f5f9': '#111720', '#f7f8fa': '#202a36',
  '#f7f9fc': '#202a36', '#edf1f6': '#2c3949', '#dfe7f0': '#344253',
  '#c9d5e3': '#435569', '#e6f1fb': '#203a53',
  '#1a2b3c': '#edf3fa', '#2a3a4a': '#d6e0ec', '#6b7c8f': '#a9b8ca',
  '#93a2b4': '#91a4bb', '#9db2c8': '#9bb0c8', '#b0becd': '#91a4bb',
  '#2e7dd4': '#80b9f3', '#2168b8': '#8dc2f7', '#3487de': '#82bdf8',
  '#256fbd': '#8dc2f7', '#1f61a6': '#a3d0ff',
  '#168a5b': '#65cba3', '#b7791f': '#e5bb71', '#c43d4b': '#ff939c',
  '#e8f6f0': '#183b31', '#fff6df': '#3b3020', '#fdedef': '#40272e',
  '#292824': '#eee9e2', '#74706a': '#bdb5ab', '#8a6544': '#d2b18f', '#eeeae5': '#3b3833'
}
const colorProperties = /^(?:color|background(?:-color|-image)?|border(?:-(?:top|right|bottom|left))?(?:-color)?|outline(?:-color)?|box-shadow|text-shadow|fill|stroke|caret-color)$/
const glassSelectors = new Set(['.student-bottom-nav', '.delivery-workspace-nav', '.app-dialog', '.app-action-sheet', '.refresh-feedback'])

function rgba(token) {
  if (token.toLowerCase() === 'white') return [255, 255, 255, 1]
  if (token.toLowerCase() === 'black') return [0, 0, 0, 1]
  if (token[0] === '#') {
    let hex = token.slice(1)
    if (hex.length <= 4) hex = [...hex].map(char => char + char).join('')
    if (hex.length !== 6 && hex.length !== 8) return null
    return [0, 2, 4].map(index => parseInt(hex.slice(index, index + 2), 16)).concat(hex.length === 8 ? parseInt(hex.slice(6), 16) / 255 : 1)
  }
  const parts = token.match(/[\d.]+/g)?.map(Number)
  return parts?.length >= 3 ? [...parts.slice(0, 3), parts[3] ?? 1] : null
}
function hex(channels) { return '#' + channels.slice(0, 3).map(n => Math.round(n).toString(16).padStart(2, '0')).join('') }
function mapped(token, property) {
  const channels = rgba(token)
  if (!channels) return token
  const [r, g, b, alpha] = channels
  const key = hex(channels)
  const hi = Math.max(r, g, b), lo = Math.min(r, g, b)
  const brightness = (hi + lo) / 510
  const saturation = hi - lo
  const foreground = /^(color|fill|stroke|caret-color)$/.test(property)
  const line = /border|outline/.test(property)
  const shadow = /shadow/.test(property)
  let replacement
  if (foreground) {
    // White is intentional on a photograph, video, mask or filled action button.
    if (lo >= 230) return token
    replacement = palette[key]
    if (!replacement) {
      if (saturation < 48 || (b > r && b > g && brightness < .37)) replacement = brightness < .32 ? '#e6edf5' : '#a9b8ca'
      else if (g > r * 1.1 && g > b) replacement = '#69cfaa'
      else if (b > r * 1.1) replacement = '#85bdf5'
      else if (r > b * 1.4 && g > b * 1.3) replacement = '#edbc7b'
      else replacement = '#ff939c'
    }
  } else if (shadow) {
    replacement = lo > 220 ? '#18212c' : '#000000'
  } else if (line) {
    replacement = saturation < 55 && brightness > .57 ? '#344253' : (brightness > .86 ? '#344253' : undefined)
  } else {
    // Preserve strong brand/action colors and already dark media backdrops.
    if (brightness < .72) return token
    replacement = palette[key]
    if (!replacement) {
      if (saturation < 15) replacement = '#202a36'
      else if (r > g + 5 && g > b + 5) replacement = '#3b3020'
      else if (r > g + 8 && r > b + 2) replacement = '#40272e'
      else if (g > r + 5 && g > b) replacement = '#183b31'
      else replacement = '#203044'
    }
  }
  if (!replacement || replacement === key) return token
  if (alpha === 1) return replacement
  const replacementChannels = rgba(replacement).slice(0, 3)
  return `rgba(${replacementChannels.join(', ')}, ${alpha})`
}

function mapValue(value, property) {
  // URLs and complete var(...) fallbacks are runtime-owned and not recolored.
  const preserved = []
  let result = value.replace(/url\([^)]*\)|var\([^)]*\)/gi, token => `__PRESERVED_${preserved.push(token) - 1}__`)
  result = result.replace(/#[a-f\d]{3,8}\b|rgba?\([^)]*\)|\b(?:white|black)\b/gi, token => mapped(token, property))
  result = result.replace(/__PRESERVED_(\d+)__/g, (_, index) => preserved[Number(index)])
  // Brand-filled controls keep the original brand blue and their white labels.
  // Only an explicit foreground use receives the brighter night accent.
  if (/^(?:color|fill|stroke|caret-color)$/.test(property)) {
    result = result.replace(/var\(--brand-primary(?:-(?:strong|light|hover|pressed))?(?:,[^)]*)?\)/g, '#80b9f3')
  }
  return result
}
function prefix(selector) {
  return selector.split(',').map(part => {
    const trimmed = part.trim()
    return /^(page|:root|html)(?:\b|[:.#\[])/.test(trimmed) ? trimmed.replace(/^(:root|html)\b/, 'page') : `page ${trimmed}`
  }).join(', ')
}

const output = []
const emitted = new Set()
let changed = 0
for (const file of [...styles].sort()) {
  // Surface enhancements use explicit semantic runtime variables, not inferred overrides.
  if (/system-dark|surface-enhancements/.test(file)) continue
  const source = fs.readFileSync(file, 'utf8').replace(/^\s*@use[^;]*(?:system-dark-theme|surface-enhancements)[^;]*;/gm, '')
  const css = sass.compileString(source, { loadPaths: [path.dirname(file)], logger: sass.Logger.silent }).css
  const ast = postcss.parse(css)
  const chunks = []
  ast.walkRules(rule => {
    if (rule.selector === 'page') return // Explicit theme variables below own the page.
    let current = rule.parent
    const contexts = []
    while (current?.type === 'atrule') {
      if (/keyframes/.test(current.name) || /prefers-color-scheme/.test(current.params)) return
      contexts.unshift(`@${current.name} ${current.params}`)
      current = current.parent
    }
    const declarations = []
    rule.nodes.filter(node => node.type === 'decl').forEach(decl => {
      if (!colorProperties.test(decl.prop) && !/^--guide-/.test(decl.prop)) return
      if (/shadow/.test(decl.prop)) return
      if (/^background/.test(decl.prop) && rule.selector.split(',').every(selector => [...glassSelectors].some(glass => selector.trim().endsWith(glass)))) return
      const role = decl.prop.startsWith('--guide-') ? (/line/.test(decl.prop) ? 'border-color' : 'color') : decl.prop
      const value = mapValue(decl.value, role)
      if (value === decl.value && !decl.prop.startsWith('--guide-')) return
      // Replay unmodified on-color labels and variant colors as well: prefixing
      // only a base rule would otherwise overpower its original active variant.
      // URL assets remain owned by their source file (no relative-path rewrite).
      // The app stylesheet is emitted before individual page bundles. Keep
      // dark semantic overrides effective when a later light rule has the
      // same selector; media/user assets are still excluded above.
      if (!/url\(/i.test(value)) declarations.push(`${decl.prop}: ${value} !important;`)
    })
    if (!declarations.length) return
    let chunk = `${prefix(rule.selector)} { ${declarations.join(' ')} }`
    for (let i = contexts.length - 1; i >= 0; i--) chunk = `${contexts[i]} {\n  ${chunk}\n}`
    // A mixin imported by several files produces the same declarations; emit once.
    if (emitted.has(chunk)) return
    emitted.add(chunk)
    changed += declarations.length
    chunks.push(chunk)
  })
  if (chunks.length) output.push(`/* ${path.relative(src, file).split(path.sep).join('/')} */\n${chunks.join('\n')}`)
}
const content = `// Generated by scripts/generate-miniapp-dark-theme.cjs; do not edit by hand.\n// ${pages.length} registered pages; ${styles.size} source style entries; ${changed} color declarations.\n// Layout, images, white on-color text, timers and event handlers are unchanged.\n@media (prefers-color-scheme: dark) {\n${output.join('\n\n')}\n}\n`
if (process.argv.includes('--patch')) {
  const exists = fs.existsSync(target)
  process.stdout.write('*** Begin Patch\n' + (exists ? `*** Delete File: ${target}\n` : '') + `*** Add File: ${target}\n` + content.split('\n').filter((_, i, arr) => i < arr.length - 1).map(line => '+' + line).join('\n') + '\n*** End Patch\n')
} else if (process.argv.includes('--check')) {
  if (!fs.existsSync(target) || fs.readFileSync(target, 'utf8').replace(/\r\n/g, '\n') !== content) {
    console.error('Dark theme is out of date. Run node scripts/generate-miniapp-dark-theme.cjs --write after changing source styles.')
    process.exitCode = 1
  } else console.log(`dark theme: ${pages.length} pages, ${styles.size} style entries, ${changed} color declarations; current`)
} else if (process.argv.includes('--write')) {
  fs.writeFileSync(target, content)
  console.log(`generated ${path.relative(root, target)} (${pages.length} pages, ${changed} color declarations)`)
} else {
  console.log('Usage: node scripts/generate-miniapp-dark-theme.cjs --check | --write | --patch')
}
