package gazeta

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func multilingualRoot(t *testing.T) string {
	t.Helper()
	root := exampleRoot(t)
	replaceTestFile(t, root, "site.yaml", "supported: [en]", "supported: [en, pt-BR, es]")
	replaceTestFile(t, root, "blog.yaml", "url: /about/", "url: /about/\n    type: Organization")
	replaceTestFile(t, root, "blog.yaml", "ProfilePage", "AboutPage")
	blog := readTestFile(t, root, "blog.yaml") + "\nlocales:\n  pt-BR: {blog: Artigos, articles: Artigos}\n  es: {blog: Artículos, articles: Artículos}\n"
	if err := writeFile(filepath.Join(root, "blog.yaml"), []byte(blog)); err != nil {
		t.Fatal(err)
	}

	for _, prefix := range []string{"pt-br", "es"} {
		for _, source := range []string{"index.md", "about/index.md", "blogs/hello/index.md"} {
			body := readTestFile(t, root, "content/"+source)
			body = strings.ReplaceAll(body, "aliases = ['/posts/hello/']", "aliases = ['\u002f"+prefix+"/posts/hello/']")
			if err := writeFile(filepath.Join(root, "content", prefix, source), []byte(body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestMultilingualPublication(t *testing.T) {
	root := multilingualRoot(t)
	for _, identity := range []struct{ name, domain, author string }{
		{"Field Notes", "https://notes.example", "Alex Example"},
		{"Garden Journal", "https://garden.example", "Garden Collective"},
	} {
		if identity.name != "Field Notes" {
			replaceTestFile(t, root, "site.yaml", "Field Notes", identity.name)
			replaceTestFile(t, root, "site.yaml", "https://notes.example", identity.domain)
			replaceTestFile(t, root, "blog.yaml", "Alex Example", identity.author)
			for _, prefix := range []string{"", "pt-br/", "es/"} {
				replaceTestFile(t, root, "content/"+prefix+"blogs/hello/index.md", "Alex Example", identity.author)
			}
		}
		if err := Build(context.Background(), root, buildTime); err != nil {
			t.Fatal(err)
		}
		for locale, prefix := range map[string]string{"en": "", "pt-BR": "pt-br/", "es": "es/"} {
			page, err := readHTML([]byte(readTestFile(t, root, "public/"+prefix+"blogs/hello/index.html")))
			if err != nil {
				t.Fatal(err)
			}
			var article blogPosting
			if err := json.Unmarshal(page.Schemas[0], &article); err != nil {
				t.Fatal(err)
			}
			if page.Language != locale || article.Language != locale || article.Authors[0].Type != "Organization" || article.Authors[0].Name != identity.author || article.Authors[0].URL != identity.domain+"/"+prefix+"about/" {
				t.Fatalf("incorrect localized organization author: %+v", article)
			}
			if len(page.Alternates) != 4 || page.Alternates[locale][0] != page.Canonical || page.Alternates["x-default"][0] != identity.domain+"/blogs/hello/" {
				t.Fatalf("incorrect equivalents: %+v", page.Alternates)
			}
			if locale != "en" {
				label := "Artigos"
				if locale == "es" {
					label = "Artículos"
				}
				if !strings.Contains(readTestFile(t, root, "public/"+prefix+"blogs/index.html"), label) {
					t.Fatal("missing localized editorial label")
				}
			}
			var feed rss
			if err := xml.Unmarshal([]byte(readTestFile(t, root, "public/"+prefix+"index.xml")), &feed); err != nil {
				t.Fatal(err)
			}
			if len(feed.Channel.Items) != 1 || feed.Channel.Language != locale || feed.Channel.Items[0].Link != page.Canonical {
				t.Fatalf("mixed feed: %+v", feed)
			}
		}
	}
	file := filepath.Join(root, "public/es/blogs/hello/index.html")
	data := readTestFile(t, root, "public/es/blogs/hello/index.html")
	data = strings.Replace(data, `hreflang="en"`, `hreflang="fr"`, 1)
	if err := os.WriteFile(file, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Check(root, buildTime); err == nil || !strings.Contains(err.Error(), "language alternates") {
		t.Fatalf("accepted corrupted alternates: %v", err)
	}
}

func TestMultilingualVisibilityAndSeries(t *testing.T) {
	root := multilingualRoot(t)
	for _, prefix := range []string{"", "pt-br/", "es/"} {
		for part := 1; part <= 2; part++ {
			body := fmt.Sprintf("+++\ntitle = '%spart %d'\ndescription = 'Series part.'\nauthors = ['Guest']\npublishedAt = '2026-01-02T09:00:00Z'\nseries = 'Shared series'\nseriesPart = %d\n+++\nBody.\n", prefix, part, part)
			if err := writeFile(filepath.Join(root, "content", prefix, fmt.Sprintf("blogs/part-%d/index.md", part)), []byte(body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, fixture := range []struct{ slug, visibility string }{{"draft", "draft = true"}, {"future", "publishedAt = '2099-01-01T00:00:00Z'"}} {
		body := "+++\ntitle = 'PRIVATE_TRANSLATION'\nauthors = ['Guest']\n" + fixture.visibility + "\n+++\nPrivate.\n"
		if err := writeFile(filepath.Join(root, "content/es/blogs", fixture.slug, "index.md"), []byte(body)); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(filepath.Join(root, "content/es/blogs", fixture.slug, "assets/private.txt"), []byte("private")); err != nil {
			t.Fatal(err)
		}
	}
	for _, prefix := range []string{"pt-br/", "es/"} {
		body := "+++\ntitle = 'Available translation'\ndescription = 'No default translation.'\nauthors = ['Guest']\npublishedAt = '2026-01-02T09:00:00Z'\n+++\nBody.\n"
		if err := writeFile(filepath.Join(root, "content", prefix, "blogs/no-default/index.md"), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(root, "content/es/blogs/hello/index.md")); err != nil {
		t.Fatal(err)
	}
	if err := Build(context.Background(), root, buildTime); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"draft", "future", "hello"} {
		if _, err := os.Stat(filepath.Join(root, "public/es/blogs", slug)); !os.IsNotExist(err) {
			t.Fatalf("unavailable translation leaked: %s", slug)
		}
	}
	for _, prefix := range []string{"", "pt-br/", "es/"} {
		page := readTestFile(t, root, "public/"+prefix+"blogs/part-1/index.html")
		if !strings.Contains(page, `href="/`+prefix+`blogs/part-2/"`) {
			t.Fatalf("missing locale series navigation: %s", prefix)
		}
		if prefix != "" && strings.Contains(page, `href="/blogs/part-2/"`) {
			t.Fatal("series crosses languages")
		}
	}
	translated, err := readHTML([]byte(readTestFile(t, root, "public/es/blogs/no-default/index.html")))
	if err != nil {
		t.Fatal(err)
	}
	if len(translated.Alternates) != 2 || translated.Alternates["x-default"] != nil || translated.Alternates["en"] != nil {
		t.Fatalf("invented default equivalent: %+v", translated.Alternates)
	}
	page, err := readHTML([]byte(readTestFile(t, root, "public/blogs/hello/index.html")))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Alternates) != 3 || page.Alternates["es"] != nil {
		t.Fatalf("advertised missing Spanish translation: %+v", page.Alternates)
	}
}
