package gazeta

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

func (e *engine) checkAlternates(public string, pages map[string]htmlPage, posts []document) error {
	groups := map[string]map[string]string{}
	keys := map[string]string{}
	for file, page := range pages {
		relative, err := filepath.Rel(public, file)
		if err != nil {
			return err
		}
		route := publicRoute(filepath.ToSlash(relative))
		if isAlias(route, posts) || strings.Contains(strings.Join(page.HeadMetadata["robots"], ","), "noindex") {
			continue
		}
		_, key := documentLocale(strings.TrimPrefix(route, "/"), e.site.Locales)
		if groups[key] == nil {
			groups[key] = map[string]string{}
		}
		if previous := groups[key][page.Language]; previous != "" {
			return fmt.Errorf("duplicate locale equivalent: %s", page.Canonical)
		}
		groups[key][page.Language], keys[file] = page.Canonical, key
	}
	for file, page := range pages {
		expected := map[string][]string{}
		if key, exists := keys[file]; exists && len(groups[key]) > 1 {
			for locale, canonical := range groups[key] {
				expected[locale] = []string{canonical}
			}
			if canonical := groups[key][e.site.Locales.Default]; canonical != "" {
				expected["x-default"] = []string{canonical}
			}
		}
		if !reflect.DeepEqual(page.Alternates, expected) {
			return fmt.Errorf("%s: language alternates differ from indexable equivalents", page.Canonical)
		}
	}
	return nil
}
