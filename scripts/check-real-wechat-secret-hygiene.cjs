const cp = require('node:child_process')
const path = require('node:path')
const { TextDecoder } = require('node:util')

const root = path.resolve(__dirname, '..')
const maxGitOutputBytes = 256 * 1024 * 1024
const secretIdentifierPattern = /(?:WECHAT[_-]?APP[_-]?SECRET|APP[_-]?SECRET)/i
const secretShapePattern = /(^|[^A-Za-z0-9])[A-Za-z0-9]{32}(?=$|[^A-Za-z0-9])/

function git(args, allowNoMatches = false) {
  try {
    return cp.execFileSync('git', args, {
      cwd: root,
      encoding: null,
      stdio: ['ignore', 'pipe', 'pipe'],
      maxBuffer: maxGitOutputBytes
    })
  } catch (error) {
    if (allowNoMatches && error && error.status === 1) return Buffer.alloc(0)
    throw new Error('unable to inspect Git index for WeChat secret hygiene')
  }
}

function decodeUTF16BE(buffer, offset) {
  const length = buffer.length - offset
  if (length % 2 !== 0) return null
  const swapped = Buffer.alloc(length - (length % 2))
  for (let index = 0; index < swapped.length; index += 2) {
    swapped[index] = buffer[offset + index + 1]
    swapped[index + 1] = buffer[offset + index]
  }
  return swapped.toString('utf16le')
}

function detectBOMlessUTF16(buffer) {
  if (buffer.length < 8 || buffer.length % 2 !== 0) return null
  const sampleLength = Math.min(buffer.length, 64 * 1024)
  const evenLength = sampleLength - (sampleLength % 2)
  let evenNuls = 0
  let oddNuls = 0
  for (let index = 0; index < evenLength; index += 2) {
    if (buffer[index] === 0) evenNuls++
    if (buffer[index + 1] === 0) oddNuls++
  }

  const pairs = evenLength / 2
  const minimumNuls = Math.max(4, Math.ceil(pairs * 0.05))
  const maximumOppositeNuls = Math.max(1, Math.floor(pairs * 0.01))
  if (oddNuls >= minimumNuls && evenNuls <= maximumOppositeNuls) return 'utf16le'
  if (evenNuls >= minimumNuls && oddNuls <= maximumOppositeNuls) return 'utf16be'
  return null
}

function decodeText(buffer) {
  if (buffer.length === 0) return ''
  if (buffer.length >= 2 && buffer[0] === 0xff && buffer[1] === 0xfe) {
    if ((buffer.length - 2) % 2 !== 0) return null
    return buffer.subarray(2).toString('utf16le')
  }
  if (buffer.length >= 2 && buffer[0] === 0xfe && buffer[1] === 0xff) {
    return decodeUTF16BE(buffer, 2)
  }

  const bomlessUTF16 = detectBOMlessUTF16(buffer)
  if (bomlessUTF16 === 'utf16le') return buffer.toString('utf16le')
  if (bomlessUTF16 === 'utf16be') return decodeUTF16BE(buffer, 0)

  const sample = buffer.subarray(0, Math.min(buffer.length, 8192))
  let controlBytes = 0
  for (const value of sample) {
    if (value === 0) return null
    if (value < 9 || (value > 13 && value < 32)) controlBytes++
  }
  if (sample.length > 0 && controlBytes / sample.length > 0.1) return null
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(buffer)
  } catch {
    return null
  }
}

function stripInlineEnvComment(value) {
  let quote = ''
  for (let index = 0; index < value.length; index++) {
    const current = value[index]
    if (quote !== '') {
      if (current === quote && value[index - 1] !== '\\') quote = ''
      continue
    }
    if (current === '"' || current === "'") {
      quote = current
      continue
    }
    if (current === '#' && (index === 0 || /\s/.test(value[index - 1]))) {
      return value.slice(0, index).trim()
    }
  }
  return value.trim()
}

function normalizeLiteral(value) {
  let normalized = stripInlineEnvComment(value).trim().replace(/[,;]\s*$/, '').trim()
  if (normalized.length >= 2) {
    const quote = normalized[0]
    if ((quote === '"' || quote === "'") && normalized[normalized.length - 1] === quote) {
      normalized = normalized.slice(1, -1).trim()
    }
  }
  return normalized
}

function isExplicitPlaceholder(value) {
  const normalized = normalizeLiteral(value)
  if (normalized === '') return true
  if (/^[A-Za-z0-9]{32}$/.test(normalized)) return false
  if (/^\$\{[^}]+\}$/.test(normalized) || /^<[^>]+>$/.test(normalized)) return true
  if (/[^\x00-\x7f]/.test(normalized)) return true
  if (/^(?:YOUR_WECHAT_APP_SECRET|WECHAT_APP_SECRET_HERE|changeme|change[-_]?me|example|sample|dummy|placeholder|replace[-_]?me|secret|short)$/i.test(normalized)) return true
  if (/(?:^|[-_])(dev|test|local|example|sample|dummy|placeholder|change|replace|not-for-production)(?:[-_]|$)/i.test(normalized)) return true
  return /(?:^|[^A-Za-z0-9])secret(?:[^A-Za-z0-9]|$)/i.test(normalized)
}

function structuredConfigFormat(file) {
  const normalized = file.replace(/\\/g, '/').toLowerCase()
  if (/\.ps1$/.test(normalized)) return 'powershell'
  if (/\.ya?ml$/.test(normalized)) return 'yaml'
  if (/\.jsonc?$/.test(normalized) || /\.json5$/.test(normalized)) return 'json'
  return null
}

function yamlScalar(value) {
  const normalized = stripInlineEnvComment(value)
  const quoted = normalized.match(/^(["'])(.*?)\1$/)
  if (quoted) return quoted[2]
  if (/^\$\{[^}]+\}$/.test(normalized) || /^<[^>]+>$/.test(normalized)) return normalized
  return /^[A-Za-z0-9_./+@=-]+$/.test(normalized) ? normalized : null
}

function jsonScalar(value) {
  const normalized = value.trim()
  const quoted = normalized.match(/^(["'])(.*?)\1\s*[,}]?\s*$/)
  if (quoted) return quoted[2]
  const variable = normalized.match(/^(\$\{[^}]+\})\s*[,}]?\s*$/)
  if (variable) return variable[1]
  const anglePlaceholder = normalized.match(/^(<[^>]+>)\s*[,}]?\s*$/)
  if (anglePlaceholder) return anglePlaceholder[1]
  const plain = normalized.match(/^([A-Za-z0-9_./+@=-]+)\s*[,}]?\s*$/)
  return plain ? plain[1] : null
}

function assignedLiterals(line, file) {
  const values = []
  const envMatch = line.match(/^\s*(?:(?:export\s+)?WECHAT_APP_SECRET|\$env:WECHAT_APP_SECRET|\$\{env:WECHAT_APP_SECRET\})\s*=\s*(.*)$/i)
  if (envMatch) values.push(stripInlineEnvComment(envMatch[1]))

  const mappingMatch = line.match(/^\s*["']?WECHAT_APP_SECRET["']?\s*:\s*(.*)$/i)
  if (mappingMatch) values.push(stripInlineEnvComment(mappingMatch[1]))

  const genericLiteral = /(?:Wechat[_-]?App[_-]?Secret|App[_-]?Secret)\s*["']?\s*(?::=|=|:)\s*(["'])(.*?)\1/gi
  for (const match of line.matchAll(genericLiteral)) values.push(match[2])

  const setenvLiteral = /Setenv\(\s*["']WECHAT_APP_SECRET["']\s*,\s*(["'])(.*?)\1/gi
  for (const match of line.matchAll(setenvLiteral)) values.push(match[2])

  const format = structuredConfigFormat(file)
  const genericKey = secretIdentifierPattern.source
  if (format === 'yaml') {
    const match = line.match(new RegExp(`^\\s*["']?${genericKey}["']?\\s*:\\s*(.+?)\\s*$`, 'i'))
    const value = match ? yamlScalar(match[1]) : null
    if (value !== null) values.push(value)
  } else if (format === 'json') {
    const match = line.match(new RegExp(`^\\s*(?:[,{}]\\s*)?["']${genericKey}["']\\s*:\\s*(.+?)\\s*$`, 'i'))
    const value = match ? jsonScalar(match[1]) : null
    if (value !== null) values.push(value)
  } else if (format === 'powershell') {
    const match = line.match(new RegExp(`^\\s*\\$?${genericKey}\\s*=\\s*(.+?)\\s*$`, 'i'))
    if (match) values.push(stripInlineEnvComment(match[1]))
  }
  return values
}

function adjacentAssignedLiteral(lines, index, file) {
  const nextLine = lines[index + 1]
  if (nextLine === undefined || nextLine.trim() === '') return null

  const line = lines[index]
  const format = structuredConfigFormat(file)
  const genericKey = secretIdentifierPattern.source
  if (format === 'yaml') {
    const keyMatch = line.match(new RegExp(`^(\\s*)["']?${genericKey}["']?\\s*:\\s*$`, 'i'))
    if (!keyMatch) return null
    const nextIndent = nextLine.match(/^\s*/)[0]
    if (nextIndent.length <= keyMatch[1].length) return null
    return yamlScalar(nextLine.trim())
  }
  if (format === 'json') {
    const keyMatch = line.match(new RegExp(`^\\s*(?:[,{}]\\s*)?["']${genericKey}["']\\s*:\\s*$`, 'i'))
    return keyMatch ? jsonScalar(nextLine) : null
  }
  return null
}

function isAssignedSecret(value) {
  const normalized = normalizeLiteral(value)
  if (/^[A-Za-z0-9]{32}$/.test(normalized)) return true
  return !isExplicitPlaceholder(normalized)
}

function containsSecret(text, file) {
  const lines = text.split(/\r?\n/)
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index]
    if (!secretIdentifierPattern.test(line)) continue

    if (secretShapePattern.test(line)) return true
    if (assignedLiterals(line, file).some(isAssignedSecret)) return true
    const adjacent = adjacentAssignedLiteral(lines, index, file)
    if (adjacent !== null && isAssignedSecret(adjacent)) return true
  }
  return false
}

function runSelfTests() {
  const secret = 'A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6'
  const envSecretName = 'WECHAT_APP_' + 'SECRET'
  const jsonSecretName = 'app' + 'Secret'
  const realLikeValue = 'prodCredentialValue123'
  const fixtures = [
    { path: 'deploy/start.ps1', text: `$env:WECHAT_APP_SECRET='${secret}'`, violation: true },
    { path: 'deploy/prod.yml', text: `WECHAT_APP_SECRET: ${secret}`, violation: true },
    { path: 'backend/.env.example', text: `WECHAT_APP_SECRET=${secret} # local only`, violation: true },
    { path: 'src/config.json', text: `{"appSecret":"${secret}"}`, violation: true },
    { path: 'deploy/start.ps1', text: `$env:${envSecretName}=${realLikeValue}`, violation: true },
    { path: 'deploy/start-braced.ps1', text: `\${env:${envSecretName}}='${realLikeValue}'`, violation: true },
    { path: 'deploy/prod.yml', text: `${envSecretName}: ${realLikeValue}`, violation: true },
    { path: 'deploy/generic.yml', text: `${jsonSecretName}: ${realLikeValue}`, violation: true },
    { path: 'deploy/next-line.yml', text: `${envSecretName}:\n  ${realLikeValue}`, violation: true },
    { path: 'deploy/next-line-32.yml', text: `${envSecretName}:\n  ${secret}`, violation: true },
    { path: '.env.example', text: `${envSecretName}=${realLikeValue}`, violation: true },
    { path: 'src/config.json', text: JSON.stringify({ [jsonSecretName]: realLikeValue }), violation: true },
    { path: 'src/config-next-line.json', text: `{"${jsonSecretName}":\n  "${realLikeValue}"}`, violation: true },
    { path: 'src/config-next-line-unquoted.json', text: `{"${jsonSecretName}":\n  ${realLikeValue}}`, violation: true },
    { path: '.env.production.example', text: 'WECHAT_APP_SECRET=YOUR_WECHAT_APP_SECRET', violation: false },
    { path: '.env.production.example', text: 'WECHAT_APP_SECRET=changeme', violation: false },
    { path: '.env.example', text: 'WECHAT_APP_SECRET=change-me', violation: false },
    { path: '.env.stage.example', text: 'WECHAT_APP_SECRET=example', violation: false },
    { path: 'docs/example.md', text: 'WECHAT_APP_SECRET=<YOUR_WECHAT_APP_SECRET>', violation: false },
    { path: 'deploy/next-line-placeholder.yml', text: 'WECHAT_APP_SECRET:\n  YOUR_WECHAT_APP_SECRET', violation: false },
    { path: 'src/config-next-line-placeholder.json', text: '{"appSecret":\n  "changeme"}', violation: false },
    { path: 'docs/adjacent.md', text: `The APP_SECRET field is documented below.\n${secret}`, violation: false },
    { path: 'deploy/not-adjacent.yml', text: `WECHAT_APP_SECRET:\n\n  ${realLikeValue}`, violation: false }
  ]
  for (const fixture of fixtures) {
    if (containsSecret(fixture.text, fixture.path) !== fixture.violation) {
      throw new Error(`secret hygiene self-test failed for ${fixture.path}`)
    }
  }
  if (normalizeLiteral(`'${secret}' # local only`) !== secret) {
    throw new Error('secret hygiene inline comment self-test failed')
  }

  const utf16Text = `$env:WECHAT_APP_SECRET='${secret}'`
  const decodedUTF16LE = decodeText(Buffer.from(utf16Text, 'utf16le'))
  if (decodedUTF16LE !== utf16Text || !containsSecret(decodedUTF16LE, 'deploy/start-utf16le.ps1')) {
    throw new Error('secret hygiene BOM-less UTF-16LE self-test failed')
  }
  const utf16BE = Buffer.from(utf16Text, 'utf16le')
  for (let index = 0; index < utf16BE.length; index += 2) {
    const first = utf16BE[index]
    utf16BE[index] = utf16BE[index + 1]
    utf16BE[index + 1] = first
  }
  const decodedUTF16BE = decodeText(utf16BE)
  if (decodedUTF16BE !== utf16Text || !containsSecret(decodedUTF16BE, 'deploy/start-utf16be.ps1')) {
    throw new Error('secret hygiene BOM-less UTF-16BE self-test failed')
  }
  if (decodeText(Buffer.from([0, 1, 2, 3, 0, 255, 4, 5])) !== null) {
    throw new Error('secret hygiene binary self-test failed')
  }
}

function indexEntries() {
  return git(['ls-files', '--stage', '-z']).toString('utf8').split('\0').filter(Boolean).flatMap((entry) => {
    const match = entry.match(/^(\d+) ([0-9a-f]+) (\d+)\t([\s\S]*)$/)
    if (!match || match[3] !== '0' || match[1] === '160000') return []
    return [{ objectID: match[2], path: match[4] }]
  })
}

function readIndexBlobs(entries) {
  if (entries.length === 0) return []
  let output
  try {
    output = cp.execFileSync('git', ['cat-file', '--batch'], {
      cwd: root,
      encoding: null,
      input: Buffer.from(`${entries.map((entry) => entry.objectID).join('\n')}\n`),
      stdio: ['pipe', 'pipe', 'pipe'],
      maxBuffer: maxGitOutputBytes
    })
  } catch {
    throw new Error('unable to inspect Git index for WeChat secret hygiene')
  }

  let offset = 0
  return entries.map((entry) => {
    const headerEnd = output.indexOf(0x0a, offset)
    if (headerEnd < 0) throw new Error('unable to inspect Git index for WeChat secret hygiene')
    const header = output.subarray(offset, headerEnd).toString('ascii')
    const match = header.match(/^([0-9a-f]+) blob (\d+)$/)
    const size = match ? Number(match[2]) : Number.NaN
    const contentStart = headerEnd + 1
    const contentEnd = contentStart + size
    if (!match || match[1] !== entry.objectID || !Number.isSafeInteger(size) || output[contentEnd] !== 0x0a) {
      throw new Error('unable to inspect Git index for WeChat secret hygiene')
    }
    offset = contentEnd + 1
    return { entry, buffer: output.subarray(contentStart, contentEnd) }
  })
}

function inspectIndex() {
  const violations = []
  for (const { entry, buffer } of readIndexBlobs(indexEntries())) {
    const text = decodeText(buffer)
    if (text !== null && containsSecret(text, entry.path)) violations.push(entry.path)
  }
  return Array.from(new Set(violations))
}

runSelfTests()
const violations = inspectIndex()
if (violations.length > 0) {
  throw new Error(`tracked WeChat AppSecret detected in: ${violations.join(', ')}`)
}
console.log('real wechat secret hygiene: ok')
