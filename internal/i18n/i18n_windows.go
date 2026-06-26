//go:build windows

package i18n

import (
	"golang.org/x/sys/windows"
)

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")
)

// sysLocale retrieves and checks the UI display language of Windows.
// It returns "ja" if it is Japanese, otherwise "en".
func sysLocale() string {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	if r == 0 {
		return ""
	}
	// The lower 10 bits represent the primary language ID
	// 0x11 is LANG_JAPANESE
	if (r & 0x03FF) == 0x11 {
		return "ja"
	}
	return "en"
}
