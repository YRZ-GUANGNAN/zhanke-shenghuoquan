const assert = require('node:assert/strict')
const ts = require('typescript')

// Read only static route declarations. Do not evaluate app.config.ts: build
// configuration can contain imports, environment lookups or executable code.
function property(object, name, required = false) {
  assert.ok(ts.isObjectLiteralExpression(object), 'route configuration must be a static object')
  assert.ok(object.properties.every(node => !ts.isSpreadAssignment(node)), 'route configuration cannot contain spreads')
  assert.ok(object.properties.every(node => !node.name || !ts.isComputedPropertyName(node.name)), 'route configuration cannot contain computed properties')
  const matches = object.properties.filter(node => node.name && (ts.isIdentifier(node.name) || ts.isStringLiteral(node.name)) && node.name.text === name)
  assert.ok(matches.length <= 1, `duplicate route property: ${name}`)
  if (!matches.length) {
    assert.ok(!required, `missing route property: ${name}`)
    return undefined
  }
  assert.ok(ts.isPropertyAssignment(matches[0]), `route property must be static: ${name}`)
  return matches[0].initializer
}

function routeString(node) {
  assert.ok(node && (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)), 'route must be a literal string')
  const value = node.text
  assert.match(value, /^[A-Za-z0-9_-]+(?:\/[A-Za-z0-9_-]+)*$/, `invalid route path: ${value}`)
  return value
}

function routeArray(node) {
  assert.ok(node && ts.isArrayLiteralExpression(node) && node.elements.length > 0, 'pages must be a non-empty literal array')
  return node.elements.map(routeString)
}

function readMiniappRoutes(source) {
  const file = ts.createSourceFile('app.config.ts', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  assert.equal(file.parseDiagnostics.length, 0, 'invalid app.config.ts syntax')
  const exports = file.statements.filter(ts.isExportAssignment)
  assert.equal(exports.length, 1, 'one default app config is required')
  const call = exports[0].expression
  assert.ok(ts.isCallExpression(call) && ts.isIdentifier(call.expression) && call.expression.text === 'defineAppConfig' && call.arguments.length === 1, 'expected static defineAppConfig({...})')
  const config = call.arguments[0]
  const main = routeArray(property(config, 'pages', true))
  const upper = property(config, 'subPackages')
  const lower = property(config, 'subpackages')
  assert.ok(!upper || !lower, 'duplicate sub-package configuration')
  const packages = upper || lower
  const roots = []
  const pages = [...main]
  if (packages) {
    assert.ok(ts.isArrayLiteralExpression(packages), 'sub-packages must be a literal array')
    for (const entry of packages.elements) {
      const root = routeString(property(entry, 'root', true))
      assert.ok(!roots.some(old => root === old || root.startsWith(`${old}/`) || old.startsWith(`${root}/`)), `duplicate or overlapping package root: ${root}`)
      assert.ok(!main.some(page => page.startsWith(`${root}/`)), `main page belongs to a sub-package: ${root}`)
      roots.push(root)
      pages.push(...routeArray(property(entry, 'pages', true)).map(page => `${root}/${page}`))
    }
  }
  assert.equal(new Set(pages).size, pages.length, 'duplicate registered page')
  return pages
}

module.exports = { readMiniappRoutes }
