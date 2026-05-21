# =============================================================================
# build-podman.ps1
# =============================================================================
# Cross-platform build script for the "orqen" CLI using Podman containers.
#
# Prerequisites:
#   - Podman installed and running
#   - PowerShell 5.1+ or PowerShell 7+
#   - If execution policy blocks the script, run once:
#       Set-ExecutionPolicy RemoteSigned -Scope Process
#
# Usage:
#   .\build-podman.ps1                          # Build all platforms (default)
#   .\build-podman.ps1 -mac                     # macOS only
#   .\build-podman.ps1 -windows -linux          # Windows + Linux
#   .\build-podman.ps1 -mac -version "v1.0.0"   # macOS with custom version tag
#
# Parameters:
#   -version   Version string embedded in the binary via ldflags (default: v1.0.0)
#   -windows   Build Windows amd64 (.exe)
#   -linux     Build Linux amd64 (ELF binary)
#   -mac       Build macOS amd64 + arm64 (.pkg installer packages)
#
# Container images used:
#   golang:1.26          — Go cross-compiler for Linux and Windows targets
#   fleetdm/bomutils     — macOS pkg creation (mkbom, xar); Go 1.25.0 is
#                          installed at runtime from go.dev for darwin builds
#
# Outputs (written to .dist/):
#   orqen.<version>.linux-amd64          — Linux ELF binary
#   orqen.<version>.windows-amd64.exe    — Windows executable
#   orqen.<version>.darwin-amd64.pkg     — macOS Intel installer package (~12MB)
#   orqen.<version>.darwin-arm64.pkg     — macOS Apple Silicon installer (~12MB)
#
# macOS pkg structure (flat format):
#   Distribution   — Installer UI definition (XML)
#   PackageInfo    — Package metadata (identifier, install location, payload size)
#   Payload        — gzip-compressed cpio archive containing /Applications/orqen
#   BOM            — Bill of Materials (file listing + permissions)
#   resources/     — Background image shown in the installer window
#
# macOS build approach:
#   Uses a SINGLE container (fleetdm/bomutils) per architecture to avoid
#   Windows→Linux volume mount visibility issues between separate containers.
#   Go 1.25.0 is installed from the official tarball at runtime.
#   The app.syso Windows resource file is excluded from darwin builds because
#   its AMD64 COFF relocations are incompatible with the darwin/arm64 linker.
#
# The resulting .pkg can be installed on macOS with:
#   sudo installer -pkg orqen.v1.0.0.darwin-arm64.pkg -target /
# which places the binary at /Applications/orqen.
# =============================================================================

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

# ----- Linux (64-bit Intel/AMD) ----------------------------------------------
# Produces a standalone ELF binary. No packaging needed — users place it
# anywhere in their PATH (e.g. /usr/local/bin).
if ($linux) {
    Write-Host "Building Linux (amd64)..." -ForegroundColor Cyan
    podman run --rm -v "${PWD}:/app" -w /app `
        -e GOOS=linux -e GOARCH=amd64 `
        $image go build -ldflags "-X main.Version=$version" `
        -o ".dist/orqen-linux-amd64"
}

# ----- Windows (64-bit Intel/AMD) --------------------------------------------
# Produces a standalone .exe. Same as Linux — no installer, just drop-in.
if ($windows) {
    Write-Host "Building Windows (amd64)..." -ForegroundColor Cyan
    podman run --rm -v "${PWD}:/app" -w /app `
        -e GOOS=windows -e GOARCH=amd64 `
        $image go build -ldflags "-X main.Version=$version" `
        -o ".dist/orqen-windows-amd64.exe"
}

# ----- macOS (Intel + Apple Silicon) -----------------------------------------
# Produces .pkg installer packages for both amd64 (Intel) and arm64 (M1/M2/M3).
#
# Each .pkg uses the macOS "flat package" format (xar archive) containing:
#   Distribution  — Installer.app UI definition (welcome text, choice layout)
#   PackageInfo   — Metadata: package identifier, target install location, payload size
#   Payload       — gzip-compressed cpio archive with the actual binary tree
#   BOM           — Bill of Materials: file permissions, ownership, checksums
#   resources/    — Background image shown in the installer window
#
# Build process per architecture (single container):
#   1. Install Go 1.25.0 + runtime deps (cpio, libcurl) in bomutils container
#   2. Copy source to /tmp excluding app.syso (Windows-only COFF resource)
#   3. Go cross-compile darwin/$ARCH → /tmp/pkg/root/Applications/orqen
#   4. Write Distribution XML via heredoc (installer UI)
#   5. Create Payload from root/ via cpio + gzip
#   6. Create BOM via mkbom
#   7. Compute payload size and file count, write PackageInfo
#   8. Verify all xar inputs, assemble .pkg via xar
#   9. Clean up temporary folders
if ($mac) {

    # Shared resources folder (background image for installer UI)
    New-Item -ItemType Directory -Force -Path ".\.dist\resources" | Out-Null

    # copy icon/background
    if (Test-Path ".\docs\ico.png") {
        Copy-Item ".\docs\ico.png" -Destination ".\.dist\resources\background.png"
    }

    $architectures = @("amd64", "arm64")
    foreach ($arch in $architectures) {

        # per-arch resources folder
        $pkgRoot = ".\.dist\pkg-$arch"
        Remove-Item -Recurse -Force $pkgRoot -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Force -Path "$pkgRoot\resources" | Out-Null

        # copy background image into per-arch resources
        if (Test-Path ".\.dist\resources\background.png") {
            Copy-Item ".\.dist\resources\background.png" -Destination "$pkgRoot\resources\background.png"
        }

        # Step 1 + 2: Build Go binary AND create .pkg in a SINGLE container run.
        # This eliminates cross-container filesystem visibility issues on Windows.
        # We use the bomutils image (has mkbom + xar) and install Go from the
        # official tarball.  The binary is built to /tmp (container tmpfs), then
        # packaged — the only output to the mounted volume is the final .pkg.
        #
        # Placeholders {ARCH} and {VER} are replaced by PowerShell.
        Write-Host "Building macOS ($arch) .pkg..." -ForegroundColor Cyan

        $shScript = @'
set -e
ARCH={ARCH}
VER={VER}

echo "[mac-$ARCH] setting up Go"
export GOROOT=/usr/local/go
export PATH=$GOROOT/bin:$PATH
export GOPATH=/tmp/gopath
export GOCACHE=/tmp/gocache
go version

cd /src
echo "[mac-$ARCH] building Go binary for darwin/$ARCH"

# Copy source to /tmp/src excluding app.syso (Windows AMD64 COFF resource
# that the darwin/arm64 linker cannot process).
rm -rf /tmp/src
mkdir -p /tmp/src
cp -r /src/* /tmp/src/
rm -f /tmp/src/app.syso

mkdir -p /tmp/pkg/root/Applications
cd /tmp/src
GOOS=darwin GOARCH=$ARCH go build -ldflags "-X main.Version=$VER" -o /tmp/pkg/root/Applications/orqen
ls -lh /tmp/pkg/root/Applications/orqen
cp /tmp/pkg/root/Applications/orqen "/dist/orqen-darwin-$ARCH"

echo "[mac-$ARCH] creating Distribution XML"
mkdir -p /tmp/pkg/resources
cp /dist/pkg-$ARCH/resources/background.png /tmp/pkg/resources/background.png 2>/dev/null || true

cat > /tmp/pkg/Distribution <<'DISTEOF'
<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<installer-gui-script minSpecVersion="2">
    <title>orqen</title>
    <options customize="never" require-scripts="false" hostArchitectures="x86_64,arm64"/>
    <background file="background.png" alignment="bottomleft" scaling="none"/>
    <choices-outline>
        <line choice="default"/>
    </choices-outline>
    <choice id="default" title="orqen">
        <pkg-ref id="com.github.nidorx.orqen"/>
    </choice>
    <pkg-ref id="com.github.nidorx.orqen" onConclusion="none">orqen.pkg</pkg-ref>
</installer-gui-script>
DISTEOF

echo "[mac-$ARCH] creating Payload"
(cd /tmp/pkg/root && find . | cpio -o --format odc 2>/dev/null | gzip -c) > /tmp/pkg/Payload

echo "[mac-$ARCH] creating BOM"
mkbom /tmp/pkg/root /tmp/pkg/BOM

PAYLOAD_KB=$(($(wc -c < /tmp/pkg/Payload) / 1024))
NUM_FILES=$(find /tmp/pkg/root -type f | wc -l)
echo "[mac-$ARCH] payload=${PAYLOAD_KB}KB files=$NUM_FILES"

cat > /tmp/pkg/PackageInfo <<PKGEOF
<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<pkg-info formatVersion="2" version="$VER" identifier="com.github.nidorx.orqen" install-location="/Applications">
    <payload installKBytes="$PAYLOAD_KB" numberOfFiles="$NUM_FILES"/>
</pkg-info>
PKGEOF

echo "[mac-$ARCH] verifying xar inputs"
for f in Distribution PackageInfo Payload BOM; do
    test -f "/tmp/pkg/$f" && echo "  OK  $f ($(wc -c < "/tmp/pkg/$f") bytes)" || { echo "  MISSING $f"; exit 1; }
done

echo "[mac-$ARCH] assembling .pkg"
cd /tmp/pkg
xar -cf "/dist/orqen-darwin-$ARCH.pkg" Distribution PackageInfo Payload BOM resources
echo "[mac-$ARCH] done: $(ls -lh "/dist/orqen-darwin-$ARCH.pkg" | awk '{print $5}')"
'@

        $shScript = $shScript -replace '\{ARCH\}', $arch -replace '\{VER\}', $version

        # Write the shell script with Unix line endings
        $scriptPath = Join-Path $PSScriptRoot ".dist\_pkg_build.sh"
        $shScriptLF = $shScript -replace "`r`n", "`n"
        [System.IO.File]::WriteAllText($scriptPath, $shScriptLF + "`n", [System.Text.Encoding]::ASCII)

        # Run everything in bomutils container (has mkbom + xar).
        # Mount .dist as /dist (background image input, .pkg output)
        # Mount project root as /src (Go source code for build).
        # Install Go from the official tarball, then run the build script.
        $outerScript = @'
set -e

# Install runtime dependencies (bomutils image is minimal — missing curl lib, cpio)
if ! curl --version >/dev/null 2>&1 || ! cpio --version >/dev/null 2>&1; then
    apt-get update -qq >/dev/null 2>&1
    apt-get install -y -qq libcurl4t64 wget cpio >/dev/null 2>&1
fi

if [ ! -d /usr/local/go/bin ]; then
    echo "[setup] installing Go 1.25.0"
    curl -sL https://go.dev/dl/go1.25.0.linux-amd64.tar.gz | tar -C /usr/local -xzf -
fi
sh /dist/_pkg_build.sh
'@
        $outerScriptLF = $outerScript -replace "`r`n", "`n"
        $outerPath = Join-Path $PSScriptRoot ".dist\_setup.sh"
        [System.IO.File]::WriteAllText($outerPath, $outerScriptLF + "`n", [System.Text.Encoding]::ASCII)

        podman run --rm `
            -v "${PWD}:/src" `
            -v "${PWD}/.dist:/dist" `
            -w /src `
            docker.io/fleetdm/bomutils:latest `
            /bin/sh /dist/_setup.sh

        Remove-Item $scriptPath -ErrorAction SilentlyContinue
        Remove-Item $outerPath -ErrorAction SilentlyContinue
        Remove-Item -Recurse -Force $pkgRoot -ErrorAction SilentlyContinue
    }
}
