const fs = require('node:fs')
const http = require('node:http')
const path = require('node:path')

const rootDir = path.resolve(__dirname, '..')

function getJson(url) {
  return new Promise((resolve, reject) => {
    const request = http.get(url, { timeout: 5000 }, (response) => {
      let body = ''
      response.setEncoding('utf8')
      response.on('data', (chunk) => {
        body += chunk
      })
      response.on('end', () => {
        if (response.statusCode < 200 || response.statusCode >= 300) {
          reject(new Error(`${url} returned ${response.statusCode}`))
          return
        }
        try {
          resolve(JSON.parse(body))
        } catch {
          resolve(body)
        }
      })
    })
    request.on('timeout', () => {
      request.destroy(new Error(`${url} timed out`))
    })
    request.on('error', reject)
  })
}

async function checkEndpoint(label, url, getCount) {
  const body = await getJson(url)
  const count = getCount ? getCount(body) : undefined
  console.log(`ok ${label}${typeof count === 'number' ? ` (${count})` : ''}`)
}

async function checkAdmin() {
  await new Promise((resolve, reject) => {
    const request = http.get('http://127.0.0.1:5173', { timeout: 5000 }, (response) => {
      response.resume()
      response.on('end', () => {
        if (response.statusCode && response.statusCode >= 200 && response.statusCode < 400) {
          console.log('ok admin ui')
          resolve()
        } else {
          reject(new Error(`admin ui returned ${response.statusCode}`))
        }
      })
    })
    request.on('timeout', () => {
      request.destroy(new Error('admin ui timed out'))
    })
    request.on('error', reject)
  })
}

async function main() {
  const miniappDist = path.join(rootDir, 'apps', 'miniapp', 'dist', 'app.json')
  const checks = [
    checkEndpoint('backend health', 'http://127.0.0.1:8080/healthz'),
	checkEndpoint('B community posts', 'http://127.0.0.1:8080/api/v1/community/posts', (body) => body.data?.items?.length ?? 0),
	checkEndpoint('C trade products', 'http://127.0.0.1:8080/api/v1/trade/products', (body) => body.data?.items?.length ?? 0),
	checkEndpoint('D navigation', 'http://127.0.0.1:8080/api/v1/navigation/categories', (body) => body.data?.length ?? 0),
    checkAdmin()
  ]

  await Promise.all(checks)

  if (fs.existsSync(miniappDist)) {
    console.log('ok miniapp dist')
  } else {
    throw new Error('miniapp dist is missing; run npm run build:miniapp')
  }
}

main().catch((error) => {
  console.error(`check failed: ${error.message}`)
  process.exitCode = 1
})
