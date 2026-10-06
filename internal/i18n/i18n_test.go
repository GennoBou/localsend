package i18n

import (
	"testing"
)

func TestI18nTranslations(t *testing.T) {
	// Test when set to English
	SetLanguage("en")

	if GetLanguage() != "en" {
		t.Errorf("Expected language 'en', got '%s'", GetLanguage())
	}

	msg := T("scanning", nil)
	expected := "Scanning for LocalSend devices..."
	if msg != expected {
		t.Errorf("Expected '%s', got '%s'", expected, msg)
	}

	// Test placeholder replacement
	msg = T("sending_files", map[string]any{"Alias": "Nice Orange"})
	expected = "Sending files to Nice Orange..."
	if msg != expected {
		t.Errorf("Expected '%s', got '%s'", expected, msg)
	}

	// Test when set to Japanese
	SetLanguage("ja")

	if GetLanguage() != "ja" {
		t.Errorf("Expected language 'ja', got '%s'", GetLanguage())
	}

	msg = T("scanning", nil)
	expected = "LocalSend デバイスをスキャン中..."
	if msg != expected {
		t.Errorf("Expected '%s', got '%s'", expected, msg)
	}

	msg = T("sending_files", map[string]any{"Alias": "Secret Banana"})
	expected = "Secret Banana へファイルを送信中..."
	if msg != expected {
		t.Errorf("Expected '%s', got '%s'", expected, msg)
	}
}

func TestI18nFallback(t *testing.T) {
	SetLanguage("ja")

	// If the key does not exist, the key name itself is returned
	msg := T("non_existent_key", nil)
	if msg != "non_existent_key" {
		t.Errorf("Expected fallback to key name, got '%s'", msg)
	}
}

func TestSetLanguage(t *testing.T) {
	origLang := GetLanguage()
	t.Cleanup(func() {
		SetLanguage(origLang)
	})

	SetLanguage("JA")
	if GetLanguage() != "ja" {
		t.Errorf("Expected 'ja' after SetLanguage('JA'), got '%s'", GetLanguage())
	}

	SetLanguage("EN")
	if GetLanguage() != "en" {
		t.Errorf("Expected 'en' after SetLanguage('EN'), got '%s'", GetLanguage())
	}

	// Invalid language should be ignored and leave language unchanged
	SetLanguage("fr")
	if GetLanguage() != "en" {
		t.Errorf("Expected 'en' (unchanged) after SetLanguage('fr'), got '%s'", GetLanguage())
	}
}

func TestDetectLanguage(t *testing.T) {
	origLang := GetLanguage()
	t.Cleanup(func() {
		currentLang = origLang
	})

	t.Run("Default detection", func(t *testing.T) {
		detectLanguage()
		lang := GetLanguage()
		t.Logf("Detected language: %s", lang)
		if lang != "ja" && lang != "en" {
			t.Errorf("Expected 'ja' or 'en', got '%s'", lang)
		}
	})

	t.Run("Env var LC_ALL=ja", func(t *testing.T) {
		t.Setenv("LC_ALL", "ja_JP.UTF-8")
		detectLanguage()
		if GetLanguage() != "ja" {
			t.Errorf("Expected 'ja', got '%s'", GetLanguage())
		}
	})

	t.Run("Env var LANG=en", func(t *testing.T) {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "en_US.UTF-8")
		detectLanguage()
		if GetLanguage() != "en" {
			t.Errorf("Expected 'en', got '%s'", GetLanguage())
		}
	})
}

func TestT(t *testing.T) {
	// Backup original state
	origLang := currentLang
	origEnTranslations := make(map[string]string)
	for k, v := range translations["en"] {
		origEnTranslations[k] = v
	}
	origJaTranslations := make(map[string]string)
	for k, v := range translations["ja"] {
		origJaTranslations[k] = v
	}

	t.Cleanup(func() {
		currentLang = origLang
		translations["en"] = origEnTranslations
		translations["ja"] = origJaTranslations
	})

	// Add test keys to translations for edge cases
	translations["en"]["en_only_key"] = "Only in English"
	translations["en"]["invalid_template_key"] = "Hello {{.Unclosed"
	translations["en"]["exec_error_key"] = "Hello {{.NonExistentField.SubField}}"

	type testCase struct {
		name     string
		lang     string
		key      string
		args     any
		expected string
	}

	tests := []testCase{
		{
			name:     "English translation without args",
			lang:     "en",
			key:      "scanning",
			args:     nil,
			expected: "Scanning for LocalSend devices...",
		},
		{
			name:     "Japanese translation without args",
			lang:     "ja",
			key:      "scanning",
			args:     nil,
			expected: "LocalSend デバイスをスキャン中...",
		},
		{
			name:     "English translation with map args",
			lang:     "en",
			key:      "sending_files",
			args:     map[string]any{"Alias": "Alice"},
			expected: "Sending files to Alice...",
		},
		{
			name:     "English translation with struct args",
			lang:     "en",
			key:      "sending_files",
			args:     struct{ Alias string }{Alias: "Bob"},
			expected: "Sending files to Bob...",
		},
		{
			name:     "Fallback to English when key missing in Japanese",
			lang:     "ja",
			key:      "en_only_key",
			args:     nil,
			expected: "Only in English",
		},
		{
			name:     "Fallback to key string when missing in all languages",
			lang:     "en",
			key:      "non_existent_key_xyz",
			args:     nil,
			expected: "non_existent_key_xyz",
		},
		{
			name:     "Fallback to English translation map when currentLang is unknown",
			lang:     "fr",
			key:      "scanning",
			args:     nil,
			expected: "Scanning for LocalSend devices...",
		},
		{
			name:     "Template parse error returns raw msg",
			lang:     "en",
			key:      "invalid_template_key",
			args:     map[string]any{"Name": "World"},
			expected: "Hello {{.Unclosed",
		},
		{
			name:     "Template execute error returns raw msg",
			lang:     "en",
			key:      "exec_error_key",
			args:     map[string]any{"NonExistentField": "not_a_struct"},
			expected: "Hello {{.NonExistentField.SubField}}",
		},
		{
			name:     "Empty key returns empty key",
			lang:     "en",
			key:      "",
			args:     nil,
			expected: "",
		},
		{
			name:     "Typed nil pointer as args gracefully handles execution error",
			lang:     "en",
			key:      "sending_files",
			args:     (*struct{ Alias string })(nil),
			expected: "Sending files to {{.Alias}}...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentLang = tt.lang
			result := T(tt.key, tt.args)
			if result != tt.expected {
				t.Errorf("T(%q, %v) = %q; want %q", tt.key, tt.args, result, tt.expected)
			}
		})
	}
}
