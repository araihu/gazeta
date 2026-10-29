package gazeta

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testDocument(t *testing.T, extra string) document {
	t.Helper()
	d, err := parseDocument([]byte("+++\ntitle = 'Go & HTML'\nlanguage = 'pt-BR'\npublishedAt = '2024-04-30T01:37:09-03:00'\nauthors = ['Ana', 'Rui']\n" + extra + "\n+++\n# Go & HTML\n\nPost body.\n"))
	if err != nil {
		t.Fatal(err)
	}
	d.Source = "blogs/test/index.md"
	d.Route = "/blogs/test/"
	return d
}

// Test fixture only. Production serializes through Margo's structured_data.
func schemaScript(d document) (string, error) {
	data, err := json.Marshal(testBlog.structuredData(d))
	return `<script type="application/ld+json">` + string(data) + `</script>`, err
}

func TestStructuredData(t *testing.T) {
	d := testDocument(t, "cover = './assets/a capa.webp'\nmodifiedAt = '2024-05-01T12:00:00Z'")
	d.Meta.Title = "Go </script><script>alert(1)</script> & HTML"
	script, err := schemaScript(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(script, "</script>") != 1 || strings.Contains(script, "<script>alert") {
		t.Fatal("metadata can break out of script")
	}
	page, err := readHTML([]byte(`<html><head><link rel="canonical" href="https://journal.example/blogs/test/">` + script + `</head><body></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := testBlog.checkSchema(page, &d); err != nil {
		t.Fatal(err)
	}
	var got blogPosting
	if err := json.Unmarshal(page.Schemas[0], &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "BlogPosting" || got.Headline != d.Meta.Title || got.Image != "https://journal.example/blogs/test/assets/a%20capa.webp" || got.Published != "2024-04-30T01:37:09-03:00" || got.Modified != "2024-05-01T12:00:00Z" {
		t.Fatalf("wrong schema: %+v", got)
	}
	if len(got.Authors) != 2 || got.Authors[0].Name != "Ana" || got.Authors[1].Type != "Person" {
		t.Fatalf("wrong authors: %+v", got.Authors)
	}
	d.Meta.Cover = ""
	d.Meta.ModifiedAt = ""
	script, err = schemaScript(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, `"image"`) || strings.Contains(script, `"dateModified"`) {
		t.Fatal("invented optional metadata")
	}
}

func TestAuthorProfile(t *testing.T) {
	d := testDocument(t, "")
	d.Meta.Authors = []string{"Alex Example", "Guest"}
	data := testBlog.structuredData(d)
	if data.Authors[0].URL != "https://journal.example/about/" || data.Authors[1].URL != "" {
		t.Fatalf("incorrect author profiles: %+v", data.Authors)
	}
	links := testBlog.publicationLinks(d.Meta)["authors"].(map[string]string)
	if links["Alex Example"] != testBlog.baseURL+"/about/" || links["Guest"] != "" {
		t.Fatalf("incorrect visible author destinations: %v", links)
	}
}

func TestSchemaValidationRejectsDrift(t *testing.T) {
	d := testDocument(t, "")
	script, err := schemaScript(d)
	if err != nil {
		t.Fatal(err)
	}
	valid := `<html><head><link rel="canonical" href="https://journal.example/blogs/test/">` + script + `</head><body></body></html>`
	for name, markup := range map[string]string{
		"missing":         strings.Replace(valid, script, "", 1),
		"duplicate":       strings.Replace(valid, script, script+script, 1),
		"wrong date":      strings.Replace(valid, "2024-04-30", "2023-04-30", 1),
		"wrong canonical": strings.Replace(valid, `href="https://journal.example/blogs/test/"`, `href="https://journal.example/"`, 1),
		"invalid JSON":    strings.Replace(valid, `"@context"`, `broken`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			page, err := readHTML([]byte(markup))
			if err != nil {
				t.Fatal(err)
			}
			if testBlog.checkSchema(page, &d) == nil {
				t.Fatal("accepted invalid schema")
			}
		})
	}
	page, err := readHTML([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if testBlog.checkSchema(page, nil) == nil {
		t.Fatal("accepted post schema on archive")
	}
}

func TestPublicationDates(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		date        string
		draft, want bool
	}{
		{"2026-10-27T00:00:00-03:00", false, false},
		{"2026-10-26T21:00:00-03:00", false, true},
		{"2024-04-30T01:37:09-03:00", true, false},
	} {
		d := testDocument(t, "")
		d.Meta.Draft = tc.draft
		var err error
		d.Published, err = time.Parse(time.RFC3339, tc.date)
		if err != nil {
			t.Fatal(err)
		}
		if d.visible(now) != tc.want {
			t.Errorf("date %s draft %v visibility wrong", tc.date, tc.draft)
		}
	}
	for _, front := range []string{
		"publishedAt = '2026-10-27T00:00:00'",
		"publishedAt = 'invalid'",
		"publishedAt = '2026-10-27T00:00:00Z'\nmodifiedAt = '2026-10-26T23:59:59Z'",
	} {
		if _, err := parseDocument([]byte("+++\ntitle = 'test'\n" + front + "\n+++\nbody")); err == nil {
			t.Errorf("accepted invalid dates: %s", front)
		}
	}
}

func TestEditorialHTML(t *testing.T) {
	d := testDocument(t, "cover = './assets/cover.webp'\ncover_alt = 'A \"cover\"'\ntags = ['SEO', 'Automação']")
	d.Meta.Title = `<script>alert("title")</script>`
	d.Meta.Description = "Go & HTML"
	for source, want := range map[string]string{"index.md": "blogs/test/assets/cover.webp", "blogs/index.md": "test/assets/cover.webp", "tags/seo/index.md": "../../blogs/test/assets/cover.webp"} {
		output, err := testBlog.listing([]document{d}, source)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output, "<script>") || !strings.Contains(output, "&lt;script&gt;") || !strings.Contains(output, "Go &amp; HTML") || !strings.Contains(output, `src="`+want+`"`) {
			t.Fatalf("unsafe or incorrect listing: %s", output)
		}
	}
	for tag, want := range map[string]string{"SEO": "seo", "social media": "social-media", "Automação": "automacao", "///": ""} {
		if got := tagSlug(tag); got != want {
			t.Errorf("slug %q: %q", tag, got)
		}
	}
	links := testBlog.publicationLinks(d.Meta)["tags"].(map[string]string)
	if links["Automação"] != testBlog.baseURL+"/tags/automacao/" {
		t.Fatalf("bad tag destination: %v", links)
	}
}

func TestPageDeclaresNativeStructuredData(t *testing.T) {
	d := testDocument(t, "")
	stage := t.TempDir()
	if err := testBlog.writePage(stage, d); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(stage, d.Source))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, `class="blog-footer"`) {
		t.Fatal("post must use the page footer supplied by Margo")
	}
	if strings.Contains(text, "data-margo-publication-metadata") || !strings.Contains(text, `"publication_links":`) {
		t.Fatal("Margo must render publication details from declared metadata")
	}

	if strings.Contains(text, `application/ld+json`) || strings.Count(text, `"structured_data":`) != 1 {
		t.Fatal("expected native structured_data without handwritten script")
	}
	if !strings.Contains(text, `"layout":{"kind":"docs","values":{"toc":true}}`) {
		t.Fatal("post must enable Margo's native TOC without the docs sidebar")
	}
	d.Route = ""
	d.Source = "index.md"
	if err := testBlog.writePage(stage, d); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(stage, d.Source))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "BlogPosting") || !strings.Contains(string(data), "WebSite") {
		t.Fatal("home must describe WebSite, not BlogPosting")
	}
	if strings.Contains(string(data), `"kind":"docs"`) {
		t.Fatal("post layout leaked to home through shared frontmatter")
	}
	if strings.Contains(string(data), `class="blog-footer"`) {
		t.Fatal("Margo must own the page footer")
	}
}

func TestFeedEscapingAndTimezone(t *testing.T) {
	d := testDocument(t, "")
	d.Meta.Title = "Go & <HTML>"
	file := filepath.Join(t.TempDir(), "index.xml")
	if err := testBlog.writeFeed(file, "Example Journal", []document{d}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var feed rss
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Channel.Items) != 1 || feed.Channel.Items[0].Title != d.Meta.Title || !strings.HasSuffix(feed.Channel.Items[0].Date, "-0300") {
		t.Fatalf("wrong feed: %+v", feed)
	}
}

func TestLoadDocumentsRejectsMissingMetadataAndCover(t *testing.T) {
	for name, extra := range map[string]string{"missing author": "authors = []", "missing date": "publishedAt = ''", "missing cover": "cover = './assets/missing.webp'", "escaping cover": "cover = '../other.webp'"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			front := "title = 'Post'\nauthors = ['Ana']\npublishedAt = '2024-01-01T00:00:00Z'\n"
			if name == "missing author" {
				front = "title = 'Post'\npublishedAt = '2024-01-01T00:00:00Z'\n"
			}
			if name == "missing date" {
				front = "title = 'Post'\nauthors = ['Ana']\n"
			}
			if err := writeFile(filepath.Join(root, "content/blogs/test/index.md"), []byte("+++\n"+front+extra+"\n+++\nbody")); err != nil {
				t.Fatal(err)
			}
			if _, _, err := loadDocuments(root, time.Now()); err == nil {
				t.Fatal("accepted invalid post")
			}
		})
	}
}

func TestAliasesAndPublicationRollback(t *testing.T) {
	d := testDocument(t, "")
	render := t.TempDir()
	d.Meta.Aliases = []string{"/posts/test/"}
	if err := testBlog.writeAliases(render, []document{d}); err != nil {
		t.Fatal(err)
	}
	if err := testBlog.writeAliases(render, []document{d}); err == nil {
		t.Fatal("alias collision ignored")
	}
	d.Meta.Aliases = []string{"/../../escape/"}
	if err := testBlog.writeAliases(render, []document{d}); err == nil {
		t.Fatal("unsafe alias accepted")
	}
	root := t.TempDir()
	public := filepath.Join(root, "public")
	if err := writeFile(filepath.Join(public, "index.html"), []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := publishDirectory(filepath.Join(root, "missing"), public); err == nil {
		t.Fatal("missing render accepted")
	}
	data, err := os.ReadFile(filepath.Join(public, "index.html"))
	if err != nil || string(data) != "old" {
		t.Fatal("old output lost on failed publish")
	}
}
