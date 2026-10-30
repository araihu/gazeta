package gazeta

import (
	"reflect"
	"strings"

	"github.com/araihu/margo/site"
)

func documentLocale(source string, locales site.LocaleConfig) (string, string) {
	for _, locale := range locales.Supported {
		prefix := strings.ToLower(locale) + "/"
		if strings.HasPrefix(source, prefix) {
			return locale, strings.TrimPrefix(source, prefix)
		}
	}
	return locales.Default, source
}

func (e *engine) forLocale(locale string) *engine {
	local := *e
	local.language = locale
	local.prefix = ""
	if locale != e.site.Locales.Default {
		local.prefix = strings.ToLower(locale) + "/"
	}
	if override, ok := e.config.Locales[locale]; ok {
		values, defaults := reflect.ValueOf(&local.config.Labels).Elem(), reflect.ValueOf(override)
		for i := 0; i < values.NumField(); i++ {
			if value := defaults.Field(i).String(); value != "" {
				values.Field(i).SetString(value)
			}
		}
	}
	return &local
}

func (e *engine) source(source string) string { return e.prefix + source }
func (e *engine) route(route string) string   { return "/" + e.prefix + strings.TrimPrefix(route, "/") }

func localeDocuments(docs []document, locale string) []document {
	var selected []document
	for _, d := range docs {
		if d.Meta.Language == locale {
			selected = append(selected, d)
		}
	}
	return selected
}

func (e *engine) authorType(name string) string {
	if kind := e.config.Authors[name].Type; kind != "" {
		return kind
	}
	return "Person"
}
