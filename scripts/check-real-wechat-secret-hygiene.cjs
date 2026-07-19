const cp = require('node:child_process')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const maxGitObjectBytes = 32 * 1024 * 1024
const secretIdentifierPattern = /(?:WECHAT[_-]?APP[_-]?SECRET|APP[_-]?SECRET)/i
const secretShapePattern = /(^|[^A-Za-z0-9])[A-Za-z0-9]{32}(?=$|[^A-Za-z0-9])/

function git(args, allowNoMatches = false) {
  try {
    return cp.execFileSync('git', args, {
      cwd: root,
      encoding: null,
      stdio: ['ignore', 'pipe', 'pipe'],
      maxBuffer: maxGitObjectBytes
    })
  } catch (error) {
    if (allowNoMatches && error && error.status === 1) return Buffer.alloc(0)
    throw new Error('unable to inspect Git index for WeChat secret hygiene')
  }
}

function decodeUTF16BE(buffer, offset) {
  const length = buffer.length - offset
  const swapped = Buffer.alloc(length - (length % 2))
  for (let index = 0; index < swapped.length; index += 2) {
    swapped[index] = buffer[offset + index + 1]
    swapped[index + 1] = buffer[offset + index]
  }
  return swapped.toString('utf16le')
}

function decodeText(buffer) {
  if (buffer.length === 0) return ''
  if (buffer.length >= 2 && buffer[0] === 0xff && buffer[1] === 0xfe) {
    return buffer.subarray(2).toString('utf16le')
  }
  if (buffer.length >= 2 && buffer[0] === 0xfe && buffer[1] === 0xff) {
    return decodeUTF16BE(buffer, 2)
  }

  const sample = buffer.subarray(0, Math.min(buffer.length, 8192))
  let controlBytes = 0
  for (const value of sample) {
    if (value === 0) return null
    if (value < 9 || (value > 13 && value < 32)) controlBytes++
  }
  if (sample.length > 0 && controlBytes / sample.length > 0.1) return null
  return buffer.toString('utf8')
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
  return /(?:^|[-_])secret(?:[-_]|$)/i.test(normalized)
}

function assignedLiterals(line) {
  const values = []
  const envMatch = line.match(/^\s*(?:(?:export\s+)|(?:\$env:))?WECHAT_APP_SECRET\s*=\s*(.*)$/i)
  if (envMatch) values.push(stripInlineEnvComment(envMatch[1]))

  const mappingMatch = line.match(/^\s*["']?WECHAT_APP_SECRET["']?\s*:\s*(.*)$/i)
  if (mappingMatch) values.push(stripInlineEnvComment(mappingMatch[1]))

  const genericLiteral = /(?:Wechat[_-]?App[_-]?Secret|App[_-]?Secret)\s*["']?\s*(?::=|=|:)\s*(["'])(.*?)\1/gi
  for (const match of line.matchAll(genericLiteral)) values.push(match[2])

  const setenvLiteral = /Setenv\(\s*["']WECHAT_APP_SECRET["']\s*,\s*(["'])(.*?)\1/gi
  for (const match of line.matchAll(setenvLiteral)) values.push(match[2])
  return values
}

function isEnvFile(file) {
  return /(^|\/)\.env(?:$|\.)/.test(file)
}

function isExampleEnvFile(file) {
  return /(^|\/)\.env(?:\.[^/]+)*\.example$/.test(file)
}

function containsSecret(text, file) {
  const lines = text.split(/\r?\n/)
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index]
    if (!secretIdentifierPattern.test(line)) continue

    const nearby = line + (index + 1 < lines.length ? `\n${lines[index + 1]}` : '')
    if (secretShapePattern.test(nearby)) return true
    if (isEnvFile(file) && !isExampleEnvFile(file) && assignedLiterals(line).some((value) => !isExplicitPlaceholder(value))) return true
  }
  return false
}

function runSelfTests() {
  const secret = 'A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6'
  const fixtures = [
    { path: 'deploy/start.ps1', text: `$env:WECHAT_APP_SECRET='${secret}'`, violation: true },
    { path: 'deploy/prod.yml', text: `WECHAT_APP_SECRET: ${secret}`, violation: true },
    { path: 'backend/.env.example', text: `WECHAT_APP_SECRET=${secret} # local only`, violation: true },
    { path: 'src/config.json', text: `{"appSecret":"${secret}"}`, violation: true },
    { path: '.env.production.example', text: 'WECHAT_APP_SECRET=YOUR_WECHAT_APP_SECRET', violation: false },
    { path: '.env.production.example', text: 'WECHAT_APP_SECRET=changeme', violation: false },
    { path: '.env.example', text: 'WECHAT_APP_SECRET=change-me', violation: false },
    { path: '.env.stage.example', text: 'WECHAT_APP_SECRET=example', violation: false },
    { path: 'docs/example.md', text: 'WECHAT_APP_SECRET=<YOUR_WECHAT_APP_SECRET>', violation: false }
  ]
  for (const fixture of fixtures) {
    if (containsSecret(fixture.text, fixture.path) !== fixture.violation) {
      throw new Error(`secret hygiene self-test failed for ${fixture.path}`)
    }
  }
  if (normalizeLiteral(`'${secret}' # local only`) !== secret) {
    throw new Error('secret hygiene inline comment self-test failed')
  }
}

function candidateTextFiles() {
  return git([
    'grep', '--cached', '-z', '-I', '-i', '-l',
    '-E', '-e', 'WECHAT[_-]?APP[_-]?SECRET', '-e', 'APP[_-]?SECRET', '--'
  ], true).toString('utf8').split('\0').filter(Boolean)
}

function inspectIndex() {
  const violations = []
  for (const file of candidateTextFiles()) {
    const text = decodeText(git(['show', `:${file}`]))
    if (text !== null && containsSecret(text, file)) violations.push(file)
  }
  return Array.from(new Set(violations))
}

runSelfTests()
const violations = inspectIndex()
if (violations.length > 0) {
  throw new Error(`tracked WeChat AppSecret detected in: ${violations.join(', ')}`)
}
console.log('real wechat secret hygiene: ok')
