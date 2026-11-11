package protocol

import (
	"path"
	"regexp"
	"strings"
)

// windowsReservedNames lists reserved device names on Windows operating systems.
var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// invalidCharsRegex matches control characters and characters prohibited across filesystems.
var invalidCharsRegex = regexp.MustCompile(`[\x00-\x1f\x7f<>:"/\\|?*]`)

// SanitizeFileName cleans an untrusted filename received from peers or network requests.
// It guards against path traversal (CWE-22), Windows reserved device names,
// NTFS alternate data streams, and forbidden filesystem characters.
func SanitizeFileName(filename string) string {
	// 1. Normalize all backslashes to forward slashes for cross-platform path consistency
	normalized := strings.ReplaceAll(filename, "\\", "/")

	// 2. Extract the base filename using path.Base (which uses '/' on all operating systems)
	base := path.Base(normalized)

	// 3. Fallback if the path yields dot, double-dot, slash, or empty string
	if base == "." || base == ".." || base == "/" || base == "" {
		return "file"
	}

	// 4. Replace invalid or dangerous characters with underscores
	base = invalidCharsRegex.ReplaceAllString(base, "_")

	// 5. Trim trailing dots and spaces (Windows restriction)
	base = strings.TrimRight(base, ". ")
	if base == "" {
		return "file"
	}

	// 6. Prevent Windows reserved device names (e.g., CON, PRN, AUX, NUL, COM1-9, LPT1-9)
	nameWithoutExt := base
	ext := ""
	if dotIdx := strings.Index(base, "."); dotIdx != -1 {
		nameWithoutExt = base[:dotIdx]
		ext = base[dotIdx:]
	}

	if windowsReservedNames[strings.ToUpper(nameWithoutExt)] {
		base = "_" + nameWithoutExt + ext
	}

	return base
}
