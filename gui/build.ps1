param(
    [ValidatePattern('^v\d+\.\d+\.\d+([-.][a-zA-Z0-9.]+)?$')]
    [string]$GuiVersion = 'v0.6.3'
)

$ErrorActionPreference = 'Stop'
$env:CGO_ENABLED = '0'
$root = [IO.Path]::GetFullPath($PSScriptRoot)
$build = Join-Path $root 'build'
$name = "scrcpy-lan-$GuiVersion-windows-x64"
$package = Join-Path $build $name
$runtimeVersion = 'v5.0'
$runtimeHash = '44c10d9e82f20ea67227d14d37bf9fbe3603117c5736df3f514544a02ba20a73'
$runtimeURL = "https://github.com/Genymobile/scrcpy/releases/download/$runtimeVersion/scrcpy-win64-$runtimeVersion.zip"

function Invoke-Go {
    param([string[]]$NativeArgs)
    & go @NativeArgs
    if ($LASTEXITCODE -ne 0) { throw "go $($NativeArgs[0]) failed with exit code $LASTEXITCODE" }
}

Push-Location -LiteralPath $root
try {
    Invoke-Go -NativeArgs @('run', 'github.com/akavel/rsrc@v0.10.2', '-arch', 'amd64', '-ico', 'assets/icon.ico', '-o', 'icon_windows_amd64.syso')
    Invoke-Go -NativeArgs @('test', '-buildvcs=false', './...')
    Invoke-Go -NativeArgs @('vet', '-buildvcs=false', './...')
    if (Test-Path -LiteralPath $package) {
        $resolved = [IO.Path]::GetFullPath($package)
        if (-not $resolved.StartsWith($build + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
            throw 'Package cleanup path is outside the build directory'
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
    New-Item -ItemType Directory -Path $package -Force | Out-Null
    $output = Join-Path $package 'scrcpy-lan.exe'
    Invoke-Go -NativeArgs @('build', '-trimpath', '-buildvcs=false', '-tags', 'mygo_noinspector', '-ldflags', "-s -w -H=windowsgui -X main.version=$GuiVersion", '-o', $output, '.')
    $archive = Join-Path $build "scrcpy-win64-$runtimeVersion.zip"
    if (-not (Test-Path -LiteralPath $archive)) {
        Invoke-WebRequest -Uri $runtimeURL -OutFile $archive
    }
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $runtimeHash) {
        throw 'Official scrcpy runtime checksum mismatch'
    }
    $unpacked = Join-Path $build 'upstream'
    Expand-Archive -LiteralPath $archive -DestinationPath $unpacked -Force
    Copy-Item -LiteralPath (Join-Path $unpacked "scrcpy-win64-$runtimeVersion") -Destination (Join-Path $package 'runtime') -Recurse
    Copy-Item -LiteralPath (Join-Path $root 'README.md') -Destination (Join-Path $package '使用说明.md')
    Copy-Item -LiteralPath (Join-Path $root '../LICENSE') -Destination (Join-Path $package 'LICENSE')
    Copy-Item -LiteralPath (Join-Path $root 'THIRD_PARTY_NOTICES.md') -Destination $package
    $licenses = Join-Path $package 'licenses'
    New-Item -ItemType Directory -Path $licenses -Force | Out-Null
    $modules = & go list -m -f '{{.Path}}|{{.Version}}|{{.Dir}}' all
    if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate module licenses' }
    foreach ($module in $modules) {
        $parts = $module.Split('|')
        if ($parts[1] -eq '' -or $parts[2] -eq '') { continue }
        $moduleName = $parts[0].Replace('/', '_')
        foreach ($file in @('LICENSE', 'LICENSE.md', 'LICENSE.txt', 'NOTICE')) {
            $source = Join-Path $parts[2] $file
            if (Test-Path -LiteralPath $source) {
                Copy-Item -LiteralPath $source -Destination (Join-Path $licenses "$moduleName-$file")
            }
        }
    }
    $manifest = [ordered]@{ gui = $GuiVersion; scrcpy = $runtimeVersion; runtime_sha256 = $runtimeHash; runtime_source = $runtimeURL }
    $manifest | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $package 'versions.json') -Encoding utf8
    $zip = Join-Path $build "$name.zip"
    Compress-Archive -LiteralPath $package -DestinationPath $zip -Force
    $hash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $name.zip" | Set-Content -LiteralPath (Join-Path $build "$name.sha256") -Encoding ascii
    Write-Output $zip
} finally {
    Pop-Location
}
