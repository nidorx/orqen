//go:build !windows

package cli

// detectLocaleFromWindowsRegistry is a no-op on non-Windows platforms.
// Locale detection on these platforms relies solely on environment variables
// (LANG, LC_ALL), which are handled in DetectLocale.
func detectLocaleFromWindowsRegistry() string {
	return ""
}

// isWindows returns false on non-Windows platforms.
func isWindows() bool {
	return false
}
