package protocol

import (
	"testing"
)

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal simple filename",
			input:    "photo.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "Filename with spaces",
			input:    "my document 2026.pdf",
			expected: "my document 2026.pdf",
		},
		{
			name:     "Japanese filename",
			input:    "ドキュメント_写真.png",
			expected: "ドキュメント_写真.png",
		},
		{
			name:     "Unix-style path traversal",
			input:    "../../../../etc/passwd",
			expected: "passwd",
		},
		{
			name:     "Windows-style path traversal",
			input:    `..\..\..\Windows\System32\cmd.exe`,
			expected: "cmd.exe",
		},
		{
			name:     "Double dot only",
			input:    "..",
			expected: "file",
		},
		{
			name:     "Single dot only",
			input:    ".",
			expected: "file",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "file",
		},
		{
			name:     "Slash only",
			input:    "/",
			expected: "file",
		},
		{
			name:     "Backslash only",
			input:    `\`,
			expected: "file",
		},
		{
			name:     "Windows reserved device name: CON",
			input:    "CON",
			expected: "_CON",
		},
		{
			name:     "Windows reserved device name with extension: nul.txt",
			input:    "nul.txt",
			expected: "_nul.txt",
		},
		{
			name:     "Windows reserved device name case-insensitive: AuX.tar.gz",
			input:    "AuX.tar.gz",
			expected: "_AuX.tar.gz",
		},
		{
			name:     "Windows reserved port: COM1",
			input:    "COM1",
			expected: "_COM1",
		},
		{
			name:     "NTFS alternate data stream",
			input:    "secret.txt:hidden.exe",
			expected: "secret.txt_hidden.exe",
		},
		{
			name:     "Forbidden characters: < > : \" / \\ | ? *",
			input:    `hello<world>?*"|.txt`,
			expected: "hello_world_____.txt",
		},
		{
			name:     "Trailing dots and spaces",
			input:    "important_report.pdf. . ",
			expected: "important_report.pdf",
		},
		{
			name:     "Only dots and spaces",
			input:    "   ...   ",
			expected: "file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := SanitizeFileName(tc.input)
			if actual != tc.expected {
				t.Errorf("SanitizeFileName(%q) = %q, expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}
