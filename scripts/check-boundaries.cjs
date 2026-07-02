const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const moduleRoot = path.join(root, 'backend', 'internal', 'module')
const allowedDomains = new Set(['user', 'community', 'trade', 'navigation', 'im', 'admin', 'media', 'takeout', 'delivery'])
const failures = []

for (const entry of fs.readdirSync(moduleRoot, { withFileTypes: true })) {
  if (entry.isDirectory() && !allowedDomains.has(entry.name)) failures.push(`unclassified backend module: ${entry.name}`)
}

function files(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const target = path.join(dir, entry.name)
    return entry.isDirectory() ? files(target) : [target]
  })
}

for (const file of files(moduleRoot).filter((file) => file.endsWith('.go'))) {
  const source = fs.readFileSync(file, 'utf8')
  const name = path.basename(file)
  const repositoryFile = name === 'repository.go' || name.endsWith('_repository.go')
  const testFile = name.endsWith('_test.go')
  if ((source.includes('*gorm.DB') || source.includes('.WithContext(')) && !repositoryFile && !testFile) {
    failures.push(`database access outside repository: ${path.relative(root, file)}`)
  }
}

const navigationSource = files(path.join(moduleRoot, 'navigation')).map((file) => fs.readFileSync(file, 'utf8')).join('\n')
if (/internal\/module\/(user|community|trade|im|admin)/.test(navigationSource)) failures.push('domain D imports another business domain')

const imSource = files(path.join(moduleRoot, 'im')).map((file) => fs.readFileSync(file, 'utf8')).join('\n')
if (/type\s+Message\b|message_content|messages\b/.test(imSource)) failures.push('domain E contains message storage')

const takeoutSource = files(path.join(moduleRoot, 'takeout'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(user|community|trade|navigation|im|admin|media)|github\.com\/gin-gonic\/gin|"net\/http"/.test(takeoutSource)) {
  failures.push('domain G core imports HTTP or another business domain')
}

const deliverySource = files(path.join(moduleRoot, 'delivery'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(user|community|trade|navigation|im|admin|media|takeout|delivery)|github\.com\/gin-gonic\/gin|"net\/http"/.test(deliverySource)) {
  failures.push('delivery core imports HTTP or another business domain')
}

const deliveryMerchantSource = files(path.join(moduleRoot, 'admin', 'deliverymerchant'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(takeout|community|trade|navigation|im|media)/.test(deliveryMerchantSource)) {
  failures.push('delivery merchant transport imports a forbidden business domain')
}

const deliveryNotifySource = files(path.join(moduleRoot, 'admin', 'deliverynotify'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(takeout|user|community|trade|navigation|im|media)/.test(deliveryNotifySource)) {
  failures.push('delivery notification adapter imports a forbidden business domain')
}

const deliveryPaymentSource = files(path.join(moduleRoot, 'admin', 'deliverypayment'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(takeout|user|community|trade|navigation|im|media)/.test(deliveryPaymentSource)) {
  failures.push('delivery payment adapter imports a forbidden business domain')
}

const deliveryConsoleSource = files(path.join(moduleRoot, 'admin', 'deliveryconsole'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(takeout|user|community|trade|navigation|im|media)/.test(deliveryConsoleSource)) {
  failures.push('delivery console adapter imports a forbidden business domain')
}

const deliveryProofSource = files(path.join(moduleRoot, 'admin', 'deliveryproof'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(takeout|user|community|trade|navigation|im)/.test(deliveryProofSource)) {
  failures.push('delivery proof adapter imports a forbidden business domain')
}

const deliveryIMSource = files(path.join(moduleRoot, 'admin', 'deliveryim'))
  .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
  .map((file) => fs.readFileSync(file, 'utf8'))
  .join('\n')
if (/internal\/module\/(takeout|user|community|trade|navigation)/.test(deliveryIMSource)) {
  failures.push('delivery IM adapter imports a forbidden business domain')
}

const forbiddenFrontend = ['content.ts', 'mock.ts']
for (const name of forbiddenFrontend) if (fs.existsSync(path.join(root, 'apps', 'miniapp', 'src', 'services', name))) failures.push(`mixed frontend service exists: ${name}`)

if (failures.length) {
  console.error(failures.map((item) => `boundary error: ${item}`).join('\n'))
  process.exitCode = 1
} else {
  console.log('ok A-G boundaries plus independent delivery and isolated P0-4 media domains')
}
