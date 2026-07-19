const cp = require('node:child_process')
const path = require('node:path')

const root = path.resolve(__dirname, '..')

function git(args, allowNoMatches = false) {
  try {
    return cp.execFileSync('git', args, {
      cwd: root,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      maxBuffer: 16 * 1024 * 1024
    })
  } catch (error) {
    if (allowNoMatches && error && error.status === 1) return ''
    throw new Error('unable to inspect tracked files for WeChat secret hygiene')
  }
}

const files = git(['ls-files', '-z']).split('\0').filter(Boolean)
const configurationFiles = files.filter((file) => (
  /(^|\/)\.env(?:$|\.)/.test(file) ||
  /(^|\/)project(?:\.private)?\.config(?:\.[^/]+)?\.json$/.test(file)
))
const violations = []

function isExplicitPlaceholder(value) {
  const normalized = value.trim().replace(/^(["'])(.*)\1$/, '$2').trim()
  if (normalized === '') return true
  if (/^\$\{[^}]+\}$/.test(normalized) || /^<[^>]+>$/.test(normalized)) return true
  if (/[.*+?^${}()[\]|\\]/.test(normalized)) return true
  return /(?:^|[-_])(dev|test|local|example|sample|dummy|placeholder|change|replace|not-for-production)(?:[-_]|$)/i.test(normalized)
}

function looksLikeWechatAppSecret(value) {
  const normalized = value.trim().replace(/^(["'])(.*)\1$/, '$2').trim()
  return /^[A-Za-z0-9]{32}$/.test(normalized)
}

for (const file of configurationFiles) {
  const text = git(['show', `:${file}`])
  for (const match of text.matchAll(/^\s*WECHAT_APP_SECRET\s*=\s*(.*?)\s*$/gm)) {
    const exampleFile = /(?:^|\/)\.env\.example$/.test(file)
    if (!isExplicitPlaceholder(match[1]) && (!exampleFile || looksLikeWechatAppSecret(match[1]))) {
      violations.push(file)
    }
  }
}

const documentationMatches = git([
  'grep', '--cached', '-l', '-I', '-E',
  'WECHAT_APP_SECRET[[:space:]]*=[[:space:]]*["' + "'" + ']?[A-Za-z0-9]{32}([^A-Za-z0-9]|$)',
  '--', 'docs'
], true).trim().split(/\r?\n/).filter(Boolean)
violations.push(...documentationMatches)

if (violations.length > 0) {
  throw new Error(`tracked WeChat secret assignment detected in: ${Array.from(new Set(violations)).join(', ')}`)
}

console.log('real wechat secret hygiene: ok')
