package gazeta

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSeriesVisibleNavigation(t *testing.T) {
	root := t.TempDir()
	for i := 1; i <= 4; i++ {
		front := fmt.Sprintf("+++\ntitle = 'Lab, parte %d: assunto'\nseries = 'Lab'\nseriesPart = %d\npublishedAt = '2024-01-01T00:00:00Z'\nauthors = ['Ana']\ndraft = %t\n+++\nTexto.\n", i, i, i == 2)
		if i == 4 {
			front = strings.Replace(front, "2024-01-01", "2099-01-01", 1)
		}
		if err := writeFile(filepath.Join(root, fmt.Sprintf("content/blogs/part-%d/index.md", i)), []byte(front)); err != nil {
			t.Fatal(err)
		}
	}
	_, posts, err := loadDocuments(root, time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	parts := seriesPosts("Lab", posts)
	if len(parts) != 2 || parts[0].Meta.SeriesPart != 1 || parts[1].Meta.SeriesPart != 3 {
		t.Fatalf("unexpected visible order: %+v", parts)
	}
	index, next := testBlog.seriesNavigation(parts[0], posts)
	if strings.Contains(index+next, "/part-2/") || strings.Contains(index+next, "/part-4/") {
		t.Fatal("navigation exposes unpublished parts")
	}
	if !strings.Contains(index, `href="/blogs/part-1/" aria-current="page"`) || !strings.Contains(next, `rel="next" href="/blogs/part-3/"`) || strings.Contains(next, `rel="prev"`) {
		t.Fatalf("incorrect first-part navigation: %s %s", index, next)
	}
	_, previous := testBlog.seriesNavigation(parts[1], posts)
	if !strings.Contains(previous, `rel="prev" href="/blogs/part-1/"`) || strings.Contains(previous, `rel="next"`) {
		t.Fatalf("incorrect last-part navigation: %s", previous)
	}
	if !strings.Contains(testBlog.seriesEntry(posts), `href="/blogs/part-1/"`) || testBlog.seriesEntry(parts[1:]) != "" || testBlog.seriesEntry(nil) != "" {
		t.Fatal("entry must point to an available first part")
	}
	// A visible duplicate must fail rather than produce ambiguous navigation.
	duplicate := "+++\ntitle = 'Duplicate'\nseries = 'Lab'\nseriesPart = 1\npublishedAt = '2024-01-01T00:00:00Z'\nauthors = ['Ana']\n+++\nText."
	if err := writeFile(filepath.Join(root, "content/blogs/duplicate/index.md"), []byte(duplicate)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadDocuments(root, time.Now()); err == nil {
		t.Fatal("accepted duplicate series part")
	}
}

func TestSeriesMetadataAndEscaping(t *testing.T) {
	for _, extra := range []string{"series = 'Lab'", "seriesPart = 1", "series = 'Lab'\nseriesPart = -1", "series = ' '\nseriesPart = 2", "series = ' '"} {
		if _, err := parseDocument([]byte("+++\ntitle = 'Post'\n" + extra + "\n+++\nText.")); err == nil {
			t.Errorf("accepted invalid series metadata: %s", extra)
		}
	}
	d := testDocument(t, "series = '<Lab>'\nseriesPart = 1")
	d.Meta.Title = "<script>"
	index, pagination := testBlog.seriesNavigation(d, []document{d})
	if strings.Contains(index+pagination+testBlog.seriesEntry([]document{d}), "<Lab>") || strings.Contains(index, "<script>") || !strings.Contains(index, "&lt;script&gt;") {
		t.Fatal("unescaped series metadata")
	}
	plain := testDocument(t, "")
	index, pagination = testBlog.seriesNavigation(plain, []document{plain})
	if index != "" || pagination != "" || testBlog.seriesEntry([]document{plain}) != "" {
		t.Fatal("series UI added to standalone post")
	}
}

func TestPostOrderUsesDatesThenSeriesParts(t *testing.T) {
	root := t.TempDir()
	for _, post := range []struct {
		slug, series, date string
		part               int
	}{
		{"z-hardware", "Homelab", "2026-10-27T12:00:00Z", 1},
		{"a-apps", "Homelab", "2026-10-27T12:00:00Z", 2},
		{"m-network", "Homelab", "2026-10-27T12:00:00Z", 3},
		{"b-operations", "Homelab", "2026-10-27T12:00:00Z", 4},
		{"n-standalone", "", "2026-10-27T12:00:00Z", 0},
		{"a-other-two", "Another", "2026-10-27T12:00:00Z", 2},
		{"z-other-one", "Another", "2026-10-27T12:00:00Z", 1},
		{"older", "Homelab", "2026-10-26T12:00:00Z", 5},
		{"newer", "", "2026-10-28T12:00:00Z", 0},
	} {
		front := fmt.Sprintf("+++\ntitle = '%s'\nauthors = ['Ana']\npublishedAt = '%s'\n", post.slug, post.date)
		if post.series != "" {
			front += fmt.Sprintf("series = '%s'\nseriesPart = %d\n", post.series, post.part)
		}
		if err := writeFile(filepath.Join(root, "content/blogs", post.slug, "index.md"), []byte(front+"+++\nText.\n")); err != nil {
			t.Fatal(err)
		}
	}
	_, posts, err := loadDocuments(root, time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"newer", "z-hardware", "a-apps", "m-network", "b-operations", "z-other-one", "a-other-two", "n-standalone", "older"}
	if len(posts) != len(want) {
		t.Fatalf("got %d posts", len(posts))
	}
	for i, slug := range want {
		if posts[i].Route != "/blogs/"+slug+"/" {
			t.Fatalf("position %d: got %s, want %s", i, posts[i].Route, slug)
		}
	}
}
