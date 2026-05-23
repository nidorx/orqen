// =============================================================================
// z_build.go — orqen cross-platform build via container runtimes
// =============================================================================
//
// Cross-platform build script for the "orqen" CLI. Runs inside a container
// runtime to avoid requiring a local Go toolchain for every target platform.
//
// Prerequisites:
//   - Docker or Podman installed and running (auto-detected in that order)
//
// Usage:
//   go run ./do -build                           # Build all platforms (default)
//   go run ./do -build -mac                      # macOS only
//   go run ./do -build -windows -linux           # Windows + Linux
//   go run ./do -build -mac -version "v1.0.0"    # macOS with custom version tag
//
// Build flags:
//   -version   Version string embedded in the binary via ldflags (default: v1.0.0)
//   -windows   Build Windows amd64 (.exe)
//   -linux     Build Linux amd64 (ELF binary)
//   -mac       Build macOS amd64 + arm64 (.pkg installer packages)
//
// Container images used:
//   golang:1.26          — Go cross-compiler for Linux and Windows targets
//   fleetdm/bomutils     — macOS pkg creation (mkbom, xar); Go 1.25.0 is
//                          installed at runtime from go.dev for darwin builds
//
// Outputs (written to .dist/ at the project root):
//   orqen-linux-amd64            — Linux ELF binary
//   orqen-windows-amd64.exe      — Windows executable
//   orqen-darwin-amd64           — macOS Intel binary (debug copy, .pkg is primary)
//   orqen-darwin-arm64           — macOS Apple Silicon binary (debug copy, .pkg is primary)
//   orqen-darwin-amd64.pkg       — macOS Intel installer package (~12MB)
//   orqen-darwin-arm64.pkg       — macOS Apple Silicon installer (~12MB)
//
// macOS pkg structure (flat format):
//   Distribution   — Installer UI definition (XML)
//   PackageInfo    — Package metadata (identifier, install location, payload size)
//   Payload        — gzip-compressed cpio archive containing /Applications/orqen
//   BOM            — Bill of Materials (file listing + permissions)
//   resources/     — Background image shown in the installer window
//
// macOS build approach:
//   Uses a SINGLE container (fleetdm/bomutils) per architecture to avoid
//   Windows→Linux volume mount visibility issues between separate containers.
//   Go 1.25.0 is installed from the official tarball at runtime.
//   The app.syso Windows resource file is excluded from darwin builds because
//   its AMD64 COFF relocations are incompatible with the darwin/arm64 linker.
//
// The resulting .pkg can be installed on macOS with:
//   sudo installer -pkg orqen.v1.0.0.darwin-arm64.pkg -target /
// which places the binary at /Applications/orqen.
// =============================================================================

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- build flags -------------------------------------------------------------

var (
	buildVersion string
	buildWindows bool
	buildLinux   bool
	buildMac     bool
)

// --- entry -------------------------------------------------------------------

func runBuild() error {
	// Find the project root (where go.mod lives).
	// Works whether invoked from do/ (via `go run .`) or from the project root.
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	var flagSet = flag.NewFlagSet("do build", flag.ExitOnError)
	flagSet.StringVar(&buildVersion, "version", "v1.0.0", "version string embedded in binaries via ldflags")
	flagSet.BoolVar(&buildWindows, "windows", false, "build Windows amd64 (.exe)")
	flagSet.BoolVar(&buildLinux, "linux", false, "build Linux amd64 (ELF binary)")
	flagSet.BoolVar(&buildMac, "mac", false, "build macOS amd64 + arm64 (.pkg installer)")

	flagSet.Parse(subArgsFor("build"))

	// default: build all platforms when none specified
	if !buildWindows && !buildLinux && !buildMac {
		buildWindows = true
		buildLinux = true
		buildMac = true
	}

	distDir := filepath.Join(projectRoot, ".dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return fmt.Errorf("create .dist: %w", err)
	}

	if buildLinux {
		if err := buildLinuxAmd64(projectRoot, distDir); err != nil {
			return fmt.Errorf("linux build: %w", err)
		}
	}

	if buildWindows {
		if err := buildWindowsAmd64(projectRoot, distDir); err != nil {
			return fmt.Errorf("windows build: %w", err)
		}
	}

	if buildMac {
		if err := buildMacPackages(projectRoot, distDir); err != nil {
			return fmt.Errorf("macOS build: %w", err)
		}
	}

	return nil
}

// ----- Linux -----------------------------------------------------------------

func buildLinuxAmd64(projectRoot, distDir string) error {
	fmt.Println("Building Linux (amd64)...")
	return RunContainer(
		"docker.io/library/golang:1.26",
		[]string{},
		map[string]string{
			"GOOS":   "linux",
			"GOARCH": "amd64",
		},
		"go", "build", "-ldflags", ldflags(buildVersion), "-o", "/dist/orqen-linux-amd64",
	)
}

// ----- Windows ---------------------------------------------------------------

func buildWindowsAmd64(projectRoot, distDir string) error {
	fmt.Println("Building Windows (amd64)...")
	return RunContainer(
		"docker.io/library/golang:1.26",
		[]string{},
		map[string]string{
			"GOOS":   "windows",
			"GOARCH": "amd64",
		},
		"go", "build", "-ldflags", ldflags(buildVersion), "-o", "/dist/orqen-windows-amd64.exe",
	)
}

// ----- macOS (Intel + Apple Silicon) -----------------------------------------
// Produces .pkg installer packages for both amd64 (Intel) and arm64 (M1/M2/M3).
//
// Each .pkg uses the macOS "flat package" format (xar archive) containing:
//
//	Distribution  — Installer.app UI definition (welcome text, choice layout)
//	PackageInfo   — Metadata: package identifier, target install location, payload size
//	Payload       — gzip-compressed cpio archive with the actual binary tree
//	BOM           — Bill of Materials: file permissions, ownership, checksums
//	resources/    — Background image shown in the installer window
//
// Build process per architecture (single container):
//  1. Install Go 1.25.0 + runtime deps (cpio, libcurl) in bomutils container
//  2. Copy source to /tmp excluding app.syso (Windows-only COFF resource)
//  3. Go cross-compile darwin/$ARCH → /tmp/pkg/root/Applications/orqen
//  4. Write Distribution XML via heredoc (installer UI)
//  5. Create Payload from root/ via cpio + gzip
//  6. Create BOM via mkbom
//  7. Compute payload size and file count, write PackageInfo
//  8. Verify all xar inputs, assemble .pkg via xar
//  9. Clean up temporary folders
func buildMacPackages(projectRoot, distDir string) error {
	// Shared resources folder (background image for installer UI)
	resDir := filepath.Join(distDir, "resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		return err
	}

	// copy icon/background
	icoPath := filepath.Join(projectRoot, "docs", "ico.png")
	if _, err := os.Stat(icoPath); err == nil {
		Must(CopyFile(icoPath, filepath.Join(resDir, "background.png")))
	}

	for _, arch := range []string{"amd64", "arm64"} {
		buildMacArch(distDir, arch)
	}
	return nil
}

func buildMacArch(distDir, arch string) {
	fmt.Printf("Building macOS (%s) .pkg...\n", arch)

	// per-arch resources folder
	pkgResDir := filepath.Join(distDir, fmt.Sprintf("pkg-%s", arch), "resources")
	os.MkdirAll(pkgResDir, 0o755)

	// copy background image into per-arch resources
	bgSrc := filepath.Join(distDir, "resources", "background.png")
	if _, err := os.Stat(bgSrc); err == nil {
		Must(CopyFile(bgSrc, filepath.Join(pkgResDir, "background.png")))
	}

	// Write the inner build script
	// Step 1 + 2: Build Go binary AND create .pkg in a SINGLE container run.
	// This eliminates cross-container filesystem visibility issues on Windows.
	// We use the bomutils image (has mkbom + xar) and install Go from the
	// official tarball.  The binary is built to /tmp (container tmpfs), then
	// packaged — the only output to the mounted volume is the final .pkg.
	//
	// Placeholders {ARCH} and {VER} are replaced by PowerShell.
	shScript := strings.ReplaceAll(macBuildScript, "{ARCH}", arch)
	shScript = strings.ReplaceAll(shScript, "{VER}", buildVersion)
	scriptPath := filepath.Join(distDir, "_pkg_build.sh")
	os.WriteFile(scriptPath, []byte(shScript), 0o644)

	// Write the outer setup script
	outerPath := filepath.Join(distDir, "_setup.sh")
	os.WriteFile(outerPath, []byte(macSetupScript), 0o644)

	defer func() {
		os.Remove(scriptPath)
		os.Remove(outerPath)
		os.RemoveAll(filepath.Join(distDir, "pkg-"+arch))
	}()

	Must(RunContainer(
		"docker.io/fleetdm/bomutils:latest",
		[]string{},
		map[string]string{},
		"/bin/sh", "/dist/_setup.sh",
	))
}

// ----- helpers ---------------------------------------------------------------

func ldflags(version string) string {
	return fmt.Sprintf("-X main.Version=%s", version)
}

// ----- embedded shell scripts for macOS pkg ----------------------------------

const macBuildScript = `set -e
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
`

const macSetupScript = `set -e

if ! curl --version >/dev/null 2>&1 || ! cpio --version >/dev/null 2>&1; then
    apt-get update -qq >/dev/null 2>&1
    apt-get install -y -qq libcurl4t64 wget cpio >/dev/null 2>&1
fi

if [ ! -d /usr/local/go/bin ]; then
    echo "[setup] installing Go 1.25.0"
    curl -sL https://go.dev/dl/go1.25.0.linux-amd64.tar.gz | tar -C /usr/local -xzf -
fi
sh /dist/_pkg_build.sh
`
