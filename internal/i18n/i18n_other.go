//go:build !windows

package i18n

// sysLocale is a placeholder function for platforms other than Windows.
// On non-Windows platforms, environment variables are prioritized, so this returns an empty string.
func sysLocale() string {
	return ""
}
