# PowerShell build script using podman
#
# Set-ExecutionPolicy RemoteSigned -Scope Proces
# .\build-podman.ps1 -windows -linux -mac -version "v1.0.0"

param (
    [string]$version = "v1.0.0",
    [switch]$windows,
    [switch]$linux,
    [switch]$mac
)

if (-not ($windows -or $linux -or $mac)) {
    $windows = $true
    $linux = $true
    $mac = $true
}

$image = "docker.io/library/golang:1.26"

# Linux (64-bit Intel/AMD)
if ($linux) {
    Write-Host "Building Linux (amd64)..." -ForegroundColor Cyan
    podman run --rm -v "${PWD}:/app" -w /app `
        -e GOOS=linux -e GOARCH=amd64 `
        $image go build -ldflags "-X main.Version=$version" `
        -o ".dist/orqen.$version.linux-amd64"
}

# Windows (64-bit Intel/AMD)orqen-windows-amd64.exe
if ($windows) {
    Write-Host "Building Windows (amd64)..." -ForegroundColor Cyan
    podman run --rm -v "${PWD}:/app" -w /app `
        -e GOOS=windows -e GOARCH=amd64 `
        $image go build -ldflags "-X main.Version=$version" `
        -o ".dist/orqen.$version.windows-amd64.exe"
}

# macOS (Apple Silicon M1/M2/M3)
# podman run --rm -v "${PWD}:/app" -w /app -e GOOS=darwin -e GOARCH=arm64 docker.io/library/golang:1.26 go build -o bin/orqen-mac-arm64
if ($mac) {
    
    # temp folder
    New-Item -ItemType Directory -Force -Path ".\.dist\root\Applications" | Out-Null
    New-Item -ItemType Directory -Force -Path ".\.dist\resources" | Out-Null

    # copy icon
    if (Test-Path ".\docs\ico.png") {
        Copy-Item ".\docs\ico.png" -Destination ".\.dist\resources\background.png"
    }

    $architectures = @("amd64", "arm64")
    foreach ($arch in $architectures) {

        # 5. Distribution.xml
        $XmlContent = @"
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
    <title>orqen</title>
    <options customize="never" require-scripts="false"/>
    <background file="background.png" alignment="bottomleft" scaling="none"/>
    <choices-outline>
        <line choice="default"/>
    </choices-outline>
    <choice id="default" title="orqen">
        <pkg-ref id="com.github.nidorx.orqen"/>
    </choice>
    <pkg-ref id="com.github.nidorx.orqen" version="$version" onConclusion="none">orqen.$version.darwin-$arch.pkg</pkg-ref>
</installer-gui-script>
"@    
        Set-Content -Path ".\.dist\resources\Distribution.xml" -Value $XmlContent
    
        # macOS (Apple Silicon M1/M2/M3)
        Write-Host "Building macOS ($arch)..." -ForegroundColor Cyan
        podman run --rm -v "${PWD}:/app" -w /app `
            -e GOOS=darwin -e GOARCH=$arch `
            $image go build -ldflags "-X main.Version=$version" `
            -o ".dist/root/Applications/orqen"

        # .pkg
        Write-Host "Building macOS ($arch) .pkg..." -ForegroundColor Green
        podman run --rm -v "${PWD}/.dist:/dist" -w /dist docker.io/fleetdm/bomutils:latest /bin/sh -c "     
            chmod -R 755 root/Applications   
            mkbom root root.bom  
            xar -cf orqen.$version.darwin-$arch.pkg root.bom resources/Distribution.xml resources/background.png     
            rm root.bom
        "
    }
}

# orqen.v1.0.0.darwin-arm64.pkg
# orqen.1.0.0.darwin-arm64
# orqen.1.0.0.darwin-amd64