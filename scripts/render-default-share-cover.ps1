# Rebuild the owned typographic share asset. Requires Windows System.Drawing and Microsoft YaHei.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$repoPath = Split-Path $PSScriptRoot -Parent
$bitmap = New-Object System.Drawing.Bitmap 1000, 800
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
$graphics.Clear([System.Drawing.Color]::White)
$blue = New-Object System.Drawing.SolidBrush ([System.Drawing.ColorTranslator]::FromHtml('#2E7DD4'))
$ink = New-Object System.Drawing.SolidBrush ([System.Drawing.ColorTranslator]::FromHtml('#20354F'))
$muted = New-Object System.Drawing.SolidBrush ([System.Drawing.ColorTranslator]::FromHtml('#718197'))
$tint = New-Object System.Drawing.SolidBrush ([System.Drawing.ColorTranslator]::FromHtml('#F0F6FD'))
$white = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::White)
$titleFont = New-Object System.Drawing.Font 'Microsoft YaHei', 54, ([System.Drawing.FontStyle]::Bold), ([System.Drawing.GraphicsUnit]::Pixel)
$bodyFont = New-Object System.Drawing.Font 'Microsoft YaHei', 27, ([System.Drawing.FontStyle]::Regular), ([System.Drawing.GraphicsUnit]::Pixel)
$markFont = New-Object System.Drawing.Font 'Microsoft YaHei', 44, ([System.Drawing.FontStyle]::Bold), ([System.Drawing.GraphicsUnit]::Pixel)
$smallFont = New-Object System.Drawing.Font 'Microsoft YaHei', 20, ([System.Drawing.FontStyle]::Regular), ([System.Drawing.GraphicsUnit]::Pixel)
try {
  $graphics.FillRectangle($tint, 0, 592, 1000, 208)
  $graphics.FillEllipse($blue, 80, 80, 100, 100)
  $format = New-Object System.Drawing.StringFormat
  $format.Alignment = [System.Drawing.StringAlignment]::Center
  $format.LineAlignment = [System.Drawing.StringAlignment]::Center
  $graphics.DrawString('湛', $markFont, $white, [System.Drawing.RectangleF]::new(80, 78, 100, 100), $format)
  $graphics.DrawString('湛科生活圈', $titleFont, $ink, 78, 282)
  $graphics.DrawString('校园日常，在这里相遇', $bodyFont, $muted, 82, 382)
  $graphics.FillRectangle($blue, 84, 662, 44, 4)
  $graphics.DrawString('打开小程序，看看校园里的新鲜事', $smallFont, $muted, 82, 700)
  foreach ($relative in @('apps/miniapp/src/assets/share/default-share.png', 'apps/admin/public/images/default-share.png')) {
    $target = Join-Path $repoPath $relative
    [System.IO.Directory]::CreateDirectory((Split-Path $target -Parent)) | Out-Null
    $bitmap.Save($target, [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Output $relative
  }
  $format.Dispose()
} finally {
  $graphics.Dispose(); $bitmap.Dispose()
  foreach ($resource in @($blue,$ink,$muted,$tint,$white,$titleFont,$bodyFont,$markFont,$smallFont)) { $resource.Dispose() }
}
