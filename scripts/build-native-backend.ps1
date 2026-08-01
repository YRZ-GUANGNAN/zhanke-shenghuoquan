[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Version,

    [Parameter()]
    [ValidateNotNullOrEmpty()]
    [string]$OutputPath = "artifacts/campus-api"
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$backendRoot = Join-Path $projectRoot "backend"
$resolvedOutput = if ([System.IO.Path]::IsPathRooted($OutputPath)) {
    [System.IO.Path]::GetFullPath($OutputPath)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $projectRoot $OutputPath))
}
$outputDirectory = Split-Path -Parent $resolvedOutput
if (-not (Test-Path -LiteralPath $outputDirectory)) {
    New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
}

$commit = (& git -C $projectRoot rev-parse --short=12 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($commit)) {
    throw "Unable to read Git commit; refusing to create an untraceable release."
}
$dirtyPaths = (& git -C $projectRoot status --porcelain --untracked-files=all -- backend scripts/build-native-backend.ps1)
if ($LASTEXITCODE -ne 0) {
    throw "Unable to inspect the release source worktree."
}
if ($dirtyPaths) {
    throw "Release source is dirty; commit backend and build-script changes before packaging."
}
$buildTime = [DateTime]::UtcNow.ToString("o")
$ldflags = "-s -w -X main.buildVersion=$Version -X main.buildCommit=$commit -X main.buildTime=$buildTime"

$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGOEnabled = $env:CGO_ENABLED
try {
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    Push-Location $backendRoot
    try {
        & go build -trimpath -ldflags $ldflags -o $resolvedOutput ./cmd/server
        if ($LASTEXITCODE -ne 0) {
            throw "Go release build failed."
        }
    } finally {
        Pop-Location
    }
} finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGOEnabled
}

$checksum = (Get-FileHash -Algorithm SHA256 -LiteralPath $resolvedOutput).Hash.ToLowerInvariant()
$checksumPath = "$resolvedOutput.sha256"
Set-Content -LiteralPath $checksumPath -Value "$checksum  $(Split-Path -Leaf $resolvedOutput)" -Encoding ascii

Write-Host "Release artifact: $resolvedOutput"
Write-Host "SHA-256: $checksum"
Write-Host "Version: $Version / $commit / $buildTime"
