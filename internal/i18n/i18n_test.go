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
