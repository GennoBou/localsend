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

func TestGetLanguage(t *testing.T) {
	tests := []struct {
		name         string
		setLang      string
		expectedLang string
	}{
		{
			name:         "Set to english",
			setLang:      "en",
			expectedLang: "en",
		},
		{
			name:         "Set to japanese",
			setLang:      "ja",
			expectedLang: "ja",
		},
		{
			name:         "Set to uppercase JA",
			setLang:      "JA",
			expectedLang: "ja",
		},
		{
			name:         "Set to uppercase EN",
			setLang:      "EN",
			expectedLang: "en",
		},
		{
			name:         "Invalid language should not change current language",
			setLang:      "fr",
			expectedLang: "en", // remains "en" from previous step
		},
		{
			name:         "Empty language string should not change current language",
			setLang:      "",
			expectedLang: "en", // remains "en"
		},
	}

	// Reset state to a known baseline
	SetLanguage("en")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLanguage(tt.setLang)
			if got := GetLanguage(); got != tt.expectedLang {
				t.Errorf("GetLanguage() = %v, want %v", got, tt.expectedLang)
			}
		})
	}
}
