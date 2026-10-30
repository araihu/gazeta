package gazeta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSitemapRequiresExactlyIndexableCanonicals(t *testing.T) {
	pages := map[string]htmlPage{
		"index.html": {Canonical: "https://notes.example/"},
		"post.html":  {Canonical: "https://notes.example/post/"},
		"alias.html": {Canonical: "https://notes.example/post/"},
		"404.html":   {Canonical: "https://notes.example/404.html", HeadMetadata: map[string][]string{"robots": {"noindex, follow"}}},
	}
	for _, test := range []struct {
		name, locations string
		valid           bool
	}{
		{"canonical pages", "https://notes.example/|https://notes.example/post/", true},
		{"noindex error page", "https://notes.example/|https://notes.example/post/|https://notes.example/404.html", false},
		{"missing article", "https://notes.example/", false},
		{"duplicate home", "https://notes.example/|https://notes.example/|https://notes.example/post/", false},
		{"wrong origin", "https://other.example/|https://notes.example/post/", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			xml := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`
			for _, location := range strings.Split(test.locations, "|") {
				xml += "<url><loc>" + location + "</loc></url>"
			}
			xml += "</urlset>"
			if err := os.WriteFile(filepath.Join(root, "sitemap.xml"), []byte(xml), 0600); err != nil {
				t.Fatal(err)
			}
			if err := checkSitemap(root, pages); (err == nil) != test.valid {
				t.Fatalf("sitemap validation = %v, valid = %v", err, test.valid)
			}
		})
	}
}
