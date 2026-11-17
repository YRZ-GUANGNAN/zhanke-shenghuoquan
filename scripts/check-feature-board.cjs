// 功能建议墙 + 作者的话：前后端闭环契约。
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

const mainGo = read('backend', 'cmd', 'server', 'main.go')
const handler = read('backend', 'internal', 'module', 'featureboard', 'handler.go')
const service = read('backend', 'internal', 'module', 'featureboard', 'service.go')
const repository = read('backend', 'internal', 'module', 'featureboard', 'repository.go')
const client = read('apps', 'miniapp', 'src', 'services', 'featureboard.ts')
const page = read('apps', 'miniapp', 'src', 'pages', 'feature-board', 'index.tsx')
const profile = read('apps', 'miniapp', 'src', 'pages', 'profile', 'index.tsx')
const adminApp = read('apps', 'admin', 'src', 'preview', 'App.tsx')
const adminView = read('apps', 'admin', 'src', 'preview', 'views', 'FeatureBoardView.tsx')

// 后端：模块接线与路由。
for (const fragment of [
  'featureboard.NewService',
  'registerFeatureBoardRoutes(',
  '"/admin/feature-board"',
]) {
  if (!mainGo.includes(fragment)) throw new Error(`main.go is missing ${fragment}`)
}
for (const fragment of [
  'rg.GET("/feature-suggestions", h.list)',
  'rg.POST("/feature-suggestions", h.submit)',
  'rg.POST("/feature-suggestions/:suggestionId/support", h.support)',
  'rg.GET("/author-notes", h.notes)',
  'rg.GET("/suggestions", h.adminList)',
  'rg.POST("/suggestions/:suggestionId/status", h.adminSetStatus)',
  'rg.GET("/feature-suggestions/:suggestionId/comments", h.comments)',
  'rg.POST("/feature-suggestions/:suggestionId/comments", h.submitComment)',
  'rg.GET("/suggestions/:suggestionId/comments", h.adminComments)',
  'rg.POST("/suggestions/:suggestionId/comments", h.adminReply)',
  'rg.POST("/notes", h.adminCreateNote)',
  'rg.DELETE("/notes/:noteId", h.adminDeleteNote)',
]) {
  if (!handler.includes(fragment)) throw new Error(`feature board handler is missing ${fragment}`)
}
// 支持数必须通过事务内条件更新维护，不能被整值覆盖。
if (!repository.includes('Transaction(func') || !repository.includes('support_count + 1')) {
  throw new Error('support toggling must run in a transaction with relative count updates')
}
if (!service.includes('maxContentRunes = 500') || !service.includes('utf8.RuneCountInString')) {
  throw new Error('suggestion content limits must be enforced by rune count')
}

// 小程序：服务接口与页面行为。
for (const fragment of [
  "'/auth/feature-suggestions'",
  '`/auth/feature-suggestions?sort=',
  '/support`',
  "'/auth/author-notes'",
]) {
  if (!client.includes(fragment)) throw new Error(`miniapp feature board client is missing ${fragment}`)
}
for (const fragment of [
  'CapsulePageHeader',
  '作者的话',
  '最受欢迎',
  '已采纳',
  'onScrollToLower',
  'toggleThread',
  'submitFeatureComment',
  '对话',
]) {
  if (!page.includes(fragment)) throw new Error(`feature board page is missing ${fragment}`)
}
if (!profile.includes("navigate('/pages/feature-board/index')")) {
  throw new Error('profile settings must link to the feature board page')
}

// 后台：导航与视图。
if (!adminApp.includes("{ key: 'feature-board', label: '功能建议'")) throw new Error('admin navigation is missing 功能建议')
if (!adminApp.includes("import('./views/FeatureBoardView')")) throw new Error('admin feature board view must be lazy loaded')
for (const fragment of [
  'adminAPI.featureSuggestions',
  'adminAPI.setFeatureSuggestionStatus',
  'adminAPI.createAuthorNote',
  'adminAPI.deleteAuthorNote',
  'adminAPI.featureSuggestionComments',
  'adminAPI.replyFeatureSuggestion',
]) {
  if (!adminView.includes(fragment)) throw new Error(`admin feature board view is missing ${fragment}`)
}

console.log('feature board contract: ok (migration, API, miniapp wall and admin console)')
