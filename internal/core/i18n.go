package core

import (
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	currentLocale string = "en"
	translations  map[string]string
)

// InitI18n loads translations for the given locale from the provided filesystem.
// The filesystem should contain YAML files named "<locale>.yml" at its root.
// Falls back to "en" if the requested locale file does not exist.
func InitI18n(localeFS fs.FS, locale string) error {
	currentLocale = locale
	translations = make(map[string]string)

	data, err := fs.ReadFile(localeFS, locale+".yml")
	if err != nil {
		// Fall back to English if the requested locale is missing.
		if locale != "en" {
			currentLocale = "en"
			data, err = fs.ReadFile(localeFS, "en.yml")
			if err != nil {
				return fmt.Errorf("i18n: could not load fallback locale en: %w", err)
			}
		} else {
			return fmt.Errorf("i18n: could not load locale %s: %w", locale, err)
		}
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("i18n: failed to parse %s.yml: %w", currentLocale, err)
	}

	flatten("", raw, translations)
	return nil
}

// CurrentLocale returns the active locale code (e.g. "en", "it").
func CurrentLocale() string {
	return currentLocale
}

// T returns the translated string for the given dot-separated key.
// If the key is not found, the key itself is returned.
func T(key string) string {
	if translations == nil {
		return key
	}
	if val, ok := translations[key]; ok {
		return val
	}
	return key
}

// Tf returns the translated string with named placeholders replaced.
// Placeholders use %{name} syntax. Arguments are provided as key-value pairs.
//
// Example:
//
//	Tf("scan.progress", "module", "ssh", "percent", "75")
//	// "Scanning ssh... 75%" (if the template is "Scanning %{module}... %{percent}%")
func Tf(key string, args ...string) string {
	text := T(key)
	for i := 0; i+1 < len(args); i += 2 {
		text = strings.ReplaceAll(text, "%{"+args[i]+"}", args[i+1])
	}
	return text
}

// flatten recursively walks a nested map and produces flat dot-separated keys.
//
// For example:
//
//	finding:
//	  ssh_root_login:
//	    title: "Root login enabled"
//
// becomes:
//
//	"finding.ssh_root_login.title" => "Root login enabled"
func flatten(prefix string, m map[string]any, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]any:
			flatten(key, val, out)
		case string:
			out[key] = val
		default:
			// Convert other scalar types (int, bool, float) to string.
			out[key] = fmt.Sprintf("%v", val)
		}
	}
}
