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
	msg = T("sending_files", map[string]interface{}{"Alias": "Nice Orange"})
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

	msg = T("sending_files", map[string]interface{}{"Alias": "Secret Banana"})
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

func TestDetectLanguage(t *testing.T) {
	detectLanguage()
	lang := GetLanguage()
	t.Logf("Detected language: %s", lang)
	if lang != "ja" && lang != "en" {
		t.Errorf("Expected 'ja' or 'en', got '%s'", lang)
	}
}

func TestSetLanguage(t *testing.T) {
	tests := []struct {
		name          string
		initialLang   string
		inputLang     string
		expectedResult string
	}{
		{
			name:           "set to english lower",
			initialLang:    "ja",
			inputLang:      "en",
			expectedResult: "en",
		},
		{
			name:           "set to japanese lower",
			initialLang:    "en",
			inputLang:      "ja",
			expectedResult: "ja",
		},
		{
			name:           "set to english upper",
			initialLang:    "ja",
			inputLang:      "EN",
			expectedResult: "en",
		},
		{
			name:           "set to japanese mixed case",
			initialLang:    "en",
			inputLang:      "Ja",
			expectedResult: "ja",
		},
		{
			name:           "invalid language preserves current language",
			initialLang:    "en",
			inputLang:      "fr",
			expectedResult: "en",
		},
		{
			name:           "empty language string preserves current language",
			initialLang:    "ja",
			inputLang:      "",
			expectedResult: "ja",
		},
		{
			name:           "unsupported language preserves current language",
			initialLang:    "en",
			inputLang:      "invalid_lang",
			expectedResult: "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLanguage(tt.initialLang)
			SetLanguage(tt.inputLang)

			if got := GetLanguage(); got != tt.expectedResult {
				t.Errorf("SetLanguage(%q) with initial %q = %q; want %q", tt.inputLang, tt.initialLang, got, tt.expectedResult)
			}
		})
	}
}
