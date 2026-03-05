// Package locales provides embedded locale files for i18n support.
//
// Go's embed directive cannot traverse parent directories (..),
// so the embed.FS must live in the same directory as the locale
// YAML files. Other packages import this to access translations.
package locales

import "embed"

// FS contains all embedded .yml locale files.
//
//go:embed *.yml
var FS embed.FS
