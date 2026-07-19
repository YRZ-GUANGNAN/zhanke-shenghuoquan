const assert = require('node:assert/strict')
const fs = require('node:fs')

const read = (path) => fs.readFileSync(path, 'utf8')
const types = read('apps/admin/src/types.ts')
const service = read('apps/admin/src/services/f-admin.ts')
const app = read('apps/admin/src/App.tsx')
const componentPath = 'apps/admin/src/IdentityApplications.tsx'
const component = fs.existsSync(componentPath) ? read(componentPath) : ''

assert.match(types, /AdminSection[^\n]*'identities'/)
assert.match(types, /interface IdentityApplicationRow[\s\S]*userId:string/)
assert.match(types, /interface IdentityApplicationEvent[\s\S]*actorUserId:string/)
assert.match(types, /interface IdentityApplicationRow[\s\S]*reviewerUserId\?:string/)
assert.match(types, /interface IdentityApplicationAdminList[\s\S]*items:IdentityApplicationRow\[\][\s\S]*page:number[\s\S]*pageSize:number[\s\S]*total:number/)

assert.match(service, /listIdentityApplications:/)
assert.match(service, /identityApplication:/)
assert.match(service, /approveIdentityApplication:/)
assert.match(service, /rejectIdentityApplication:/)
assert.match(service, /\/admin\/identity-applications/)
assert.match(service, /expectedRowVersion/)
assert.match(service, /reason/)

assert.match(app, /key:'identities'|key: 'identities'/)
assert.match(app, /<IdentityApplications\s*\/>/)
assert.ok(component.length > 0, 'IdentityApplications.tsx is required')
assert.match(component, /proofs/)
assert.match(component, /events/)
assert.match(component, /该申请已被其他管理员处理/)
assert.match(component, /window\.confirm/)
assert.doesNotMatch(component, /mediaAsset\(|\/media\/|resolveMedia/)

console.log('admin identity applications contract: ok')
