function homeToolsDataFlowViolations(home) {
  const violations = []
  if (!/import\s*\{[^}]*\blistNavigationItems\b[^}]*\}\s*from\s*['"]\.\.\/\.\.\/services\/d-navigation['"]/.test(home)) {
    violations.push('tools must import listNavigationItems from the navigation service')
  }
  if (!/const\s+\[tools,\s*setTools\]\s*=\s*useState<ToolLink\[\]>\(\[\]\)/.test(home)) {
    violations.push('tools state must start as an empty ToolLink array')
  }
  if ((home.match(/\blistNavigationItems\(\)/g) || []).length !== 1) {
    violations.push('tools must have exactly one navigation service load call')
  }
  if (!/const\s+values\s*=\s*await\s+listNavigationItems\(\)/.test(home) || !/setTools\(values\)/.test(home)) {
    violations.push('listNavigationItems results must flow directly into setTools')
  }

  const setToolsArguments = [...home.matchAll(/\bsetTools\(\s*([^)]*?)\s*\)/g)].map((match) => match[1].trim())
  if (setToolsArguments.length !== 2 || setToolsArguments.some((value) => value !== 'values' && value !== '[]')) {
    violations.push('setTools may only receive service values or an empty reset')
  }
  if (!/\btools\.map\(\(tool\)\s*=>\s*\(/.test(home)) {
    violations.push('tool cards must render from tools.map')
  }
  for (const token of ['openTool(tool)', 'tool.color', 'tool.iconText', 'tool.title', 'tool.subtitle']) {
    if (!home.includes(token)) violations.push(`service-backed tool render is missing ${token}`)
  }
  if ((home.match(/className=['"]student-tool-card['"]/g) || []).length !== 1) {
    violations.push('tool cards must have exactly one service-backed render site')
  }
  if (/\btools\.(?:push|unshift|splice)\s*\(/.test(home) || /\btools\s*\[[^\]]+\]\s*=/.test(home)) {
    violations.push('tools state must not be mutated with static entries')
  }
  if (/\bvalues\.(?:push|unshift|splice)\s*\(/.test(home) || /\bvalues\s*\[[^\]]+\]\s*=/.test(home)) {
    violations.push('navigation service results must not be mutated with static entries')
  }

  for (const match of home.matchAll(/\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)(?:\s*:[^=\n]+)?\s*=\s*\[/g)) {
    if (/tool/i.test(match[1])) {
      violations.push(`static tool array ${match[1]} is forbidden`)
    }
  }
  return violations
}

function assertHomeToolsDataFlow(home) {
  const violations = homeToolsDataFlowViolations(home)
  if (violations.length > 0) {
    throw new Error(`home tools data-flow contract failed:\n${violations.join('\n')}`)
  }
}

function assertSyntheticStaticToolsRejected(home) {
  const marker = 'function HomeContent() {'
  if (!home.includes(marker)) throw new Error('HomeContent marker is missing')
  const mutated = home.replace(marker, `${marker}\n  const arbitraryFakeTools: ToolLink[] = [{ id: 'fake', title: 'arbitrary fake tool' }]`)
  const violations = homeToolsDataFlowViolations(mutated)
  if (!violations.some((value) => value.includes('static tool array arbitraryFakeTools'))) {
    throw new Error('synthetic arbitraryFakeTools bypass was not rejected')
  }
}

module.exports = {
  assertHomeToolsDataFlow,
  assertSyntheticStaticToolsRejected,
  homeToolsDataFlowViolations
}
