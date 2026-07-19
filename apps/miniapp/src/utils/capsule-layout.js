const BUSINESS_ROW_HEIGHT = 48

function positiveNumber(value, fallback) {
  return Number.isFinite(value) && value > 0 ? value : fallback
}

function validMenuRect(menu) {
  return Boolean(
    menu &&
    Number.isFinite(menu.top) &&
    Number.isFinite(menu.bottom) &&
    Number.isFinite(menu.left) &&
    Number.isFinite(menu.right) &&
    menu.width > 0 &&
    menu.height > 0 &&
    menu.bottom > menu.top &&
    menu.right > menu.left
  )
}

function calculateCapsuleLayout(windowInfo, menu) {
  const statusBarHeight = Math.max(20, positiveNumber(windowInfo?.statusBarHeight, 20))
  const fallbackSystemRowHeight = statusBarHeight + 52
  const systemRowHeight = validMenuRect(menu)
    ? Math.max(menu.bottom + 8, fallbackSystemRowHeight)
    : fallbackSystemRowHeight

  return {
    systemRowHeight,
    businessRowTop: systemRowHeight,
    businessRowHeight: BUSINESS_ROW_HEIGHT,
    totalHeaderHeight: systemRowHeight + BUSINESS_ROW_HEIGHT,
  }
}

module.exports = { BUSINESS_ROW_HEIGHT, calculateCapsuleLayout }
