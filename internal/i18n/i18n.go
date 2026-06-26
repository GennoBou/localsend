package i18n

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"os"
	"strings"
)

//go:embed en.json ja.json
var localesFS embed.FS

var currentLang = "en"
var translations = make(map[string]map[string]string)

func init() {
	// Load embedded translation files
	loadTranslations("en")
	loadTranslations("ja")

	// Detect default language
	detectLanguage()
}

func loadTranslations(lang string) {
	data, err := localesFS.ReadFile(lang + ".json")
	if err != nil {
		return
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		translations[lang] = m
	}
}

func detectLanguage() {
	// 1. Check environment variables (common across all platforms, prioritized)
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := os.Getenv(env); val != "" {
			if strings.HasPrefix(strings.ToLower(val), "ja") {
				currentLang = "ja"
				return
			}
			if strings.HasPrefix(strings.ToLower(val), "en") {
				currentLang = "en"
				return
			}
		}
	}

	// 2. Detect platform-specific system language (e.g., Windows API)
	if sysLocale() == "ja" {
		currentLang = "ja"
	} else {
		currentLang = "en"
	}
}

// SetLanguage explicitly changes the language to be used ("en" or "ja").
func SetLanguage(lang string) {
	lang = strings.ToLower(lang)
	if lang == "ja" || lang == "en" {
		currentLang = lang
	}
}

// GetLanguage returns the current language code.
func GetLanguage() string {
	return currentLang
}

// T translates and returns the message for the specified key.
// If args are provided, it interpolates placeholders using Go's template engine.
func T(key string, args interface{}) string {
	langTranslations, ok := translations[currentLang]
	if !ok {
		langTranslations = translations["en"]
	}

	msg, ok := langTranslations[key]
	if !ok {
		// Fallback to English
		msg, ok = translations["en"][key]
		if !ok {
			return key // Return the key itself if no translation is found
		}
	}

	if args == nil {
		return msg
	}

	// Interpolate placeholders with Go's template engine
	tmpl, err := template.New("msg").Parse(msg)
	if err != nil {
		return msg
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return msg
	}
	return buf.String()
}
