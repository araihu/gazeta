package gazeta

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNonPostSchemasMatchPagePurpose(t *testing.T) {
	posts := []document{testDocument(t, "tags = ['Go']")}
	for _, tc := range []struct{ source, route, kind string }{
		{"index.md", "/", "WebPage"}, {"blogs/index.md", "/blogs/", "CollectionPage"},
		{"tags/index.md", "/tags/", "CollectionPage"}, {"tags/go/index.md", "/tags/go/", "CollectionPage"},
		{"projects/index.md", "/projects/", "CollectionPage"}, {"about/index.md", "/about/", "AboutPage"},
		{"contact/index.md", "/contact/", "ContactPage"}, {"categories/index.md", "/categories/", "WebPage"},
	} {
		t.Run(tc.route, func(t *testing.T) {
			d := document{Source: tc.source, Meta: metadata{Title: "Page", Description: "Description"}}
			if tc.route == "/" || tc.route == "/blogs/" || tc.route == "/tags/go/" {
				d.SchemaItems = testBlog.postSchemaItems(posts)
			}
			raw, err := json.Marshal(testBlog.pageStructuredData(d))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), tc.kind) || strings.Contains(string(raw), "BlogPosting") {
				t.Fatalf("wrong type: %s", raw)
			}
			page := htmlPage{Canonical: testBlog.baseURL + tc.route, CanonicalCount: 1, HeadSchemas: 1, Schemas: []json.RawMessage{raw}, Refs: []string{posts[0].Route}, HeadMetadata: map[string][]string{"og:title": {"Page"}, "description": {"Description"}}}
			if err := testBlog.checkNonPostSchema(page, tc.route, posts); err != nil {
				t.Fatal(err)
			}
			for name, bad := range map[string]string{
				"wrong canonical": strings.ReplaceAll(string(raw), testBlog.baseURL+tc.route+`"`, testBlog.baseURL+"/wrong/"+`"`),
				"wrong context":   strings.ReplaceAll(string(raw), "https://schema.org", "https://wrong.example"),
				"wrong type":      strings.ReplaceAll(string(raw), tc.kind, "BlogPosting"),
			} {
				page.Schemas = []json.RawMessage{json.RawMessage(bad)}
				if err := testBlog.checkNonPostSchema(page, tc.route, posts); err == nil {
					t.Fatalf("accepted %s", name)
				}
			}
			page.Schemas = []json.RawMessage{raw, raw}
			if err := testBlog.checkNonPostSchema(page, tc.route, posts); err == nil {
				t.Fatal("accepted duplicate")
			}
		})
	}
	if testBlog.pageStructuredData(document{Source: "404.md"}) != nil {
		t.Fatal("404 received structured data")
	}
}

func TestStructuredListRejectsHiddenOrReorderedPosts(t *testing.T) {
	post := testDocument(t, "")
	d := document{Source: "index.md", Meta: metadata{Title: "Home"}, SchemaItems: testBlog.postSchemaItems([]document{post})}
	raw, err := json.Marshal(testBlog.pageStructuredData(d))
	if err != nil {
		t.Fatal(err)
	}
	page := htmlPage{Canonical: testBlog.baseURL + "/", CanonicalCount: 1, HeadSchemas: 1, Schemas: []json.RawMessage{raw}, HeadMetadata: map[string][]string{"og:title": {"Home"}}}
	if err := testBlog.checkNonPostSchema(page, "/", []document{post}); err == nil {
		t.Fatal("accepted item with no visible link")
	}
	page.Refs = []string{post.Route}
	if err := testBlog.checkNonPostSchema(page, "/", nil); err == nil {
		t.Fatal("accepted hidden post")
	}
}
