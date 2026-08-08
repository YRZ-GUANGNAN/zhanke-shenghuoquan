const { spawn } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')

const rootDir = path.resolve(__dirname, '..')
const backendDir = path.join(rootDir, 'backend')
const envFiles = [path.join(backendDir, '.env'), path.join(rootDir, '.env')]
const LOCAL_HOSTS = new Set(['127.0.0.1', 'localhost', '::1'])
const DEMO_OPEN_ID = 'local_secondhand_demo_20260808'
const DEMO_PREFIX = '[本机预览]'

const DEMO_PRODUCTS = [
  {
    title: `${DEMO_PREFIX} 87键机械键盘`,
    description: '自用青轴机械键盘，按键和灯光正常，附数据线。适合宿舍学习与桌面办公，可在校内当面验货。',
    category: '数码', price: 12900, originalPrice: 25900, condition: '九成新'
  },
  {
    title: `${DEMO_PREFIX} iPad 手写笔`,
    description: '电容手写笔，支持磁吸收纳，书写与续航正常。仅用于本机二手页面多卡片与分类预览。',
    category: '数码', price: 6800, originalPrice: 16900, condition: '八成新'
  },
  {
    title: `${DEMO_PREFIX} 高等数学教材与笔记`,
    description: '高等数学上下册及课堂笔记，重点章节有少量标注，页面完整无缺失。',
    category: '书籍资料', price: 1800, originalPrice: 5600, condition: '有使用痕迹'
  },
  {
    title: `${DEMO_PREFIX} 英语四级真题套装`,
    description: '近年四级真题、听力文本和答案解析，部分题目用铅笔做过，可擦除。',
    category: '书籍资料', price: 1200, originalPrice: 3990, condition: '八成新'
  },
  {
    title: `${DEMO_PREFIX} 宿舍折叠桌`,
    description: '可折叠床上桌，桌面平整、支架稳固，适合宿舍看书或放电脑。',
    category: '生活用品', price: 3500, originalPrice: 7990, condition: '九成新'
  },
  {
    title: `${DEMO_PREFIX} 小功率电煮锅`,
    description: '宿舍小功率电煮锅，双档调节，锅体与电源线完好。交易前请自行确认宿舍用电规定。',
    category: '生活用品', price: 2600, originalPrice: 5990, condition: '八成新'
  },
  {
    title: `${DEMO_PREFIX} 校园通勤自行车`,
    description: '单速通勤自行车，刹车、车铃和车胎状态正常，适合校内短途代步。',
    category: '代步', price: 28000, originalPrice: 69900, condition: '七成新'
  },
  {
    title: `${DEMO_PREFIX} 桌面台灯`,
    description: '三档亮度触控台灯，可折叠调节角度，适合书桌阅读。',
    category: '其他', price: 2200, originalPrice: 4990, condition: '九成新'
  }
]

function unquote(value) {
  const trimmed = value.trim()
  if (trimmed.length >= 2 && ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'")))) {
    return trimmed.slice(1, -1)
  }
  return trimmed
}

function loadEnvValue(key) {
  if (process.env[key]) return process.env[key]
  for (const file of envFiles) {
    if (!fs.existsSync(file)) continue
    for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('#')) continue
      const index = trimmed.indexOf('=')
      if (index < 1 || trimmed.slice(0, index).trim() !== key) continue
      return unquote(trimmed.slice(index + 1))
    }
  }
  return ''
}

function parseMysqlDsn(dsn) {
  const marker = '@tcp('
  const markerIndex = dsn.indexOf(marker)
  const addressEnd = dsn.indexOf(')/', markerIndex + marker.length)
  if (markerIndex < 1 || addressEnd < 0) throw new Error('MYSQL_DSN 格式不受支持')

  const credentials = dsn.slice(0, markerIndex)
  const separator = credentials.indexOf(':')
  const address = dsn.slice(markerIndex + marker.length, addressEnd)
  const portSeparator = address.lastIndexOf(':')
  const databaseEnd = dsn.indexOf('?', addressEnd + 2)
  if (separator < 1 || portSeparator < 1) throw new Error('MYSQL_DSN 缺少用户名、主机或端口')

  return {
    user: credentials.slice(0, separator),
    password: credentials.slice(separator + 1),
    host: address.slice(0, portSeparator).replace(/^\[|\]$/g, ''),
    port: address.slice(portSeparator + 1),
    database: dsn.slice(addressEnd + 2, databaseEnd === -1 ? undefined : databaseEnd)
  }
}

function mysqlBinary() {
  const candidates = [
    process.env.MYSQL_BIN,
    String.raw`C:\Program Files\MySQL\MySQL Server 8.4\bin\mysql.exe`,
    String.raw`C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe`,
    'mysql'
  ].filter(Boolean)
  return candidates.find((candidate) => candidate === 'mysql' || fs.existsSync(candidate))
}

function sqlString(value) {
  return `'${String(value).replace(/'/g, "''")}'`
}

function runMysql(mysqlBin, connection, sql) {
  return new Promise((resolve, reject) => {
    const child = spawn(mysqlBin, [
      `--user=${connection.user}`,
      `--host=${connection.host}`,
      '--protocol=TCP',
      `--port=${connection.port}`,
      '--default-character-set=utf8mb4',
      `--database=${connection.database}`,
      '--batch',
      '--raw',
      '--skip-column-names'
    ], {
      cwd: rootDir,
      env: { ...process.env, MYSQL_PWD: connection.password },
      stdio: ['pipe', 'pipe', 'pipe']
    })

    const stdout = []
    const stderr = []
    child.stdout.on('data', (chunk) => stdout.push(chunk))
    child.stderr.on('data', (chunk) => stderr.push(chunk))
    child.on('error', reject)
    child.on('close', (code) => {
      if (code === 0) {
        resolve(Buffer.concat(stdout).toString('utf8').trim())
        return
      }
      reject(new Error(`MySQL 命令失败（退出码 ${code}）：${Buffer.concat(stderr).toString('utf8').trim()}`))
    })
    child.stdin.end(Buffer.from(sql, 'utf8'))
  })
}

async function listTables(mysqlBin, connection) {
  const output = await runMysql(mysqlBin, connection, 'SELECT table_name FROM information_schema.tables WHERE table_schema=DATABASE() ORDER BY table_name;')
  return new Set(output ? output.split(/\r?\n/) : [])
}

async function listColumns(mysqlBin, connection, table) {
  const output = await runMysql(mysqlBin, connection, `SELECT column_name FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=${sqlString(table)} ORDER BY ordinal_position;`)
  return new Set(output ? output.split(/\r?\n/) : [])
}

async function scalar(mysqlBin, connection, sql) {
  const output = await runMysql(mysqlBin, connection, sql)
  return Number(output.split(/\r?\n/)[0] || 0)
}

function assertLocalSafety(connection) {
  const appEnv = loadEnvValue('APP_ENV').trim().toLowerCase()
  if (appEnv === 'production') throw new Error('已拒绝：APP_ENV=production，演示数据只允许本机开发环境')
  if (!LOCAL_HOSTS.has(connection.host.toLowerCase())) {
    throw new Error(`已拒绝：数据库主机 ${connection.host} 不是本机回环地址`)
  }
  if (!connection.database) throw new Error('已拒绝：MYSQL_DSN 未指定数据库')
}

async function assertSchema(mysqlBin, connection, tables) {
  const required = ['users', 'products', 'review_tasks', 'trade_search_outbox']
  const missing = required.filter((table) => !tables.has(table))
  if (missing.length) throw new Error(`本机数据库缺少必要表：${missing.join(', ')}。请先单独确认迁移状态，本脚本不会自动执行迁移。`)

  const productColumns = await listColumns(mysqlBin, connection, 'products')
  const requiredColumns = ['seller_id', 'title', 'description', 'category', 'price', 'original_price', 'condition_label', 'campus', 'status', 'review_status', 'favorite_count', 'created_at', 'updated_at', 'deleted_at']
  const missingColumns = requiredColumns.filter((column) => !productColumns.has(column))
  if (missingColumns.length) throw new Error(`products 表缺少必要字段：${missingColumns.join(', ')}`)
}

function buildSeedSql(reviewColumns) {
  const productStatements = DEMO_PRODUCTS.map((product, index) => {
    const createdOffset = DEMO_PRODUCTS.length - index
    const values = [
      '@demo_user_id', sqlString(product.title), sqlString(product.description), sqlString(product.category),
      product.price, product.originalPrice, sqlString(product.condition), sqlString('湛江科技学院'),
      sqlString('selling'), sqlString('approved'), 0,
      `DATE_SUB(NOW(3), INTERVAL ${createdOffset} MINUTE)`, `DATE_SUB(NOW(3), INTERVAL ${createdOffset} MINUTE)`, 'NULL'
    ].join(', ')
    return `
INSERT INTO products (seller_id,title,description,category,price,original_price,condition_label,campus,status,review_status,favorite_count,created_at,updated_at,deleted_at)
SELECT ${values}
WHERE NOT EXISTS (
  SELECT 1 FROM products WHERE seller_id=@demo_user_id AND title=${sqlString(product.title)}
);
UPDATE products SET
  description=${sqlString(product.description)}, category=${sqlString(product.category)}, price=${product.price},
  original_price=${product.originalPrice}, condition_label=${sqlString(product.condition)}, campus=${sqlString('湛江科技学院')},
  status='selling', review_status='approved', favorite_count=0, updated_at=NOW(3), deleted_at=NULL
WHERE seller_id=@demo_user_id AND title=${sqlString(product.title)};`
  }).join('\n')

  const decisionColumn = reviewColumns.has('decision_source') ? ',decision_source' : ''
  const decisionValue = reviewColumns.has('decision_source') ? ",'automatic'" : ''
  const decisionUpdate = reviewColumns.has('decision_source') ? ", decision_source='automatic'" : ''

  return `SET NAMES utf8mb4;
START TRANSACTION;
INSERT INTO users (open_id,nickname,avatar,school,campus,role,status,token_version,created_at,updated_at,deleted_at)
VALUES (${sqlString(DEMO_OPEN_ID)},${sqlString('本机预览卖家')},'',${sqlString('湛江科技学院')},${sqlString('湛江科技学院')},'user','active',1,NOW(3),NOW(3),NULL)
ON DUPLICATE KEY UPDATE nickname=VALUES(nickname), school=VALUES(school), campus=VALUES(campus), status='active', updated_at=NOW(3), deleted_at=NULL;
SET @demo_user_id := (SELECT id FROM users WHERE open_id=${sqlString(DEMO_OPEN_ID)} LIMIT 1);
${productStatements}
INSERT INTO review_tasks (object_type,object_id,submitter_id,risk,status,reason,operator_id${decisionColumn},created_at,updated_at)
SELECT 'product',p.id,@demo_user_id,'local secondhand demo','approved','',0${decisionValue},NOW(3),NOW(3)
FROM products p
WHERE p.seller_id=@demo_user_id AND p.title LIKE ${sqlString(`${DEMO_PREFIX}%`)}
ON DUPLICATE KEY UPDATE submitter_id=VALUES(submitter_id), risk=VALUES(risk), status='approved', reason='', operator_id=0${decisionUpdate}, updated_at=NOW(3);
DELETE tso FROM trade_search_outbox tso
JOIN products p ON p.id=tso.product_id
WHERE p.seller_id=@demo_user_id AND p.title LIKE ${sqlString(`${DEMO_PREFIX}%`)} AND tso.processed_at IS NULL;
INSERT INTO trade_search_outbox (product_id,operation,attempts,available_at,processed_at,last_error,created_at,updated_at)
SELECT p.id,'upsert',0,NOW(3),NULL,'',NOW(3),NOW(3)
FROM products p
WHERE p.seller_id=@demo_user_id AND p.title LIKE ${sqlString(`${DEMO_PREFIX}%`)};
COMMIT;`
}

async function seed(mysqlBin, connection, tables) {
  const before = await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM products p LEFT JOIN users u ON u.id=p.seller_id WHERE COALESCE(u.open_id,'')<>${sqlString(DEMO_OPEN_ID)} OR p.title NOT LIKE ${sqlString(`${DEMO_PREFIX}%`)};`)
  const reviewColumns = await listColumns(mysqlBin, connection, 'review_tasks')
  await runMysql(mysqlBin, connection, buildSeedSql(reviewColumns))
  const after = await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM products p LEFT JOIN users u ON u.id=p.seller_id WHERE COALESCE(u.open_id,'')<>${sqlString(DEMO_OPEN_ID)} OR p.title NOT LIKE ${sqlString(`${DEMO_PREFIX}%`)};`)
  const demoCount = await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM products p JOIN users u ON u.id=p.seller_id WHERE u.open_id=${sqlString(DEMO_OPEN_ID)} AND p.title LIKE ${sqlString(`${DEMO_PREFIX}%`)} AND p.deleted_at IS NULL;`)
  if (demoCount !== DEMO_PRODUCTS.length) throw new Error(`演示商品数量异常：期望 ${DEMO_PRODUCTS.length}，实际 ${demoCount}`)
  if (before !== after) throw new Error(`非演示商品数量发生变化：执行前 ${before}，执行后 ${after}`)
  console.log(`已写入 ${demoCount} 个本机演示商品；非演示商品保持 ${after} 个。`)
  if (!tables.has('product_images')) console.log('提示：本机库没有 product_images 表，商品仍会按无图状态展示。')
}

async function status(mysqlBin, connection) {
  const output = await runMysql(mysqlBin, connection, `
SELECT CONCAT('卖家 #',u.id,'  ',u.nickname) FROM users u WHERE u.open_id=${sqlString(DEMO_OPEN_ID)};
SELECT CONCAT('#',p.id,' | ',p.category,' | ¥',FORMAT(p.price/100,2),' | ',p.status,'/',p.review_status,' | ',p.title)
FROM products p JOIN users u ON u.id=p.seller_id
WHERE u.open_id=${sqlString(DEMO_OPEN_ID)} AND p.title LIKE ${sqlString(`${DEMO_PREFIX}%`)} AND p.deleted_at IS NULL
ORDER BY p.created_at DESC,p.id DESC;`)
  console.log(output || '当前没有本机二手演示数据。')
}

async function clear(mysqlBin, connection, tables) {
  const demoUserID = await scalar(mysqlBin, connection, `SELECT COALESCE((SELECT id FROM users WHERE open_id=${sqlString(DEMO_OPEN_ID)} LIMIT 1),0);`)
  if (!demoUserID) {
    console.log('当前没有本机二手演示数据，无需清理。')
    return
  }

  const guards = []
  guards.push(['非演示商品', await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM products WHERE seller_id=${demoUserID} AND title NOT LIKE ${sqlString(`${DEMO_PREFIX}%`)};`)])
  if (tables.has('posts')) guards.push(['帖子', await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM posts WHERE user_id=${demoUserID};`)])
  if (tables.has('comments')) guards.push(['帖子评论', await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM comments WHERE user_id=${demoUserID};`)])
  if (tables.has('product_comments')) guards.push(['商品留言', await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM product_comments WHERE user_id=${demoUserID};`)])
  if (tables.has('im_conversations')) guards.push(['非演示会话', await scalar(mysqlBin, connection, `SELECT COUNT(*) FROM im_conversations WHERE (buyer_id=${demoUserID} OR seller_id=${demoUserID}) AND reference_id NOT IN (SELECT id FROM products WHERE seller_id=${demoUserID} AND title LIKE ${sqlString(`${DEMO_PREFIX}%`)});`)])
  const unsafe = guards.filter(([, count]) => count > 0)
  if (unsafe.length) throw new Error(`已拒绝清理：演示用户出现非演示关联（${unsafe.map(([name, count]) => `${name} ${count}`).join('，')}）`)

  const statements = [
    'CREATE TEMPORARY TABLE local_demo_product_ids (id BIGINT UNSIGNED PRIMARY KEY);',
    `INSERT INTO local_demo_product_ids(id) SELECT id FROM products WHERE seller_id=${demoUserID} AND title LIKE ${sqlString(`${DEMO_PREFIX}%`)};`
  ]

  if (tables.has('notifications')) {
    const notificationColumns = await listColumns(mysqlBin, connection, 'notifications')
    if (notificationColumns.has('product_id')) statements.push('DELETE FROM notifications WHERE product_id IN (SELECT id FROM local_demo_product_ids);')
    if (notificationColumns.has('product_comment_id') && tables.has('product_comments')) statements.push("DELETE FROM notifications WHERE product_comment_id IN (SELECT id FROM product_comments WHERE product_id IN (SELECT id FROM local_demo_product_ids));")
  }
  if (tables.has('review_tasks') && tables.has('product_comments')) statements.push("DELETE FROM review_tasks WHERE object_type='product_comment' AND object_id IN (SELECT id FROM product_comments WHERE product_id IN (SELECT id FROM local_demo_product_ids));")
  if (tables.has('trade_write_outbox')) {
    if (tables.has('product_comments')) statements.push("DELETE FROM trade_write_outbox WHERE aggregate_type='product_comment' AND aggregate_id IN (SELECT id FROM product_comments WHERE product_id IN (SELECT id FROM local_demo_product_ids));")
    statements.push("DELETE FROM trade_write_outbox WHERE aggregate_type='product' AND aggregate_id IN (SELECT id FROM local_demo_product_ids);")
  }
  if (tables.has('product_comments')) statements.push('DELETE FROM product_comments WHERE product_id IN (SELECT id FROM local_demo_product_ids);')
  if (tables.has('im_conversations')) statements.push(`DELETE FROM im_conversations WHERE seller_id=${demoUserID} AND reference_id IN (SELECT id FROM local_demo_product_ids);`)
  if (tables.has('product_favorites')) statements.push('DELETE FROM product_favorites WHERE product_id IN (SELECT id FROM local_demo_product_ids);')
  if (tables.has('product_images')) statements.push('DELETE FROM product_images WHERE product_id IN (SELECT id FROM local_demo_product_ids);')
  if (tables.has('trade_search_outbox')) statements.push('DELETE FROM trade_search_outbox WHERE product_id IN (SELECT id FROM local_demo_product_ids);')
  statements.push("DELETE FROM review_tasks WHERE object_type IN ('product','trade_product') AND object_id IN (SELECT id FROM local_demo_product_ids);")
  statements.push('DELETE FROM products WHERE id IN (SELECT id FROM local_demo_product_ids);')
  statements.push(`DELETE FROM users WHERE id=${demoUserID} AND NOT EXISTS (SELECT 1 FROM products WHERE seller_id=${demoUserID});`)
  statements.push('DROP TEMPORARY TABLE local_demo_product_ids;')

  await runMysql(mysqlBin, connection, `SET NAMES utf8mb4; START TRANSACTION; ${statements.join('\n')} COMMIT;`)
  console.log('本机二手演示商品及其关联数据已清理。')
}

async function main() {
  const mode = (process.argv[2] || 'status').toLowerCase()
  if (!['seed', 'status', 'clear'].includes(mode)) throw new Error('用法：node scripts/local-secondhand-demo.cjs <seed|status|clear>')

  const connection = parseMysqlDsn(loadEnvValue('MYSQL_DSN'))
  assertLocalSafety(connection)
  const mysqlBin = mysqlBinary()
  if (!mysqlBin) throw new Error('未找到 mysql 客户端；如安装在自定义目录，请设置 MYSQL_BIN')
  console.log(`本机数据库：${connection.host}:${connection.port}/${connection.database}（模式：${mode}）`)

  const tables = await listTables(mysqlBin, connection)
  await assertSchema(mysqlBin, connection, tables)
  if (mode === 'seed') await seed(mysqlBin, connection, tables)
  if (mode === 'status') await status(mysqlBin, connection)
  if (mode === 'clear') await clear(mysqlBin, connection, tables)
}

main().catch((error) => {
  console.error(error.message)
  process.exitCode = 1
})
