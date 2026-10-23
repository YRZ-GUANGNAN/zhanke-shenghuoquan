const assert = require('node:assert/strict')

function assertCampusInfoHome(source) {
  assert.match(source, /import\s*\{\s*listCampusGuides[^}]*\}\s*from\s*'\.\.\/\.\.\/services\/campus-info'/)
  assert.doesNotMatch(source, /services\/(b-community|c-trade|d-navigation|e-im)|navigateToMiniProgram|listIMConversations|interleaveCards/)
  assert.match(source, /useState<CampusGuide\[\]>\(\[\]\)/)
  assert.match(source, /listCampusGuides\(\)\.then\(items =>/)
  assert.match(source, /setGuides\(items\)/)
  assert.match(source, /visibleGuides\.map/)
  assert.match(source, /encodeURIComponent\(id\)/)
  assert.match(source, /kind='loading'/)
  assert.match(source, /kind='error'/)
  assert.match(source, /kind='empty'/)
  assert.match(source, /非学校官方应用/)
  assert.doesNotMatch(source, /<Image|imageUrl|https?:\/\//)
}
module.exports = { assertCampusInfoHome }
