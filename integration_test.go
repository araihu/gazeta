package gazeta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func exampleRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := copyTree("examples/minimal", root); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"public", ".build"} {
		if err := os.RemoveAll(filepath.Join(root, dir)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func readTestFile(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func replaceTestFile(t *testing.T, root, name, old, new string) {
	t.Helper()
	s := readTestFile(t, root, name)
	if !strings.Contains(s, old) {
		t.Fatalf("fixture lacks %q", old)
	}
	if err := writeFile(filepath.Join(root, name), []byte(strings.ReplaceAll(s, old, new))); err != nil {
		t.Fatal(err)
	}
}

var buildTime = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func TestBuildIndependentIdentities(t *testing.T) {
	for _, identity := range []struct{ name, domain, author, profile, color string }{
		{"Field Notes", "https://notes.example", "Alex Example", "/about/", "#334455"},
		{"Garden Journal", "https://garden.example", "Taylor Gardener", "https://authors.example/taylor/", "#557733"},
	} {
		t.Run(identity.name, func(t *testing.T) {
			root := exampleRoot(t)
			replaceTestFile(t, root, "site.yaml", "Field Notes", identity.name)
			replaceTestFile(t, root, "site.yaml", "https://notes.example", identity.domain)
			replaceTestFile(t, root, "blog.yaml", "Alex Example", identity.author)
			replaceTestFile(t, root, "blog.yaml", "/about/\n", identity.profile+"\n")
			replaceTestFile(t, root, "content/blogs/hello/index.md", "Alex Example", identity.author)
			if err := writeFile(filepath.Join(root, "assets/editorial.css"), []byte("body { --color-primary: "+identity.color+"; }")); err != nil {
				t.Fatal(err)
			}
			if err := Build(context.Background(), root, buildTime); err != nil {
				t.Fatal(err)
			}
			if err := Check(root, buildTime); err != nil {
				t.Fatal(err)
			}
			page, err := readHTML([]byte(readTestFile(t, root, "public/blogs/hello/index.html")))
			if err != nil {
				t.Fatal(err)
			}
			if page.Canonical != identity.domain+"/blogs/hello/" || page.Language != "en" {
				t.Fatalf("incorrect identity: %+v", page)
			}
			var posting blogPosting
			if len(page.Schemas) != 1 {
				t.Fatal("expected native BlogPosting")
			}
			if err := json.Unmarshal(page.Schemas[0], &posting); err != nil {
				t.Fatal(err)
			}
			profile := identity.profile
			if strings.HasPrefix(profile, "/") {
				profile = identity.domain + profile
			}
			if len(posting.Authors) != 2 || posting.Authors[0].Name != identity.author || posting.Authors[0].URL != profile || posting.Authors[1].URL != "" {
				t.Fatalf("wrong authors: %+v", posting.Authors)
			}
			html := readTestFile(t, root, "public/blogs/hello/index.html")
			if !strings.Contains(html, `href="`+profile+`"`) || !strings.Contains(html, ">January 1, 2026</time>") {
				t.Fatal("native author/date missing")
			}
			feed := readTestFile(t, root, "public/index.xml")
			if !strings.Contains(feed, "<title>"+identity.name+"</title>") || !strings.Contains(feed, identity.domain+"/blogs/hello/") {
				t.Fatal("RSS identity differs")
			}
			if !strings.Contains(readTestFile(t, root, "public/robots.txt"), identity.domain+"/sitemap.xml") {
				t.Fatal("robots identity differs")
			}
			if !strings.Contains(readTestFile(t, root, "public/assets/editorial.css"), identity.color) {
				t.Fatal("consumer CSS lost")
			}
			if strings.Contains(html+feed, "guigoes") || strings.Contains(html+feed, "Guilherme") {
				t.Fatal("consumer identity leaked into engine")
			}
			if identity.name != "Field Notes" && strings.Contains(html+feed, "https://notes.example") {
				t.Fatal("previous build identity leaked")
			}
		})
	}
}

func TestBuildVisibilityAndRollback(t *testing.T) {
	root := exampleRoot(t)
	for _, name := range []string{"draft", "future"} {
		extra := "draft = true\npublishedAt = '2020-01-01T00:00:00Z'"
		if name == "future" {
			extra = "publishedAt = '2099-01-01T00:00:00Z'"
		}
		if err := writeFile(filepath.Join(root, "content/blogs", name, "index.md"), []byte("+++\ntitle = 'PRIVATE_POST'\nauthors = ['Guest']\n"+extra+"\n+++\nPRIVATE_BODY")); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(filepath.Join(root, "content/blogs", name, "assets/private.txt"), []byte("private asset")); err != nil {
			t.Fatal(err)
		}
	}
	if err := Build(context.Background(), root, buildTime); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"draft", "future"} {
		if _, err := os.Stat(filepath.Join(root, "public/blogs", name)); !os.IsNotExist(err) {
			t.Fatal("unpublished directory leaked")
		}
	}
	for _, file := range []string{"index.html", "index.xml", "sitemap.xml", "margo-manifest.json"} {
		if strings.Contains(readTestFile(t, root, "public/"+file), "PRIVATE_") {
			t.Fatal("private metadata leaked")
		}
	}
	before := readTestFile(t, root, "public/margo-manifest.json")
	if err := writeFile(filepath.Join(root, "required-urls.txt"), []byte("/missing-route/")); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(root, "blog.yaml"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintln(f, "required_urls: required-urls.txt")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := Build(context.Background(), root, buildTime); err == nil {
		t.Fatal("invalid publication accepted")
	}
	if after := readTestFile(t, root, "public/margo-manifest.json"); after != before {
		t.Fatal("failed build replaced previous publication")
	}
}

func TestAuthoredHTMLRequiresOptIn(t *testing.T) {
	root := exampleRoot(t)
	replaceTestFile(t, root, "content/blogs/hello/index.md", "Content stays in Git.", "<script>window.reviewedDemo=true</script>\n\nContent stays in Git.")
	if err := Build(context.Background(), root, buildTime); err == nil {
		t.Fatal("authored script accepted without opt-in")
	}
	f, err := os.OpenFile(filepath.Join(root, "blog.yaml"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintln(f, "unsafe_html: true")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := Build(context.Background(), root, buildTime); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readTestFile(t, root, "public/blogs/hello/index.html"), "window.reviewedDemo=true") {
		t.Fatal("reviewed demo lost")
	}
}
