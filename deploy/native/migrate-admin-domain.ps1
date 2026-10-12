[CmdletBinding()]
param(
    [ValidateSet('zhandui.top')]
    [string]$Domain = 'zhandui.top',
    [ValidatePattern('^\d{1,3}(\.\d{1,3}){3}$')]
    [string]$ExpectedAddress = '8.138.227.107',
    [ValidatePattern('^\d{1,3}(\.\d{1,3}){3}$')]
    [string]$HostAddress = '8.138.227.107',
    [ValidatePattern('^[A-Za-z0-9._-]+$')]
    [string]$RemoteUser = 'admin',
    [string]$KeyPath = (Join-Path $env:USERPROFILE '.ssh\id_ed25519_zhandui'),
    [switch]$BuildAdmin,
    [switch]$Deploy,
    [switch]$RequestCertificate,
    [switch]$EnableHttps
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$adminDist = Join-Path $repoRoot 'apps\admin\dist'
$fullConfig = Join-Path $PSScriptRoot 'nginx-zhandui.top.conf'
$acmeConfig = Join-Path $PSScriptRoot 'nginx-zhandui.top.acme.conf'
$remote = "$RemoteUser@$HostAddress"
$sshOptions = @('-i', $KeyPath, '-o', 'BatchMode=yes', '-o', 'IdentitiesOnly=yes', '-o', 'ConnectTimeout=12')

function Invoke-Remote {
    param([Parameter(Mandatory = $true)][string]$Command)

    & ssh @sshOptions $remote $Command
    if ($LASTEXITCODE -ne 0) {
        throw "Remote command failed (exit=$LASTEXITCODE): $Command"
    }
}

function Invoke-RemoteCapture {
    param([Parameter(Mandatory = $true)][string]$Command)

    $output = & ssh @sshOptions $remote $Command 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Remote capture failed (exit=$LASTEXITCODE): $Command`n$($output | Out-String)"
    }
    return (($output | Out-String).Trim())
}

function Copy-Remote {
    param(
        [Parameter(Mandatory = $true)][string]$LocalPath,
        [Parameter(Mandatory = $true)][string]$RemotePath
    )

    & scp @sshOptions $LocalPath ("{0}:{1}" -f $remote, $RemotePath)
    if ($LASTEXITCODE -ne 0) {
        throw "Upload failed (exit=$LASTEXITCODE): $LocalPath"
    }
}

function Assert-SafeCurrentTarget {
    param([Parameter(Mandatory = $true)][string]$Target)

    if ($Target -notmatch '^/var/www/campus-admin-zhandui/releases/[A-Za-z0-9._-]+$') {
        throw "Unexpected current symlink target: $Target"
    }
}

function Assert-SafeNginxTarget {
    param([Parameter(Mandatory = $true)][string]$Target)

    if ($Target -notmatch '^/etc/nginx/sites-available/[A-Za-z0-9._-]+$') {
        throw "Unexpected Nginx symlink target: $Target"
    }
}

if ($ExpectedAddress -ne $HostAddress) {
    throw 'ExpectedAddress and HostAddress must be identical for a single-host migration.'
}
if (-not (Test-Path -LiteralPath $KeyPath -PathType Leaf)) {
    throw "SSH key was not found: $KeyPath"
}
foreach ($requiredPath in @($fullConfig, $acmeConfig)) {
    if (-not (Test-Path -LiteralPath $requiredPath -PathType Leaf)) {
        throw "Deployment asset was not found: $requiredPath"
    }
}

$dnsRecords = @(Resolve-DnsName -Name $Domain -Type A -ErrorAction Stop | Where-Object { $_.Type -eq 'A' -and $_.Name -eq $Domain -and $_.IPAddress })
$dnsAddresses = @($dnsRecords | ForEach-Object { $_.IPAddress } | Sort-Object -Unique)
if ($dnsAddresses.Count -ne 1 -or $dnsAddresses[0] -ne $ExpectedAddress) {
    $actual = if ($dnsAddresses.Count -gt 0) { $dnsAddresses -join ', ' } else { '<no A record>' }
    throw "$Domain resolves to [$actual], not exactly $ExpectedAddress; no remote write was attempted."
}
$aaaaRecords = @(Resolve-DnsName -Name $Domain -Type AAAA -ErrorAction SilentlyContinue | Where-Object { $_.Type -eq 'AAAA' -and $_.Name -eq $Domain -and $_.IPAddress })
if ($aaaaRecords.Count -gt 0) {
    $aaaaAddresses = @($aaaaRecords | ForEach-Object { $_.IPAddress } | Sort-Object -Unique)
    throw "$Domain has an AAAA record [$($aaaaAddresses -join ', ')]; remove it or add an explicitly supported IPv6 host before deployment."
}
Write-Output "DNS points only to $ExpectedAddress and has no AAAA record."

if ($BuildAdmin) {
    Push-Location $repoRoot
    try {
        & npm run build:admin
        if ($LASTEXITCODE -ne 0) {
            throw "Admin build failed (exit=$LASTEXITCODE)."
        }
    } finally {
        Pop-Location
    }
}

if (-not ($Deploy -or $RequestCertificate -or $EnableHttps)) {
    Write-Output 'Read-only checks complete. Use -Deploy for remote writes; certificate and HTTPS switches are -RequestCertificate and -EnableHttps.'
    exit 0
}
if (-not $Deploy) {
    throw '-RequestCertificate and -EnableHttps require -Deploy.'
}
if (-not ($RequestCertificate -or $EnableHttps)) {
    throw '-Deploy requires -RequestCertificate or -EnableHttps so the new domain is not left on the ACME-only site.'
}
if (-not (Test-Path -LiteralPath $adminDist -PathType Container)) {
    throw "Admin build directory was not found: $adminDist; run -BuildAdmin first."
}

$releaseId = 'admin-' + (Get-Date -Format 'yyyyMMdd_HHmmss')
$archiveName = "$releaseId.tar.gz"
$archivePath = Join-Path $env:TEMP $archiveName
$remoteStage = "/tmp/$releaseId"
$remoteArchive = "/tmp/$archiveName"
$adminRoot = '/var/www/campus-admin-zhandui'
$releasePath = "$adminRoot/releases/$releaseId"
$remoteConfigName = "campus-admin-$Domain"
$remoteAcmeConfigName = "$remoteConfigName.acme"
$remoteAcmeLink = "/etc/nginx/sites-enabled/$remoteConfigName.conf"
$remoteFullPath = "/etc/nginx/sites-available/$remoteConfigName.conf"
$probeName = "migration-$releaseId"
$probeContent = 'campus-domain-probe'
$currentSwitched = $false
$nginxLinkChanged = $false
$oldCurrent = ''
$oldNginxLink = ''

try {
    Invoke-Remote "sudo install -d -o root -g root -m 0755 '$adminRoot/releases'"
    $oldCurrent = Invoke-RemoteCapture "readlink '$adminRoot/current' || true"
    if ($oldCurrent -ne '') {
        Assert-SafeCurrentTarget $oldCurrent
    }
    $oldNginxLink = Invoke-RemoteCapture "readlink '$remoteAcmeLink' || true"
    if ($oldNginxLink -ne '') {
        Assert-SafeNginxTarget $oldNginxLink
    }

    & tar.exe -czf $archivePath -C $adminDist .
    if ($LASTEXITCODE -ne 0) {
        throw "Admin archive failed (exit=$LASTEXITCODE)."
    }
    $localHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()

    Invoke-Remote "sudo install -d -o root -g root -m 0755 '$remoteStage' && sudo install -d -o root -g root -m 0755 '/var/www/letsencrypt/.well-known/acme-challenge'"
    Copy-Remote -LocalPath $archivePath -RemotePath $remoteArchive
    Copy-Remote -LocalPath $acmeConfig -RemotePath "$remoteStage/$remoteAcmeConfigName.conf"
    Copy-Remote -LocalPath $fullConfig -RemotePath "$remoteStage/$remoteConfigName.conf"

    $remoteHash = Invoke-RemoteCapture "sha256sum '$remoteArchive' | cut -d ' ' -f1"
    if ($remoteHash.ToLowerInvariant() -ne $localHash) {
        throw "Archive checksum mismatch: local=$localHash remote=$remoteHash"
    }
    Write-Output "Archive checksum verified: $localHash"

    Invoke-Remote "sudo install -d -o root -g root -m 0755 '$releasePath' && sudo tar -xzf '$remoteArchive' -C '$releasePath' && sudo find '$releasePath' -type d -exec chmod 0755 {} + && sudo find '$releasePath' -type f -exec chmod 0644 {} + && sudo test -s '$releasePath/index.html'"
    Invoke-Remote "sudo ln -sfn '$releasePath' '$adminRoot/current.next' && sudo mv -Tf '$adminRoot/current.next' '$adminRoot/current'"
    $currentSwitched = $true

    Invoke-Remote "sudo install -o root -g root -m 0644 '$remoteStage/$remoteAcmeConfigName.conf' '/etc/nginx/sites-available/$remoteAcmeConfigName.conf' && sudo rm -f '$remoteAcmeLink' && sudo ln -sfn '/etc/nginx/sites-available/$remoteAcmeConfigName.conf' '$remoteAcmeLink'"
    $nginxLinkChanged = $true
    Invoke-Remote "sudo nginx -t && sudo systemctl reload nginx"
    Invoke-Remote "printf '%s' '$probeContent' | sudo tee '/var/www/letsencrypt/.well-known/acme-challenge/$probeName' >/dev/null && curl -fsS -H 'Host: $Domain' 'http://127.0.0.1/.well-known/acme-challenge/$probeName' | grep -Fx -- '$probeContent' && sudo rm -f '/var/www/letsencrypt/.well-known/acme-challenge/$probeName'"
    Write-Output "ACME webroot probe passed for $Domain."

    if ($RequestCertificate) {
        Invoke-Remote "sudo certbot certonly --webroot -w /var/www/letsencrypt -d $Domain --non-interactive --keep-until-expiring"
        Write-Output "Certificate request completed for $Domain."
    }

    if ($EnableHttps) {
        Invoke-Remote "sudo test -s '/etc/letsencrypt/live/$Domain/fullchain.pem' && sudo test -s '/etc/letsencrypt/live/$Domain/privkey.pem' && sudo install -o root -g root -m 0644 '$remoteStage/$remoteConfigName.conf' '$remoteFullPath' && sudo rm -f '$remoteAcmeLink' && sudo ln -sfn '$remoteFullPath' '$remoteAcmeLink'"
        Invoke-Remote "sudo nginx -t && sudo systemctl reload nginx"
        Write-Output "Enabled HTTPS config for $Domain."
    }

    Write-Output "Admin release $releaseId is active at $releasePath."
} catch {
    Write-Warning "Deployment failed: $($_.Exception.Message)"
    if ($nginxLinkChanged) {
        try {
            if ($oldNginxLink -ne '') {
                Invoke-Remote "sudo ln -sfn '$oldNginxLink' '$remoteAcmeLink' && sudo nginx -t && sudo systemctl reload nginx"
            } else {
                Invoke-Remote "sudo rm -f '$remoteAcmeLink' && sudo nginx -t && sudo systemctl reload nginx"
            }
            Write-Warning 'Previous Nginx link was restored.'
        } catch {
            Write-Warning "Nginx rollback failed and needs manual review: $($_.Exception.Message)"
        }
    }
    if ($currentSwitched) {
        try {
            if ($oldCurrent -ne '') {
                Invoke-Remote "sudo ln -sfn '$oldCurrent' '$adminRoot/current.next' && sudo mv -Tf '$adminRoot/current.next' '$adminRoot/current'"
            } else {
                Invoke-Remote "sudo rm -f '$adminRoot/current'"
            }
            Write-Warning 'Previous Admin current symlink was restored.'
        } catch {
            Write-Warning "Admin symlink rollback failed and needs manual review: $($_.Exception.Message)"
        }
    }
    throw
} finally {
    Remove-Item -LiteralPath $archivePath -Force -ErrorAction SilentlyContinue
    try {
        Invoke-Remote "rm -f '$remoteArchive'; rm -rf '$remoteStage'"
    } catch {
        Write-Warning "Remote temporary cleanup failed: $($_.Exception.Message)"
    }
}
