//go:build windows

package cli

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// detectLocaleFromWindowsRegistry queries the Windows registry to determine
// the user's preferred locale. It reads from:
//
//	HKCU\Control Panel\International\User Profile\SystemDefaultLocaleName
//
// Returns the locale string (e.g., "pt-BR", "en-US") or empty string if
// the registry key cannot be read.
func detectLocaleFromWindowsRegistry() string {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Control Panel\International\User Profile`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return ""
	}
	defer k.Close()

	val, _, err := k.GetStringValue("SystemDefaultLocaleName")
	if err != nil {
		return ""
	}

	// Windows returns something like "pt-BR" or "en-US" — normalize
	return strings.TrimSpace(val)
}

// isWindows returns true on Windows (satisfies build tag).
func isWindows() bool {
	return true
}
