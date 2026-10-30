package gazeta

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func checkSitemap(public string, pages map[string]htmlPage) error {
	expected := map[string]bool{}
	blocked := map[string]bool{}
	for _, page := range pages {
		expected[page.Canonical] = true
		for _, robots := range page.HeadMetadata["robots"] {
			for _, directive := range strings.Split(robots, ",") {
				if strings.EqualFold(strings.TrimSpace(directive), "noindex") {
					blocked[page.Canonical] = true
				}
			}
		}
	}
	for canonical := range blocked {
		delete(expected, canonical)
	}
	data, err := os.ReadFile(filepath.Join(public, "sitemap.xml"))
	if err != nil {
		return err
	}
	var sitemap struct {
		XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		URLs    []struct {
			Location string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(data, &sitemap); err != nil {
		return fmt.Errorf("invalid sitemap: %w", err)
	}
	for _, entry := range sitemap.URLs {
		if !expected[entry.Location] {
			return fmt.Errorf("sitemap contains a duplicate, noncanonical or noindex URL: %s", entry.Location)
		}
		delete(expected, entry.Location)
	}
	if len(expected) != 0 {
		return fmt.Errorf("sitemap omits %d indexable canonical pages", len(expected))
	}
	return nil
}
