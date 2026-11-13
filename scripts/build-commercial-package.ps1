# GoTeams commercial package builder (Windows). Entry: scripts/build-commercial-package.bat
# Usage: build-commercial-package.bat [cn|dev2|dev3|dev6] [-Offline|-Both] [-MinUpgradeVersion vX.Y.Z]
#   -MinUpgradeVersion is the manifest upgrade floor written into every package (default v0.0.1,
#   i.e. any commercial version may upgrade straight to this one). Never set it to a build number
#   you happened to produce last; raise it only for a genuinely non-direct upgrade.
#   -Both emits the standard and the offline package from a single build: VERSION is occupied once
#   and the frontend dist plus both Go binaries are compiled once, so the two .run files of one
#   release always carry the same version. Only payload/images/images.tar.gz differs.
#   With an argument the build is non-interactive and selects that config; with no
#   argument (e.g. double-click) the target environment is chosen interactively below.
# Flow per docs/design/商业版授权.md 16.6:
#   lock VERSION -> bump patch -> inject version/commit into goteams+goteamsctl (linux amd64)
#   -> restore web/node_modules with npm ci when missing/stale -> frontend build (vite)
#   -> whitelist payload copy -> manifest + SHA256SUMS -> gzip tar payload + entry check
#   -> payload extract re-verify -> leak scan
#   -> self-extracting .run (sh stub + verified gzip payload) + sidecar sha256.
#   -Offline variant: docker save+gzip every compose image into payload/images/images.tar.gz
#   (cache keyed by image-list hash) so install/upgrade docker load it instead of pulling.
#   Both variants: payload/runtime/ carries the upstream Docker Engine static tarball plus the
#   Compose V2 plugin binary and the systemd units, so goteamsctl install can bring up Docker
#   on a clean host without network access (an existing Docker is never modified). Those two
#   artifacts are fetched from pinned URLs + pinned SHA-256 into build/commercial/vendor/docker
#   (git-ignored) and reused by later builds.
#   The assembly + verification chain below runs once per selected variant; with -Both it runs
#   twice over the same version and the same compiled artifacts.
# The occupied version is never rolled back, even when the build fails afterwards.
# Requires Windows PowerShell 5.1+ and git/go/node/npm/tar in PATH (docker additionally for
# -Offline and -Both; network access additionally on the first build, until the vendor runtime
# artifacts are cached).

param(
    [Parameter(Position = 0)]
    [ValidateSet('cn', 'dev2', 'dev3', 'dev6')]
    [string]$Environment,
    [switch]$Offline,
    # Build the standard and the offline package in one run (one VERSION, one set of artifacts).
    [switch]$Both,
    # Release-policy floor: the lowest installed version this package may upgrade from.
    # It must never be derived from the previous build number — build/commercial/VERSION is a
    # single counter shared by every environment and is consumed by failed builds too, so a
    # floor taken from it would pin each package to one specific predecessor and reject any
    # host that skipped a build. Raise it only when an upgrade genuinely stops being direct.
    [string]$MinUpgradeVersion = 'v0.0.1'
)

$ErrorActionPreference = 'Stop'

# --- Target environment resolution --------------------------------------------
# The .bat forwards its arguments untouched. When no environment argument is
# given, resolve it interactively here so environments stay single-sourced and
# are never duplicated in the .bat launcher. Non-interactive callers (CI / schedulers)
# keep the historical 'cn' default.
if (-not $PSBoundParameters.ContainsKey('Environment')) {
    if ([Console]::IsInputRedirected) {
        Write-Host '[package] no environment argument given; defaulting to cn'
        $Environment = 'cn'
    } else {
        $envPrompt = @(
            '请选择商业包打包目标环境 (build target environment):',
            '  [1] cn   -> config_embed_cn.ini   + docker-compose.yml',
            '  [2] dev2 -> config_embed_dev2.ini + docker-compose-dev2.yml',
            '  [3] dev3 -> config_embed_dev3.ini + docker-compose-dev3.yml',
            '  [4] dev6 -> config_embed_dev6.ini + docker-compose-dev6.yml',
            ''
        ) -join [Environment]::NewLine
        while ($true) {
            Write-Host $envPrompt
            $choice = Read-Host '请输入 1、2、3 或 4，直接回车默认 cn'
            if ([string]::IsNullOrWhiteSpace($choice))           { $Environment = 'cn';   break }
            if ($choice -eq '1' -or $choice -ieq 'cn')           { $Environment = 'cn';   break }
            if ($choice -eq '2' -or $choice -ieq 'dev2')         { $Environment = 'dev2'; break }
            if ($choice -eq '3' -or $choice -ieq 'dev3')         { $Environment = 'dev3'; break }
            if ($choice -eq '4' -or $choice -ieq 'dev6')         { $Environment = 'dev6'; break }
            Write-Host "无效输入 '$choice'，请输入 1、2、3 或 4" -ForegroundColor Yellow
        }
        Write-Host "[package] 已选择目标环境: $Environment"
    }
}

# --- Package variant resolution -------------------------------------------------
# The offline package embeds a docker-save archive of every compose image so that
# install/upgrade run fully without network access. It shares the same VERSION numbering,
# verification chain and layout as the standard package; only the extra payload file and
# the -offline name suffix differ. CI callers default to the standard package.
# -Both builds the two variants off a single VERSION bump and a single set of compiled
# artifacts, so one release never ships a standard and an offline package under different
# versions; -Offline then adds nothing, and passing both is simply -Both.
$variants = @('standard')
if ($Both) {
    $variants = @('standard', 'offline')
    Write-Host '[package] -Both given: building the standard and the offline package'
} elseif ($PSBoundParameters.ContainsKey('Offline')) {
    if ($Offline) { $variants = @('offline') }
} elseif ([Console]::IsInputRedirected) {
    Write-Host '[package] no -Offline switch given; building the standard (online) package'
} else {
    $variantPrompt = @(
        '请选择打包变体:',
        '  [1] 标准包（安装/升级时需联网拉取 docker 镜像）',
        '  [2] 离线包（内含镜像 tar.gz，安装/升级免联网，包体积较大）',
        '  [3] 两个都打（标准包 + 离线包，共用同一版本号，产物只编译一次）',
        ''
    ) -join [Environment]::NewLine
    while ($true) {
        Write-Host $variantPrompt
        $choice = Read-Host '请输入 1、2 或 3，直接回车默认标准包'
        if ([string]::IsNullOrWhiteSpace($choice) -or $choice -eq '1') { break }
        if ($choice -eq '2') { $variants = @('offline'); break }
        if ($choice -eq '3') { $variants = @('standard', 'offline'); break }
        Write-Host "无效输入 '$choice'，请输入 1、2 或 3" -ForegroundColor Yellow
    }
    Write-Host "[package] 已选择打包变体: $($variants -join ' + ')"
}
$needOffline = $variants -contains 'offline'

$repoRoot    = Split-Path -Parent $PSScriptRoot
$versionDir  = Join-Path $repoRoot 'build\commercial'
$versionFile = Join-Path $versionDir 'VERSION'
$lockFile    = Join-Path $versionDir 'VERSION.lock'
$stagingRoot = Join-Path $versionDir 'staging'
$outputDir   = Join-Path $versionDir 'dist'
$composeRelative = if ($Environment -eq 'dev2') { 'docker\commercial\docker-compose-dev2.yml' } elseif ($Environment -eq 'dev3') { 'docker\commercial\docker-compose-dev3.yml' } elseif ($Environment -eq 'dev6') { 'docker\commercial\docker-compose-dev6.yml' } else { 'docker\commercial\docker-compose.yml' }
$composeFile = Join-Path $repoRoot $composeRelative
$buildTags   = if ($Environment -eq 'dev2') { 'commercial_embed,commercial_dev2' } elseif ($Environment -eq 'dev3') { 'commercial_embed,commercial_dev3' } elseif ($Environment -eq 'dev6') { 'commercial_embed,commercial_dev6' } else { 'commercial_embed' }
$webDir      = Join-Path $repoRoot 'web'
$sharedDir   = ''   # binaries built once per run, then copied into every package staging
$newVersion  = ''   # non-empty once this run has occupied a VERSION number
$packageList = @()
$versionOccupied = $false   # true only after the bumped version is written back to VERSION

# --- Bundled Docker runtime (pinned upstream release artifacts) --------------------
# goteamsctl install uses payload/runtime/ to bring up Docker on a host that has none.
# Both hashes are the SHA-256 of the upstream release files as published; bumping a
# version means updating the URL-derived name and its hash together, never "latest".
$dockerEngineVersion  = '27.5.1'
$dockerComposeVersion = 'v2.40.3'
$runtimeVendorDir     = Join-Path $versionDir 'vendor\docker'
$runtimeArtifacts     = @(
    [pscustomobject]@{
        Package  = "payload/runtime/docker-$dockerEngineVersion.tgz"
        Name     = "docker-$dockerEngineVersion.tgz"
        Url      = "https://download.docker.com/linux/static/stable/x86_64/docker-$dockerEngineVersion.tgz"
        SHA256   = '4f798b3ee1e0140eab5bf30b0edc4e84f4cdb53255a429dc3bbae9524845d640'
    },
    [pscustomobject]@{
        Package  = 'payload/runtime/docker-compose'
        Name     = 'docker-compose'
        Url      = "https://github.com/docker/compose/releases/download/$dockerComposeVersion/docker-compose-linux-x86_64"
        SHA256   = 'dba9d98e1ba5bfe11d88c99b9bd32fc4a0624a30fafe68eea34d61a3e42fd372'
    }
)

# Repo-side source of the systemd units goteamsctl drops into /etc/systemd/system when it
# installs the bundled runtime; the names must match the unit list in cmd/goteamsctl.
$systemdUnitSrcDir = 'docker/commercial/runtime/systemd'
$runtimeUnits      = @('containerd.service', 'docker.service', 'docker.socket')

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$privateKeyMarkers = @(
    '-----BEGIN EC PRIVATE KEY-----',
    '-----BEGIN RSA PRIVATE KEY-----',
    '-----BEGIN PRIVATE KEY-----',
    '-----BEGIN OPENSSH PRIVATE KEY-----'
)

function Fail([string]$message) { throw $message }
function Info([string]$message) { Write-Host "[package] $message" }

# Validates the manifest upgrade floor before VERSION is consumed, so a bad -MinUpgradeVersion
# never burns a release number. The floor must be a strict lower bound: a floor equal to or
# above the package version would leave nothing that can upgrade to it.
function Test-MinUpgradeFloor([string]$floor, [string]$packageVersion) {
    if ($floor -notmatch '^v\d+\.\d+\.\d+$') { Fail "invalid -MinUpgradeVersion '$floor' (expected vX.Y.Z)" }
    $floorParts    = $floor.Substring(1).Split('.')
    $packageParts  = $packageVersion.Substring(1).Split('.')
    for ($i = 0; $i -lt 3; $i++) {
        $f = [int]$floorParts[$i]
        $p = [int]$packageParts[$i]
        if ($f -lt $p) { return }
        if ($f -gt $p) { Fail "-MinUpgradeVersion $floor must be lower than the package version $packageVersion" }
    }
    Fail "-MinUpgradeVersion $floor must be lower than the package version $packageVersion"
}

# Compress-GzipFile gzips srcPath into dstPath via GzipStream (no gzip.exe dependency);
# docker load auto-detects gzip when loading the archive on the install host.
function Compress-GzipFile([string]$srcPath, [string]$dstPath) {
    $src = [System.IO.File]::OpenRead($srcPath)
    try {
        $dst = [System.IO.File]::Create($dstPath)
        try {
            $gz = New-Object System.IO.Compression.GzipStream($dst, [System.IO.Compression.CompressionLevel]::Optimal)
            try { $src.CopyTo($gz) } finally { $gz.Dispose() }
        } finally { $dst.Dispose() }
    } finally { $src.Dispose() }
}

function Require-Tool([string]$name) {
    if ($null -eq (Get-Command $name -ErrorAction SilentlyContinue)) {
        Fail "required tool '$name' not found in PATH"
    }
}

# Save-VendorRuntime downloads (or reuses) the pinned Docker runtime artifacts in the
# git-ignored vendor cache. A cached file is only trusted after its SHA-256 matches the
# pin, so a truncated or tampered cache self-heals; anything unverifiable fails the build
# instead of shipping an unverified third-party binary.
function Save-VendorRuntime {
    New-Item -ItemType Directory -Force -Path $runtimeVendorDir | Out-Null
    # PowerShell 5.1 may default to TLS 1.0; the download hosts require TLS 1.2.
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $previousProgress = $ProgressPreference
    try {
        $ProgressPreference = 'SilentlyContinue'   # the default progress bar makes IWR crawl
        foreach ($artifact in $runtimeArtifacts) {
            $cached = Join-Path $runtimeVendorDir $artifact.Name
            if (Test-Path -LiteralPath $cached) {
                $cachedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $cached).Hash.ToLowerInvariant()
                if ($cachedHash -eq $artifact.SHA256) {
                    Info "runtime artifact cache hit: $($artifact.Name)"
                    continue
                }
                Info "runtime artifact hash drifted ($cachedHash), redownloading: $($artifact.Name)"
                Remove-Item -Force -LiteralPath $cached
            }
            $tmp = "$cached.part"
            if (Test-Path -LiteralPath $tmp) { Remove-Item -Force -LiteralPath $tmp }
            Info "downloading runtime artifact: $($artifact.Url)"
            try {
                Invoke-WebRequest -Uri $artifact.Url -OutFile $tmp -UseBasicParsing -TimeoutSec 1800
            } catch {
                Fail "cannot download runtime artifact $($artifact.Url): $($_.Exception.Message)"
            }
            if (-not (Test-Path -LiteralPath $tmp)) { Fail "runtime artifact download produced no file: $($artifact.Name)" }
            $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $tmp).Hash.ToLowerInvariant()
            if ($actual -ne $artifact.SHA256) {
                Remove-Item -Force -LiteralPath $tmp
                Fail "runtime artifact sha256 mismatch for $($artifact.Name): got $actual expected $($artifact.SHA256)"
            }
            Move-Item -Force -LiteralPath $tmp -Destination $cached
            Info "runtime artifact ready: $($artifact.Name)"
        }
    } finally {
        $ProgressPreference = $previousProgress
    }
}

# Windows ships bsdtar at System32; GNU tar from other toolchains must not be picked up.
$winTar = Join-Path $env:SystemRoot 'System32\tar.exe'

function Invoke-Native([string]$file, [string[]]$arguments, [string]$workDir) {
    if ($workDir) { Push-Location $workDir }
    try {
        & $file @arguments
        if ($LASTEXITCODE -ne 0) { Fail "$file $($arguments -join ' ') failed with exit code $LASTEXITCODE" }
    } finally {
        if ($workDir) { Pop-Location }
    }
}

# Test-FrontendDepsStale reports whether web/node_modules must be restored before the Vite
# build: the tree is missing, older than package-lock.json, or `npm ls` finds declared
# dependencies that are not installed (a partial node_modules makes Vite fail with
# "failed to resolve import"). Restoring only when stale keeps repeat and offline builds fast.
function Test-FrontendDepsStale {
    $modules = Join-Path $webDir 'node_modules'
    if (-not (Test-Path $modules)) { return $true }
    $lock = Join-Path $webDir 'package-lock.json'
    if ((Test-Path $lock) -and
        (Get-Item -LiteralPath $lock).LastWriteTimeUtc -gt (Get-Item -LiteralPath $modules).LastWriteTimeUtc) {
        return $true
    }
    Push-Location $webDir
    try {
        & npm ls --depth=0 --silent *> $null
        return ($LASTEXITCODE -ne 0)
    } finally {
        Pop-Location
    }
}

# Reject anything that must never ship inside the commercial package.
function Test-LeakFree([string]$rootDir) {
    $dirs = Get-ChildItem -Path $rootDir -Recurse -Directory -Force
    foreach ($d in $dirs) {
        if ($d.Name -eq '.git' -or $d.Name -eq 'node_modules') {
            Fail "forbidden directory inside package: $($d.FullName.Substring($rootDir.Length))"
        }
    }
    $files = Get-ChildItem -Path $rootDir -Recurse -File -Force
    foreach ($f in $files) {
        $rel  = $f.FullName.Substring($rootDir.Length).TrimStart('\', '/')
        $name = $f.Name
        if ($name -match '\.(go|vue|map|log|sql|dump|bak|sqlite|sqlite3|db|pem|key|pfx|p12|jks|exe|dll|so|o|a|zip|7z)$') {
            Fail "forbidden artifact inside package: $rel"
        }
        if ($name -ieq '.env' -or $name -like '.env.*') { Fail "environment file leaked: $rel" }
        if ($name -like '*.ini' -and $name -notlike '*.ini.example') { Fail "non-example ini inside package: $rel" }
        if ($name -like 'config_dev*' -or $name -ieq 'config_loc.ini') { Fail "development config leaked: $rel" }
        if ($f.Length -le 2MB) {
            $text = [System.IO.File]::ReadAllText($f.FullName)
            foreach ($marker in $privateKeyMarkers) {
                if ($text.Contains($marker)) { Fail "private key material inside package: $rel" }
            }
        }
    }
}

# Read SHA256SUMS lines and re-verify every entry under extractedDir.
function Test-ExtractedSums([string]$extractedDir) {
    $sumsPath = Join-Path $extractedDir 'SHA256SUMS'
    if (-not (Test-Path $sumsPath)) { Fail 'extracted package has no SHA256SUMS' }
    $lines = [System.IO.File]::ReadAllText($sumsPath) -split "`n"
    foreach ($line in $lines) {
        $line = $line.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $parts = $line -split '\s+', 2
        if ($parts.Count -ne 2 -or $parts[0] -notmatch '^[0-9a-f]{64}$') { Fail "bad SHA256SUMS line: $line" }
        $rel = $parts[1].Trim()
        if ($rel -match '\\|\.\.' -or [System.IO.Path]::IsPathRooted($rel)) { Fail "unsafe path in SHA256SUMS: $rel" }
        $full = Join-Path $extractedDir ($rel -replace '/', '\')
        if (-not (Test-Path $full)) { Fail "SHA256SUMS lists missing file: $rel" }
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $full).Hash.ToLowerInvariant()
        if ($actual -ne $parts[0]) { Fail "sha256 mismatch after extraction: $rel" }
    }
}

# Add-ManifestFile hashes one file under $stagingDir and appends it to the manifest entry and
# SHA256SUMS line lists. The lists are passed in so a caller owns their lifetime.
function Add-ManifestFile([string]$stagingDir, [string]$relPath, $files, $sums) {
    $full = Join-Path $stagingDir ($relPath -replace '/', '\')
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $full).Hash.ToLowerInvariant()
    $files.Add([ordered]@{ path = $relPath; sha256 = $hash })
    $sums.Add("$hash  $relPath")
}

# New-CommercialPackage assembles and verifies one package variant from the artifacts this run
# already produced: whitelist payload copy -> manifest + SHA256SUMS -> leak scan -> gzip tar
# payload + entry check -> payload extract re-verify -> self-extracting .run + sidecar sha256.
# Every variant runs the identical chain; $WithImages only adds payload/images/images.tar.gz,
# so -Both yields two independently verified packages carrying the same version.
function New-CommercialPackage([string]$baseName, [bool]$WithImages) {
    $staging = Join-Path $stagingRoot $baseName
    if (Test-Path $staging) { Remove-Item -Recurse -Force $staging }
    $stagingDirs = @('payload\bin', 'payload\docker\nginx', 'payload\docs', 'payload\runtime\systemd', 'payload\web')
    if ($WithImages) { $stagingDirs += 'payload\images' }
    foreach ($relDir in $stagingDirs) {
        New-Item -ItemType Directory -Force -Path (Join-Path $staging $relDir) | Out-Null
    }
    # The binaries are compiled once per run; each package gets its own copy so the hashes,
    # the payload tar and the leak scan stay scoped to that package's tree.
    Copy-Item -LiteralPath (Join-Path $sharedDir 'goteamsctl') -Destination (Join-Path $staging 'goteamsctl') -Force
    Copy-Item -LiteralPath (Join-Path $sharedDir 'payload\bin\goteams') -Destination (Join-Path $staging 'payload\bin\goteams') -Force

    # --- whitelist payload copy ---------------------------------------------------
    $whitelist = [ordered]@{
        'payload/docker/docker-compose.yml'              = $composeRelative
        'payload/docker/nginx/web.conf'                  = 'docker/commercial/nginx/web.conf'
        'payload/docker/start-web.sh'                    = 'docker/commercial/start-web.sh'
        'payload/docs/license_install.md'                = 'docs/license_install.md'
        'payload/web/dist.tar.gz'                        = 'web/dist.tar.gz'
    }
    foreach ($artifact in $runtimeArtifacts) {
        $whitelist[$artifact.Package] = "build/commercial/vendor/docker/$($artifact.Name)"
    }
    foreach ($unit in $runtimeUnits) {
        $whitelist["payload/runtime/systemd/$unit"] = "$systemdUnitSrcDir/$unit"
    }
    if ($WithImages) { $whitelist['payload/images/images.tar.gz'] = $offlineBundleRel }
    foreach ($entry in $whitelist.GetEnumerator()) {
        $src = Join-Path $repoRoot ($entry.Value -replace '/', '\')
        $dst = Join-Path $staging ($entry.Key -replace '/', '\')
        if (-not (Test-Path $src)) { Fail "whitelist source missing: $($entry.Value)" }
        Copy-Item -LiteralPath $src -Destination $dst -Force
    }

    # --- manifest.json + SHA256SUMS ------------------------------------------------
    $manifestFiles = New-Object System.Collections.Generic.List[object]
    $sumLines      = New-Object System.Collections.Generic.List[string]
    Add-ManifestFile $staging 'goteamsctl' $manifestFiles $sumLines
    Add-ManifestFile $staging 'payload/bin/goteams' $manifestFiles $sumLines
    foreach ($entry in $whitelist.GetEnumerator()) { Add-ManifestFile $staging $entry.Key $manifestFiles $sumLines }
    $sortedFiles = @($manifestFiles | Sort-Object -Property path)
    $sortedSums  = @($sumLines | Sort-Object)

    $manifest = [ordered]@{
        name                   = 'goteams-commercial'
        version                = $newVersion
        edition                = 'commercial'
        build_environment      = $Environment
        arch                   = 'linux-amd64'
        min_upgrade_version    = $MinUpgradeVersion
        docker_engine_version  = $dockerEngineVersion
        docker_compose_version = $dockerComposeVersion
        images                 = $images
        files                  = $sortedFiles
    }
    [System.IO.File]::WriteAllText((Join-Path $staging 'manifest.json'),
        ((ConvertTo-Json -InputObject $manifest -Depth 5) + "`n"), $utf8NoBom)
    [System.IO.File]::WriteAllText((Join-Path $staging 'SHA256SUMS'),
        (($sortedSums -join "`n") + "`n"), $utf8NoBom)
    Info "manifest + SHA256SUMS written ($($sortedFiles.Count) files, upgrade floor $MinUpgradeVersion)"

    # --- leak scan on staging ------------------------------------------------------
    Test-LeakFree $staging
    Info 'leak scan on staging passed'

    # --- gzip tar payload + entry-name validation -----------------------------------
    # The same archive becomes the .run payload, so entry names are the extraction contract:
    # forward slashes only, every entry below the package directory, no traversal.
    $payloadTar = Join-Path $outputDir "$baseName.payload.tar.gz"
    if (Test-Path $payloadTar) { Remove-Item -Force $payloadTar }
    Invoke-Native $winTar @('-czf', $payloadTar, '-C', $stagingRoot, $baseName) $null
    $payloadEntries = @(& $winTar -tzf $payloadTar)
    if ($LASTEXITCODE -ne 0) { Fail 'cannot list payload tar entries' }
    $requiredEntries = @('manifest.json', 'SHA256SUMS', 'goteamsctl', 'payload/bin/goteams', 'payload/docs/license_install.md') +
        @($runtimeArtifacts.Package) +
        @($runtimeUnits | ForEach-Object { "payload/runtime/systemd/$_" })
    foreach ($rel in $requiredEntries) {
        if ($payloadEntries -notcontains "$baseName/$rel") { Fail "payload is missing $rel" }
    }
    foreach ($e in $payloadEntries) {
        if ($e -match '\\') { Fail "payload entry uses backslash separator: $e" }
        if ($e.StartsWith('/') -or $e -match '(^|/)\.\.(/|$)') { Fail "unsafe payload entry: $e" }
        if (-not $e.StartsWith("$baseName/")) { Fail "payload entry outside package directory: $e" }
    }
    Info "payload tar created: $payloadTar ($($payloadEntries.Count) entries)"

    # --- payload extract re-verify (structure + hashes + manifest + leak scan) ------
    $verifyRoot = Join-Path $stagingRoot '_verify'
    if (Test-Path $verifyRoot) { Remove-Item -Recurse -Force $verifyRoot }
    New-Item -ItemType Directory -Force -Path $verifyRoot | Out-Null
    Invoke-Native $winTar @('-xzf', $payloadTar, '-C', $verifyRoot) $null
    $extracted = Join-Path $verifyRoot $baseName
    $expected = @('manifest.json', 'SHA256SUMS', 'goteamsctl', 'payload/bin/goteams') + @($whitelist.Keys)
    foreach ($rel in $expected) {
        if (-not (Test-Path -LiteralPath (Join-Path $extracted ($rel -replace '/', '\')))) {
            Fail "extracted payload is missing $rel"
        }
    }
    Test-ExtractedSums $extracted
    $mf = ConvertFrom-Json ([System.IO.File]::ReadAllText((Join-Path $extracted 'manifest.json')))
    if ($mf.name -ne 'goteams-commercial' -or $mf.edition -ne 'commercial' -or $mf.build_environment -ne $Environment -or $mf.arch -ne 'linux-amd64') {
        Fail 'manifest identity mismatch after extraction'
    }
    if ($mf.version -ne $newVersion) { Fail "manifest version mismatch: $($mf.version) != $newVersion" }
    if (@($mf.files).Count -ne $sortedFiles.Count) { Fail 'manifest file count mismatch after extraction' }
    Test-LeakFree $extracted
    Info 'payload extract re-verify passed (versions, hashes, manifest, leak scan)'
    Remove-Item -Recurse -Force $verifyRoot

    # --- self-extracting .run wrapper (embeds the verified payload tar) -------------
    # The gzip tar of the identical staging tree is appended after the sh stub; the stub
    # locates the payload by the #__GOTEAMS_PAYLOAD_BELOW__ marker line, so no byte offset
    # is stored. goteamsctl install re-verifies SHA256SUMS/manifest, the stub check is a
    # convenience pass with identical semantics.
    $sfxStub = @'
#!/bin/sh
# GoTeams commercial self-extracting package (generated by scripts/build-commercial-package.ps1).
# Extracts the embedded package directory, then verifies SHA256SUMS.
# Usage:
#   sh <package>.run [--target DIR] [--no-verify] [--list]
# The extract target defaults to the current directory and an existing package
# directory is never overwritten. goteamsctl install re-verifies the manifest,
# so --no-verify only skips this helper's early sha256 pass.
# The file must be run from disk (sh ./x.run); piping into sh cannot self-locate.
set -eu

name='GOTEAMS_SFX_NAME'
target='.'
verify=1
list=0
while [ $# -gt 0 ]; do
    case "$1" in
        --target)
            if [ $# -lt 2 ]; then echo '[sfx] --target requires a directory argument' >&2; exit 2; fi
            target="$2"; shift 2 ;;
        --no-verify) verify=0; shift ;;
        --list)      list=1; shift ;;
        -h|--help)
            echo "Usage: sh $0 [--target DIR] [--no-verify] [--list]"
            echo "Extracts the '$name' package directory (default: current directory)."
            exit 0 ;;
        *) echo "[sfx] unknown option: $1" >&2; exit 2 ;;
    esac
done

skip=$(awk '/^#__GOTEAMS_PAYLOAD_BELOW__$/{print NR+1; exit}' "$0")
if [ -z "$skip" ]; then
    echo '[sfx] payload marker not found; file is truncated or corrupted' >&2
    exit 1
fi

if [ "$list" = 1 ]; then
    tail -n +"$skip" "$0" | tar -tzf -
    exit 0
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/goteams-sfx.XXXXXX")
trap 'rm -rf "$work"' EXIT
trap 'rm -rf "$work"; exit 1' INT TERM

echo "[sfx] extracting $name ..."
tail -n +"$skip" "$0" | tar -xzf - -C "$work"
if [ ! -d "$work/$name" ]; then
    echo '[sfx] payload does not contain the package directory' >&2
    exit 1
fi

if [ "$verify" = 1 ]; then
    if command -v sha256sum >/dev/null 2>&1; then
        if (cd "$work/$name" && sha256sum -c SHA256SUMS >/dev/null 2>&1); then
            echo '[sfx] sha256 verification passed'
        else
            echo '[sfx] sha256 verification FAILED; payload does not match SHA256SUMS' >&2
            exit 1
        fi
    else
        echo '[sfx] sha256sum not available; skipped early check (goteamsctl install re-verifies manifest)' >&2
    fi
fi

if [ -e "$target/$name" ]; then
    echo "[sfx] target already exists, refusing to overwrite: $target/$name" >&2
    exit 1
fi
mkdir -p "$target"
mv "$work/$name" "$target/$name"

for f in goteamsctl payload/bin/goteams payload/docker/start-web.sh; do
    if [ -f "$target/$name/$f" ]; then
        chmod +x "$target/$name/$f"
    fi
done

echo "[sfx] done: $target/$name"
echo '[sfx] next: cd into that directory and run ./goteamsctl install'
# exit before the payload: the trailing binary must never be parsed as shell input
exit 0
'@
    $sfxStub = $sfxStub.Replace('GOTEAMS_SFX_NAME', $baseName).Replace("`r`n", "`n")
    # the payload-locator marker must terminate its own line; the stub here-string itself has none
    $sfxStub = "$sfxStub`n#__GOTEAMS_PAYLOAD_BELOW__`n"
    $sfxBytes = [System.Text.Encoding]::UTF8.GetBytes($sfxStub)
    # stream assembly: the offline payload can be hundreds of MB, never buffer it fully
    $payloadLen = (Get-Item -LiteralPath $payloadTar).Length
    $runPath = Join-Path $outputDir "$baseName.run"
    $payloadStream = [System.IO.File]::OpenRead($payloadTar)
    $runStream = [System.IO.File]::Create($runPath)
    try {
        $runStream.Write($sfxBytes, 0, $sfxBytes.Length)
        $payloadStream.CopyTo($runStream)
    } finally {
        $runStream.Dispose()
        $payloadStream.Dispose()
    }
    if ((Get-Item -LiteralPath $runPath).Length -ne ($sfxBytes.Length + $payloadLen)) { Fail 'sfx payload length mismatch' }
    $runFs = [System.IO.File]::OpenRead($runPath)
    try {
        $null = $runFs.Seek($sfxBytes.Length, [System.IO.SeekOrigin]::Begin)
        $magic = New-Object byte[] 2
        if ($runFs.Read($magic, 0, 2) -ne 2 -or $magic[0] -ne 0x1F -or $magic[1] -ne 0x8B) {
            Fail 'sfx payload does not start with gzip magic right after stub'
        }
    } finally { $runFs.Dispose() }
    $runHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $runPath).Hash.ToLowerInvariant()
    [System.IO.File]::WriteAllText((Join-Path $outputDir "$baseName.run.sha256"),
        "$runHash  $baseName.run`n", $utf8NoBom)
    Remove-Item -Force $payloadTar
    Remove-Item -Recurse -Force $staging
    Info "self-extracting package: $runPath"
    Info ("package size: {0:N1} MB" -f ((Get-Item -LiteralPath $runPath).Length / 1MB))
    Info "run sha256: $runHash"
    Info "all versions consistent: dir=$baseName manifest=$($mf.version) binaries=$newVersion"
    $script:packageList += $runPath
    Write-Host "DONE $baseName"
}

$lockStream = $null
try {
    foreach ($tool in @('git', 'go', 'node', 'npm')) { Require-Tool $tool }
    if ($needOffline) { Require-Tool 'docker' }
    if (-not (Test-Path $winTar)) { Fail 'windows bsdtar (System32\tar.exe) not found; .run packaging requires it' }
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem

    New-Item -ItemType Directory -Force -Path $versionDir, $outputDir | Out-Null

    # --- Step 1: exclusive version lock, then patch+1 with atomic write-back -------------
    try {
        $lockStream = [System.IO.File]::Open($lockFile,
            [System.IO.FileMode]::OpenOrCreate,
            [System.IO.FileAccess]::Read,
            [System.IO.FileShare]::None)
    } catch {
        Fail "another packaging process is holding $lockFile; only one build may run at a time"
    }
    if (-not (Test-Path $versionFile)) { Fail "missing $versionFile (expected content v0.0.0)" }
    # $baseVersion is only the counter value this build bumps from; it says nothing about which
    # installed versions may upgrade, so it must never reach the manifest.
    $baseVersion = [System.IO.File]::ReadAllText($versionFile).Trim()
    if ($baseVersion -notmatch '^v(\d+)\.(\d+)\.(\d+)$') { Fail "invalid VERSION content '$baseVersion' (expected vX.Y.Z)" }
    $newVersion = 'v{0}.{1}.{2}' -f ([int]$Matches[1]), ([int]$Matches[2]), ([int]$Matches[3] + 1)
    Test-MinUpgradeFloor $MinUpgradeVersion $newVersion
    $tmpVersion = "$versionFile.tmp"
    [System.IO.File]::WriteAllText($tmpVersion, "$newVersion`n", $utf8NoBom)
    try {
        [System.IO.File]::Replace($tmpVersion, $versionFile, $null)
    } catch {
        Move-Item -Force $tmpVersion $versionFile
    }
    $versionOccupied = $true
    Info "version $newVersion occupied (a failed build will NOT roll it back)"

    $packageBase = "goteams-commercial-$Environment-$newVersion"   # variant suffix is appended per package

    # --- Step 2: git commit + clean staging, shared build dir -------------------------
    $commit = (& git -C $repoRoot rev-parse --short=12 HEAD 2>$null)
    if ($LASTEXITCODE -ne 0 -or -not $commit) { Fail 'cannot read git commit' }
    $commit = $commit.Trim()
    Info "git commit: $commit"

    if (Test-Path $stagingRoot) { Remove-Item -Recurse -Force $stagingRoot }
    # One compile per run; every package variant copies the verified binaries from here.
    $sharedDir = Join-Path $stagingRoot 'shared'
    New-Item -ItemType Directory -Force -Path $sharedDir, (Join-Path $sharedDir 'payload\bin') | Out-Null

    # --- Step 3: frontend deps, then build (VITE_BUILD_VERSION injected, no .map) ---------
    # node_modules is not committed; restore the locked tree with npm ci when it is missing
    # or out of sync, otherwise a partial install fails the Vite build on unresolved imports.
    if (Test-FrontendDepsStale) {
        Info 'frontend dependencies missing or out of sync; running npm ci ...'
        Invoke-Native 'npm' @('ci', '--no-audit', '--no-fund') $webDir
    } else {
        Info 'frontend dependencies up to date (skipping npm ci)'
    }

    $distTar = Join-Path $webDir 'dist.tar.gz'
    if (Test-Path $distTar) { Remove-Item -Force $distTar }
    $env:VITE_BUILD_VERSION = $newVersion
    Invoke-Native 'npm' @('run', 'build') $webDir
    if (-not (Test-Path $distTar)) { Fail 'frontend build did not produce web/dist.tar.gz' }
    $tarEntries = & $winTar -tzf $distTar
    if ($LASTEXITCODE -ne 0) { Fail 'cannot list entries of web/dist.tar.gz' }
    $mapEntries = @($tarEntries | Where-Object { $_ -match '\.map$' })
    if ($mapEntries.Count -gt 0) { Fail "frontend dist contains .map files: $($mapEntries -join ', ')" }
    Info 'frontend build ok (no .map entries)'

    # --- Step 4: Go builds (linux amd64, identical ldflags) -------------------------------
    # Both binaries embed the environment-specific INI selected by cn/dev2/dev3/dev6 build
    # tags; goteamsctl needs it so install can print the default admin credential.
    $ldflags = "-X goteams/internal/app/goteams/define.BuildVersion=$newVersion -X goteams/internal/app/goteams/define.BuildCommit=$commit"
    $env:CGO_ENABLED = '0'
    $env:GOOS        = 'linux'
    $env:GOARCH      = 'amd64'
    Invoke-Native 'go' @('build', '-tags', $buildTags, '-trimpath', '-ldflags', $ldflags, '-o', (Join-Path $sharedDir 'payload\bin\goteams'), './cmd/goteams') $repoRoot
    Invoke-Native 'go' @('build', '-tags', $buildTags, '-trimpath', '-ldflags', $ldflags, '-o', (Join-Path $sharedDir 'goteamsctl'), './cmd/goteamsctl') $repoRoot
    foreach ($bin in @((Join-Path $sharedDir 'payload\bin\goteams'), (Join-Path $sharedDir 'goteamsctl'))) {
        $meta = (& go version -m $bin) -join "`n"
        if ($meta -notlike '*GOOS=linux*')    { Fail "binary is not GOOS=linux: $bin" }
        if ($meta -notlike '*GOARCH=amd64*')  { Fail "binary is not GOARCH=amd64: $bin" }
        if ($meta -notlike '*CGO_ENABLED=0*') { Fail "binary is not CGO_ENABLED=0: $bin" }
        # go version -m does not record -ldflags values, so verify the injection by byte scan:
        # the version string can only be present if -ldflags -X embedded it.
        $raw = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes($bin))
        if (-not $raw.Contains('edition=%s commit=%s')) { Fail "binary lacks -version identity code: $bin" }
        if (-not $raw.Contains($newVersion)) { Fail "binary missing version injection: $bin" }
        if (-not $raw.Contains($commit))     { Fail "binary missing commit injection: $bin" }
    }
    # Both binaries must carry the selected embedded INI, not merely any commercial config.
    foreach ($bin in @((Join-Path $sharedDir 'payload\bin\goteams'), (Join-Path $sharedDir 'goteamsctl'))) {
        $raw = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes($bin))
        if (-not $raw.Contains("build_environment = $Environment")) {
            Fail "binary lacks the $Environment embedded config: $bin (tags: $buildTags)"
        }
        if (-not $raw.Contains('admin_password =')) {
            Fail "binary lacks the embedded admin credential: $bin (tags: $buildTags)"
        }
    }
    Info 'go builds ok (version/commit verified in both binaries)'

    # --- Step 4.5: offline image bundle (docker save, offline variant only) --------------
    # Parse the compose image list once; the offline package ships them as one docker-save
    # tar.gz cached under build/commercial/images/ keyed by the image-list hash, so repeated
    # builds reuse the archive unless the image set changes. With -Both the bundle is still
    # produced once and copied into the single offline package.
    $images = @(Select-String -Path $composeFile -Pattern '^\s*image:\s*(\S+)' |
        ForEach-Object { $_.Matches[0].Groups[1].Value })
    if ($images.Count -eq 0) { Fail 'no images found in commercial compose file' }
    $offlineBundleRel = ''
    if ($needOffline) {
        $sortedImages = @($images | Sort-Object -Unique)
        $nameHasher = [System.Security.Cryptography.SHA256]::Create()
        try {
            $nameBytes = [System.Text.Encoding]::UTF8.GetBytes(($sortedImages -join "`n"))
            $bundleTag = (($nameHasher.ComputeHash($nameBytes) | ForEach-Object { $_.ToString('x2') }) -join '').Substring(0, 8)
        } finally { $nameHasher.Dispose() }
        $imagesDir   = Join-Path $versionDir 'images'
        $bundleCache = Join-Path $imagesDir "images-$bundleTag.tar.gz"
        if (Test-Path $bundleCache) {
            Info "offline image bundle cache hit: $bundleCache"
        } else {
            New-Item -ItemType Directory -Force -Path $imagesDir | Out-Null
            foreach ($img in $sortedImages) {
                Info "ensuring local image: $img"
                Invoke-Native 'docker' @('pull', $img) $null
            }
            $tmpTar = Join-Path $imagesDir "images-$bundleTag.tmp.tar"
            if (Test-Path $tmpTar) { Remove-Item -Force $tmpTar }
            Info 'docker save (this can take a few minutes) ...'
            Invoke-Native 'docker' (@('save', '-o', $tmpTar) + $sortedImages) $null
            Info 'compressing image bundle (gzip) ...'
            $bundleTmpGz = "$bundleCache.tmp"
            if (Test-Path $bundleTmpGz) { Remove-Item -Force $bundleTmpGz }
            Compress-GzipFile $tmpTar $bundleTmpGz
            Move-Item -Force $bundleTmpGz $bundleCache
            Remove-Item -Force $tmpTar
        }
        $offlineBundleRel = "build/commercial/images/$(Split-Path -Leaf $bundleCache)"
    }

    # --- Step 5: emit one package per selected variant --------------------------------------
    # VERSION, the frontend dist, both Go binaries and the vendored runtime exist once at this
    # point, so a variant only repeats assembly + verification over its own staging tree. If a
    # later variant fails, the packages already written stay in dist/ and the occupied version
    # is not rolled back — same rule as a single-variant build.
    Save-VendorRuntime
    foreach ($variant in $variants) {
        $isOffline = ($variant -eq 'offline')
        New-CommercialPackage "$packageBase$(if ($isOffline) { '-offline' })" $isOffline
    }
    Info "all packages built: $($packageList -join ', ')"
    exit 0
} catch {
    Write-Host "[package] FAILED: $($_.Exception.Message)" -ForegroundColor Red
    if ($versionOccupied) {
        Write-Host "[package] version $newVersion stays occupied; next run uses a newer patch version" -ForegroundColor Yellow
    }
    exit 1
} finally {
    if ($lockStream) { $lockStream.Dispose() }
    if ($stagingRoot -and (Test-Path $stagingRoot)) { Remove-Item -Recurse -Force $stagingRoot }
}
