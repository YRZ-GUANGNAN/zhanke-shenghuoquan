const { parse } = require('@babel/parser')

const toolFields = new Set(['id', 'title', 'subtitle', 'iconText', 'appId', 'path', 'color'])
const ignoredASTKeys = new Set([
  'type', 'start', 'end', 'loc', 'extra', 'errors',
  'leadingComments', 'innerComments', 'trailingComments'
])

function indexAST(root) {
  const nodes = []
  const parentOf = new WeakMap()
  const visit = (value, parent = null, key = '') => {
    if (Array.isArray(value)) {
      value.forEach((item, index) => visit(item, parent, `${key}[${index}]`))
      return
    }
    if (!value || typeof value !== 'object' || typeof value.type !== 'string') return
    if (parent) parentOf.set(value, { parent, key })
    nodes.push(value)
    for (const [childKey, child] of Object.entries(value)) {
      if (!ignoredASTKeys.has(childKey)) visit(child, value, childKey)
    }
  }
  visit(root)
  return { nodes, parentOf }
}

function unwrapExpression(node) {
  let current = node
  while (current && ['ParenthesizedExpression', 'TSAsExpression', 'TSTypeAssertion', 'TSNonNullExpression'].includes(current.type)) {
    current = current.expression
  }
  return current
}

function isIdentifier(node, name) {
  return node?.type === 'Identifier' && node.name === name
}

function isString(node, value) {
  return node?.type === 'StringLiteral' && node.value === value
}

function isMember(node, objectName, propertyName) {
  const value = unwrapExpression(node)
  return value?.type === 'MemberExpression' && !value.computed &&
    isIdentifier(value.object, objectName) && isIdentifier(value.property, propertyName)
}

function isStrictStringEquality(node, identifierName, stringValue) {
  const value = unwrapExpression(node)
  if (value?.type !== 'BinaryExpression' || value.operator !== '===') return false
  return (isIdentifier(value.left, identifierName) && isString(value.right, stringValue)) ||
    (isString(value.left, stringValue) && isIdentifier(value.right, identifierName))
}

function jsxName(node) {
  return node?.type === 'JSXIdentifier' ? node.name : ''
}

function meaningfulJSXChildren(node) {
  return (node?.children || []).filter((child) => {
    if (child.type === 'JSXText') return child.value.trim() !== ''
    return child.type !== 'JSXExpressionContainer' || child.expression.type !== 'JSXEmptyExpression'
  })
}

function jsxAttributes(node) {
  const result = new Map()
  for (const attribute of node?.openingElement?.attributes || []) {
    if (attribute.type !== 'JSXAttribute' || attribute.name.type !== 'JSXIdentifier' || result.has(attribute.name.name)) return null
    result.set(attribute.name.name, attribute.value)
  }
  return result
}

function stringAttribute(attributes, name, value) {
  const attribute = attributes?.get(name)
  return attribute?.type === 'StringLiteral' && attribute.value === value
}

function expressionAttribute(attributes, name) {
  const attribute = attributes?.get(name)
  return attribute?.type === 'JSXExpressionContainer' ? unwrapExpression(attribute.expression) : null
}

function isVoidCallArrow(node, calleeName, argumentName) {
  const value = unwrapExpression(node)
  if (value?.type !== 'ArrowFunctionExpression' || value.params.length !== 0) return false
  const body = unwrapExpression(value.body)
  const call = body?.type === 'UnaryExpression' && body.operator === 'void' ? unwrapExpression(body.argument) : null
  if (call?.type !== 'CallExpression' || !isIdentifier(call.callee, calleeName)) return false
  if (argumentName === undefined) return call.arguments.length === 0
  return call.arguments.length === 1 && isIdentifier(call.arguments[0], argumentName)
}

function isToolLinkArrayType(node) {
  const annotation = node?.typeAnnotation?.type === 'TSTypeAnnotation' ? node.typeAnnotation.typeAnnotation : node
  return annotation?.type === 'TSArrayType' && annotation.elementType?.type === 'TSTypeReference' &&
    isIdentifier(annotation.elementType.typeName, 'ToolLink')
}

function objectPropertyName(property) {
  if (property?.type !== 'ObjectProperty' || property.computed) return ''
  if (property.key.type === 'Identifier' || property.key.type === 'StringLiteral') return property.key.name || property.key.value
  return ''
}

function isToolLikeObject(node) {
  if (node?.type !== 'ObjectExpression') return false
  const properties = node.properties.filter((property) => toolFields.has(objectPropertyName(property)))
  const fields = new Set(properties.map(objectPropertyName))
  return fields.size >= 2 && properties.every((property) =>
    ['StringLiteral', 'NumericLiteral', 'BooleanLiteral', 'NullLiteral'].includes(unwrapExpression(property.value)?.type))
}

function toolCollectionMapFields(node) {
  if (node?.type !== 'CallExpression' || node.callee.type !== 'MemberExpression' || node.callee.computed ||
      !isIdentifier(node.callee.property, 'map') || node.arguments.length !== 1) return new Set()
  const callback = unwrapExpression(node.arguments[0])
  if (callback?.type !== 'ArrowFunctionExpression' || callback.params.length !== 1 || callback.params[0].type !== 'Identifier') {
    return new Set()
  }
  const itemName = callback.params[0].name
  return new Set(indexAST(callback.body).nodes
    .filter((candidate) => candidate.type === 'MemberExpression' && !candidate.computed &&
      isIdentifier(candidate.object, itemName) && candidate.property.type === 'Identifier' && toolFields.has(candidate.property.name))
    .map((candidate) => candidate.property.name))
}

function validateToolText(node, className, fieldName) {
  if (node?.type !== 'JSXElement' || jsxName(node.openingElement.name) !== 'Text') return false
  const attributes = jsxAttributes(node)
  if (!attributes || attributes.size !== 1 || !stringAttribute(attributes, 'className', className)) return false
  const children = meaningfulJSXChildren(node)
  return children.length === 1 && children[0].type === 'JSXExpressionContainer' &&
    isMember(children[0].expression, 'tool', fieldName)
}

function validateToolIcon(node) {
  if (node?.type !== 'JSXElement' || jsxName(node.openingElement.name) !== 'View') return false
  const attributes = jsxAttributes(node)
  if (!attributes || attributes.size !== 2 || !stringAttribute(attributes, 'className', 'student-tool-card__icon')) return false
  const style = expressionAttribute(attributes, 'style')
  if (style?.type !== 'ObjectExpression' || style.properties.length !== 1) return false
  const color = style.properties[0]
  if (objectPropertyName(color) !== 'color' || !isMember(color.value, 'tool', 'color')) return false
  const children = meaningfulJSXChildren(node)
  if (children.length !== 1 || children[0].type !== 'JSXElement' || jsxName(children[0].openingElement.name) !== 'Text') return false
  const textAttributes = jsxAttributes(children[0])
  const textChildren = meaningfulJSXChildren(children[0])
  return textAttributes?.size === 0 && textChildren.length === 1 &&
    textChildren[0].type === 'JSXExpressionContainer' && isMember(textChildren[0].expression, 'tool', 'iconText')
}

function validateToolCard(node) {
  if (node?.type !== 'JSXElement' || jsxName(node.openingElement.name) !== 'View') return false
  const attributes = jsxAttributes(node)
  if (!attributes || attributes.size !== 3 || !stringAttribute(attributes, 'className', 'student-tool-card')) return false
  if (!isMember(expressionAttribute(attributes, 'key'), 'tool', 'id')) return false
  if (!isVoidCallArrow(expressionAttribute(attributes, 'onClick'), 'openTool', 'tool')) return false
  const children = meaningfulJSXChildren(node)
  return children.length === 3 && validateToolIcon(children[0]) &&
    validateToolText(children[1], 'student-tool-card__label', 'title') &&
    validateToolText(children[2], 'student-tool-card__sub', 'subtitle')
}

function validateReadyBranch(node, violations) {
  const ready = unwrapExpression(node)
  if (ready?.type !== 'JSXElement' || jsxName(ready.openingElement.name) !== 'View') {
    violations.push('tools ready branch must be one student-tools-grid View')
    return null
  }
  const attributes = jsxAttributes(ready)
  if (!attributes || attributes.size !== 1 || !stringAttribute(attributes, 'className', 'student-tools-grid')) {
    violations.push('tools ready branch root must be the student-tools-grid only')
  }
  const children = meaningfulJSXChildren(ready)
  if (children.length !== 1 || children[0].type !== 'JSXExpressionContainer') {
    violations.push('tools ready branch may render only one tools.map collection')
    return null
  }
  const mapCall = unwrapExpression(children[0].expression)
  if (mapCall?.type !== 'CallExpression' || !isMember(mapCall.callee, 'tools', 'map') || mapCall.arguments.length !== 1) {
    violations.push('tools ready branch collection must be exactly tools.map')
    return null
  }
  const callback = unwrapExpression(mapCall.arguments[0])
  if (callback?.type !== 'ArrowFunctionExpression' || callback.params.length !== 1 ||
      !isIdentifier(callback.params[0], 'tool') || !validateToolCard(unwrapExpression(callback.body))) {
    violations.push('tools.map must render the canonical card using only fields from tool')
  }
  const readyIndex = indexAST(ready)
  const collectionCalls = readyIndex.nodes.filter((candidate) => candidate.type === 'CallExpression' &&
    candidate.callee.type === 'MemberExpression' && !candidate.callee.computed &&
    ['map', 'concat'].includes(candidate.callee.property.name))
  if (collectionCalls.length !== 1 || collectionCalls[0] !== mapCall) {
    violations.push('tools ready branch must not contain another map or concat collection')
  }
  const arrays = readyIndex.nodes.filter((candidate) => candidate.type === 'ArrayExpression')
  if (arrays.length > 0) violations.push('tools ready branch must not contain static arrays')
  const objects = readyIndex.nodes.filter((candidate) => candidate.type === 'ObjectExpression')
  if (objects.length !== 1 || objects[0].properties.length !== 1 || objectPropertyName(objects[0].properties[0]) !== 'color') {
    violations.push('tools ready branch must not contain static card objects')
  }
  return mapCall
}

function validateStatusBranch(node, status, violations) {
  const expected = {
    error: ['student-tool-state student-tool-state--retry', '工具加载失败，点击重试'],
    empty: ['student-tool-state', '暂无可用工具'],
    loading: ['student-tool-state', '正在加载真实工具...']
  }[status]
  const branch = unwrapExpression(node)
  if (branch?.type !== 'JSXElement' || jsxName(branch.openingElement.name) !== 'View') {
    violations.push(`tools ${status} branch must render only status UI`)
    return
  }
  const attributes = jsxAttributes(branch)
  const expectedAttributeCount = status === 'error' ? 2 : 1
  if (!attributes || attributes.size !== expectedAttributeCount || !stringAttribute(attributes, 'className', expected[0])) {
    violations.push(`tools ${status} branch must use only the canonical status View`)
    return
  }
  if (status === 'error' && !isVoidCallArrow(expressionAttribute(attributes, 'onClick'), 'loadTools')) {
    violations.push('tools error branch retry must call loadTools without rendering data')
  }
  const children = meaningfulJSXChildren(branch)
  if (children.length !== 1 || children[0].type !== 'JSXElement' || jsxName(children[0].openingElement.name) !== 'Text' ||
      jsxAttributes(children[0])?.size !== 0) {
    violations.push(`tools ${status} branch must contain only one Text status message`)
    return
  }
  const textChildren = meaningfulJSXChildren(children[0])
  if (textChildren.length !== 1 || textChildren[0].type !== 'JSXText' || textChildren[0].value.trim() !== expected[1]) {
    violations.push(`tools ${status} branch status message is not canonical`)
  }
}

function homeToolsDataFlowViolations(home) {
  const violations = []
  let ast
  try {
    ast = parse(home, { sourceType: 'module', plugins: ['typescript', 'jsx'] })
  } catch (error) {
    return [`home tools source must parse as TypeScript JSX: ${error.message}`]
  }

  const navigationImports = ast.program.body.filter((node) => node.type === 'ImportDeclaration' &&
    node.source.value === '../../services/d-navigation')
  const listImports = navigationImports.flatMap((node) => node.specifiers).filter((specifier) =>
    specifier.type === 'ImportSpecifier' && isIdentifier(specifier.imported, 'listNavigationItems') &&
    isIdentifier(specifier.local, 'listNavigationItems'))
  if (listImports.length !== 1) violations.push('tools must import listNavigationItems once from the navigation service without aliasing')

  const homeFunctions = ast.program.body.filter((node) => node.type === 'FunctionDeclaration' && isIdentifier(node.id, 'HomeContent'))
  if (homeFunctions.length !== 1) return [...violations, 'HomeContent function must exist exactly once']
  const homeFunction = homeFunctions[0]
  const homeIndex = indexAST(homeFunction)
  const moduleIndex = indexAST(ast.program)

  const toolStates = homeIndex.nodes.filter((node) => node.type === 'VariableDeclarator' && node.id.type === 'ArrayPattern' &&
    node.id.elements.length === 2 && isIdentifier(node.id.elements[0], 'tools') && isIdentifier(node.id.elements[1], 'setTools') &&
    node.init?.type === 'CallExpression' && isIdentifier(node.init.callee, 'useState') && node.init.arguments.length === 1 &&
    node.init.arguments[0].type === 'ArrayExpression' && node.init.arguments[0].elements.length === 0 &&
    isToolLinkArrayType((node.init.typeParameters || node.init.typeArguments)?.params?.[0]))
  if (toolStates.length !== 1) violations.push('tools state must be exactly one empty useState<ToolLink[]> binding')

  const listCalls = homeIndex.nodes.filter((node) => node.type === 'CallExpression' && isIdentifier(node.callee, 'listNavigationItems'))
  if (listCalls.length !== 1) violations.push('tools must have exactly one navigation service load call')
  let valuesBinding = null
  if (listCalls.length === 1) {
    const awaitEntry = homeIndex.parentOf.get(listCalls[0])
    const declaratorEntry = awaitEntry && awaitEntry.parent.type === 'AwaitExpression' ? homeIndex.parentOf.get(awaitEntry.parent) : null
    if (declaratorEntry?.parent.type === 'VariableDeclarator' && isIdentifier(declaratorEntry.parent.id, 'values')) {
      valuesBinding = declaratorEntry.parent.id
    } else {
      violations.push('listNavigationItems result must bind directly to const values')
    }
  }

  const setToolsCalls = homeIndex.nodes.filter((node) => node.type === 'CallExpression' && isIdentifier(node.callee, 'setTools'))
  const valuesSetCalls = setToolsCalls.filter((node) => node.arguments.length === 1 && isIdentifier(node.arguments[0], 'values'))
  const emptySetCalls = setToolsCalls.filter((node) => node.arguments.length === 1 &&
    node.arguments[0].type === 'ArrayExpression' && node.arguments[0].elements.length === 0)
  if (setToolsCalls.length !== 2 || valuesSetCalls.length !== 1 || emptySetCalls.length !== 1) {
    violations.push('setTools must receive service values once and an empty reset once')
  }

  for (const node of moduleIndex.nodes) {
    if (isToolLikeObject(node)) {
      violations.push('home module must not declare a static tool-like object')
      break
    }
    if (node.type === 'VariableDeclarator' && node.init?.type === 'ArrayExpression' && node.init.elements.length > 0 &&
        isToolLinkArrayType(node.id)) {
      violations.push('home module must not declare a static ToolLink array')
      break
    }
  }

  const renderCandidates = homeFunction.body.body
    .filter((node) => node.type === 'ReturnStatement' && node.argument)
    .map((node) => indexAST(unwrapExpression(node.argument)))
    .filter((index) => index.nodes.some((node) => node.type === 'JSXExpressionContainer' &&
      unwrapExpression(node.expression)?.type === 'LogicalExpression' && unwrapExpression(node.expression).operator === '&&' &&
      isStrictStringEquality(unwrapExpression(node.expression).left, 'activeTab', 'tools')))
  if (renderCandidates.length !== 1) violations.push('HomeContent must have exactly one main JSX return containing the tools branch')
  const renderIndex = renderCandidates[0] || { nodes: [] }
  const toolsContainers = renderIndex.nodes.filter((node) => node.type === 'JSXExpressionContainer' &&
    unwrapExpression(node.expression)?.type === 'LogicalExpression' && unwrapExpression(node.expression).operator === '&&' &&
    isStrictStringEquality(unwrapExpression(node.expression).left, 'activeTab', 'tools'))
  let readyMapCall = null
  if (toolsContainers.length !== 1) {
    violations.push("HomeContent must have exactly one activeTab === 'tools' render branch")
  } else {
    const toolsExpression = unwrapExpression(toolsContainers[0].expression)
    const readyConditional = unwrapExpression(toolsExpression.right)
    if (readyConditional?.type !== 'ConditionalExpression' || !isStrictStringEquality(readyConditional.test, 'toolStatus', 'ready')) {
      violations.push("tools branch must start with toolStatus === 'ready'")
    } else {
      readyMapCall = validateReadyBranch(readyConditional.consequent, violations)
      const errorConditional = unwrapExpression(readyConditional.alternate)
      const emptyConditional = errorConditional?.type === 'ConditionalExpression' ? unwrapExpression(errorConditional.alternate) : null
      if (errorConditional?.type !== 'ConditionalExpression' || !isStrictStringEquality(errorConditional.test, 'toolStatus', 'error')) {
        violations.push("tools status chain must include toolStatus === 'error'")
      } else {
        validateStatusBranch(errorConditional.consequent, 'error', violations)
      }
      if (emptyConditional?.type !== 'ConditionalExpression' || !isStrictStringEquality(emptyConditional.test, 'toolStatus', 'empty')) {
        violations.push("tools status chain must include toolStatus === 'empty'")
      } else {
        validateStatusBranch(emptyConditional.consequent, 'empty', violations)
        validateStatusBranch(emptyConditional.alternate, 'loading', violations)
      }
    }
  }

  const extraToolCollectionMaps = renderIndex.nodes.filter((node) =>
    node !== readyMapCall && toolCollectionMapFields(node).size >= 2)
  if (extraToolCollectionMaps.length > 0) {
    violations.push('HomeContent main JSX return must not render another tool-like collection outside canonical tools.map')
  }

  if (toolStates.length === 1 && readyMapCall) {
    const allowedTools = new Set([toolStates[0].id.elements[0], readyMapCall.callee.object])
    if (homeIndex.nodes.some((node) => isIdentifier(node, 'tools') && !allowedTools.has(node))) {
      violations.push('tools may only be referenced by the canonical tools.map render')
    }
  }
  if (toolStates.length === 1) {
    const allowedSetTools = new Set([toolStates[0].id.elements[1], ...setToolsCalls.map((node) => node.callee)])
    if (homeIndex.nodes.some((node) => isIdentifier(node, 'setTools') && !allowedSetTools.has(node))) {
      violations.push('setTools must not be aliased or referenced outside its two canonical calls')
    }
  }
  if (valuesBinding && valuesSetCalls.length === 1) {
    const allowedValues = new Set([valuesBinding, valuesSetCalls[0].arguments[0]])
    for (const node of homeIndex.nodes) {
      if (node.type === 'MemberExpression' && !node.computed && isIdentifier(node.object, 'values') && isIdentifier(node.property, 'length')) {
        allowedValues.add(node.object)
      }
    }
    if (homeIndex.nodes.some((node) => isIdentifier(node, 'values') && !allowedValues.has(node))) {
      violations.push('navigation service values may only flow into setTools or the ready/empty status decision')
    }
  }

  return [...new Set(violations)]
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
  const toolsState = '  const [tools, setTools] = useState<ToolLink[]>([])'
  const toolsMap = '{tools.map((tool) => ('
  const messagesRender = "        {activeTab === 'messages' && ("
  if (!home.includes(toolsState) || !home.includes(toolsMap) || !home.includes(messagesRender)) {
    throw new Error('Home tools state or render marker is missing')
  }
  const fakeTool = "{ id: 'fake', title: 'fake', subtitle: 'fake', iconText: 'F', appId: 'wx1234567890ABCDEF', path: 'pages/home/index', color: '#000' }"
  const mutations = [
    ['arbitraryFakeTools', home.replace(marker, `${marker}\n  const arbitraryFakeTools: ToolLink[] = [${fakeTool}]`)],
    ['arbitraryFakeCards', home.replace(marker, `${marker}\n  const arbitraryFakeCards: ToolLink[] = [${fakeTool}]`)],
    ['static fake object', home.replace(marker, `${marker}\n  const arbitraryFakeCard: ToolLink = ${fakeTool}`)],
    ['tools.concat([fake])', home.replace(toolsState, `${toolsState}\n  const decoratedCards = tools.concat([${fakeTool}])`)],
    ['decoratedCards.map', home
      .replace(toolsState, `${toolsState}\n  const decoratedCards = tools`)
      .replace(toolsMap, '{decoratedCards.map((tool) => (')],
    ['extra fake card', home.replace(toolsMap, `<View className='alternate-fake-card'><Text>fake</Text></View>\n              ${toolsMap}`)],
    ['module arbitraryFakeCards', home.replace(marker, `const arbitraryFakeCards: ToolLink[] = [${fakeTool}]\n\n${marker}`)],
    ['module static fake object', home.replace(marker, `const arbitraryFakeCard: ToolLink = ${fakeTool}\n\n${marker}`)],
    ['module decoratedCollection.map outside tools', home
      .replace(marker, `const arbitraryFakeCard: ToolLink = ${fakeTool}\nconst decoratedCollection: ToolLink[] = [arbitraryFakeCard]\n\n${marker}`)
      .replace(messagesRender, `{decoratedCollection.map((entry) => (<View key={entry.id} className='alternate-fake-card'><Text>{entry.title}</Text></View>))}\n${messagesRender}`)]
  ]
  for (const [name, mutated] of mutations) {
    const violations = homeToolsDataFlowViolations(mutated)
    if (violations.length === 0) {
      throw new Error(`synthetic ${name} bypass was not rejected`)
    }
  }
}

module.exports = {
  assertHomeToolsDataFlow,
  assertSyntheticStaticToolsRejected,
  homeToolsDataFlowViolations
}
