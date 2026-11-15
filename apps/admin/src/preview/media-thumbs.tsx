import { useState } from 'react'

/** 管理端素材预览网关：302 到 COS 签名地址，含未审核与未被业务引用的资产。 */
export function adminAssetContentUrl(assetId: string): string {
  return `/api/v1/admin/media/assets/${encodeURIComponent(assetId)}/content`
}

/** 面向普通用户的媒体内容网关。 */
export function assetContentUrl(assetId: string): string {
  return `/api/v1/media/assets/${encodeURIComponent(assetId)}/content`
}

const contentUrlPattern = /\/media\/assets\/([0-7][0-9A-HJKMNP-TV-Z]{25})\/content/

/** 后台展示业务图片时优先走管理端预览网关，保证未发布内容也能核对。 */
export function adminPreviewUrl(url: string): string {
  const match = contentUrlPattern.exec(url)
  return match ? adminAssetContentUrl(match[1]) : url
}

/** 业务数据里已存的图片地址（帖子/商品为 content 链接，认证证明为签名链接）。 */
export function isDirectImageUrl(value: string | undefined): value is string {
  return typeof value === 'string' && /^https?:\/\//.test(value.trim())
}

export function RealImage({ src, alt, className }: { src: string; alt: string; className?: string }) {
  const [failed, setFailed] = useState(false)
  if (failed) {
    return <span className={`preview-image-fallback${className ? ` ${className}` : ''}`} role="img" aria-label={`${alt}（加载失败）`}>图片加载失败</span>
  }
  return <img
    className={className}
    src={src}
    alt={alt}
    loading="lazy"
    onError={() => setFailed(true)}
  />
}

/** 图片墙：帖子/商品详情、认证证明等使用真实地址。 */
export function RealImageStrip({ urls, label }: { urls: string[]; label: string }) {
  const valid = urls.filter(isDirectImageUrl)
  if (valid.length === 0) return <p className="preview-image-empty">没有图片</p>
  return <div className="preview-image-strip" role="list" aria-label={`${label}，共 ${valid.length} 张图片`}>
    {valid.map((url, index) => {
      const preview = adminPreviewUrl(url)
      return <a
        key={`${url}-${index}`}
        role="listitem"
        className="preview-image-thumb preview-image-thumb--real"
        href={preview}
        target="_blank"
        rel="noreferrer"
        aria-label={`${label} 图片 ${index + 1}`}
      >
        <RealImage src={preview} alt={`${label} 图片 ${index + 1}`} />
      </a>
    })}
  </div>
}

/** 表格内的小缩略图：有真实地址时显示首图，否则「无图」。 */
export function RealImageCell({ url, label }: { url?: string; label: string }) {
  if (!isDirectImageUrl(url)) return <span className="preview-image-cell is-empty">无图</span>
  return <span className="preview-image-cell preview-image-cell--real" aria-label={`${label} 首图`}>
    <RealImage src={adminPreviewUrl(url)} alt={`${label} 首图`} />
  </span>
}
