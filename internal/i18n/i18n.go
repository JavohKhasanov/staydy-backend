// Package i18n provides minimal localization for client-facing messages (uz/ru/en).
package i18n

import "strings"

type Lang string

const (
	LangUZ Lang = "uz"
	LangRU Lang = "ru"
	LangEN Lang = "en"
)

const DefaultLang = LangUZ

// Detect picks a supported language from an Accept-Language header (or a Telegram
// language_code), falling back to the default (Uzbek).
func Detect(header string) Lang {
	header = strings.ToLower(strings.TrimSpace(header))
	switch {
	case strings.HasPrefix(header, "ru"):
		return LangRU
	case strings.HasPrefix(header, "en"):
		return LangEN
	case strings.HasPrefix(header, "uz"):
		return LangUZ
	default:
		return DefaultLang
	}
}

// Message is a localized string with uz/ru/en variants. UZ is the source of truth;
// RU/EN fall back to UZ when empty.
type Message struct {
	UZ string
	RU string
	EN string
}

// For returns the message in the requested language, falling back to Uzbek.
func (m Message) For(lang Lang) string {
	switch lang {
	case LangRU:
		if m.RU != "" {
			return m.RU
		}
	case LangEN:
		if m.EN != "" {
			return m.EN
		}
	}
	return m.UZ
}
