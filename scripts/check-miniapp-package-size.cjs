const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const defaultDist = path.join(root, 'apps/miniapp/dist')
const packageLimit = 2 * 1024 * 1024
const totalLimit = 20 * 1024 * 1024

function asPosix(value) { return String(value).replaceAll('\\', '/') }
function normalizeRelative(value, label) {
  if (typeof value !== 'string' || !value.trim()) throw new Error(`${label} must be a non-empty relative path`)
  const raw = asPosix(value.trim())
  if (raw.startsWith('/') || /^[A-Za-z]:/.test(raw)) throw new Error(`${label} must not be absolute`)
  if (raw.split('/').includes('..')) throw new Error(`${label} must not contain ..`)
  const normalized = path.posix.normalize(raw)
  if (normalized === '.' || normalized === '..' || normalized.startsWith('../') || normalized.includes('/../')) throw new Error(`${label} must not escape dist`)
  return normalized.replace(/^\.\//, '').replace(/\/$/, '')
}
function isWithin(relative, directory) { return relative === directory || relative.startsWith(`${directory}/`) }
function filesIn(directory, relative = '') {
  if (!fs.existsSync(directory)) return []
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const absolute = path.join(directory, entry.name)
    const next = relative ? `${relative}/${entry.name}` : entry.name
    if (entry.isSymbolicLink()) throw new Error(`symbolic links are not allowed in dist: ${next}`)
    if (entry.isDirectory()) return filesIn(absolute, next)
    if (!entry.isFile()) throw new Error(`unsupported dist entry: ${next}`)
    return [next]
  })
}
function readJSON(file, label) {
  try { return JSON.parse(fs.readFileSync(file, 'utf8')) } catch (error) { throw new Error(`${label} is missing or invalid JSON: ${error instanceof Error ? error.message : String(error)}`) }
}
function pageArtifact(files, page, label) {
  if (!files.has(`${page}.js`) || !files.has(`${page}.json`) || !files.has(`${page}.wxml`)) throw new Error(`${label} is missing page output: ${page}`)
}
function analyzePackage(distDir = defaultDist) {
  const appJSONPath = path.join(distDir, 'app.json')
  if (!fs.existsSync(appJSONPath)) throw new Error('dist is missing app.json')
  const manifest = readJSON(appJSONPath, 'app.json')
  const files = filesIn(distDir)
  const fileSet = new Set(files)
  for (const required of ['app.js', 'common.js', 'vendors.js']) if (!fileSet.has(required)) throw new Error(`dist is missing required build output: ${required}`)
  const mainPages = Array.isArray(manifest.pages) ? manifest.pages : []
  if (!mainPages.length) throw new Error('app.json.pages must be a non-empty array')
  if (manifest.subPackages != null && manifest.subpackages != null) throw new Error('app.json must not define both subPackages and subpackages')
  const subPackages = manifest.subPackages ?? manifest.subpackages ?? []
  if (!Array.isArray(subPackages)) throw new Error('app.json.subPackages must be an array')
  const packages = [{ name: 'main', root: '', pages: mainPages }]
  for (let index = 0; index < subPackages.length; index++) {
    const item = subPackages[index]
    if (!item || typeof item !== 'object') throw new Error(`subPackages[${index}] must be an object`)
    const rootName = normalizeRelative(item.root, `subPackages[${index}].root`)
    const pages = Array.isArray(item.pages) ? item.pages : []
    if (!pages.length) throw new Error(`subPackages[${index}].pages must be a non-empty array`)
    packages.push({ name: rootName, root: rootName, pages })
  }
  const roots = packages.slice(1).map(item => item.root)
  if (roots.includes('main')) throw new Error('sub-package root main is reserved')
  if (new Set(roots).size !== roots.length) throw new Error('sub-package roots must be unique')
  for (let i = 0; i < roots.length; i++) for (let j = i + 1; j < roots.length; j++) if (isWithin(roots[i], roots[j]) || isWithin(roots[j], roots[i])) throw new Error('sub-package roots must not overlap')
  for (const pkg of packages) {
    const normalizedPages = pkg.pages.map((page, index) => normalizeRelative(page, `${pkg.name || 'main'}.pages[${index}]`))
    if (new Set(normalizedPages).size !== normalizedPages.length) throw new Error(`${pkg.name || 'main'} pages must be unique`)
    pkg.pages = normalizedPages
    for (const page of normalizedPages) pageArtifact(fileSet, pkg.root ? `${pkg.root}/${page}` : page, pkg.name || 'main')
  }
  if (mainPages.some(page => roots.some(rootName => isWithin(page, rootName)))) throw new Error('main pages must not be inside a sub-package root')
  const packageFiles = new Map()
  packageFiles.set('main', files.filter(file => !roots.some(rootName => isWithin(file, rootName))))
  for (const pkg of packages.slice(1)) packageFiles.set(pkg.name, files.filter(file => isWithin(file, pkg.root)))
  const sizes = [...packageFiles.entries()].map(([name, packageFilesForName]) => ({ name, bytes: packageFilesForName.reduce((sum, file) => sum + fs.statSync(path.join(distDir, file)).size, 0) }))
  const totalBytes = files.reduce((sum, file) => sum + fs.statSync(path.join(distDir, file)).size, 0)
  return { files, packages: sizes, totalBytes, packageLimit, totalLimit, manifest }
}
function formatReport(report) {
  const failures = report.packages.filter(pkg => pkg.bytes > report.packageLimit)
  if (report.totalBytes > report.totalLimit) failures.push({ name: 'total', bytes: report.totalBytes })
  if (failures.length) throw new Error(`package size exceeds limit: ${failures.map(pkg => `${pkg.name}=${pkg.bytes} bytes`).join(', ')}`)
  return `miniapp package: PASS (${report.packages.map(pkg => `${pkg.name}=${pkg.bytes}`).join(', ')}; total=${report.totalBytes})`
}
function main() { try { console.log(formatReport(analyzePackage())) } catch (error) { console.error(`miniapp package: FAIL (${error instanceof Error ? error.message : String(error)})`); process.exitCode = 1 } }
module.exports = { analyzePackage, filesIn, formatReport, normalizeRelative, packageLimit, totalLimit }
if (require.main === module) main()
