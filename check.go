package gazeta

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type htmlPage struct {
	IDs                         map[string]bool
	Refs                        []string
	Canonical, Language         string
	Schemas                     []json.RawMessage
	HeadSchemas, CanonicalCount int
	HeadMetadata                map[string][]string
}

func readHTML(data []byte) (htmlPage, error) {
	page := htmlPage{IDs: map[string]bool{}, HeadMetadata: map[string][]string{}}
	root, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return page, err
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if id := attrs["id"]; id != "" {
				page.IDs[id] = true
			}
			if n.Data == "html" {
				page.Language = attrs["lang"]
			}
			if n.Data == "meta" && n.Parent != nil && n.Parent.Data == "head" {
				key := attrs["property"]
				if key == "" {
					key = attrs["name"]
				}
				page.HeadMetadata[key] = append(page.HeadMetadata[key], attrs["content"])
			}
			if n.Data == "link" && attrs["rel"] == "canonical" {
				page.Canonical = attrs["href"]
				page.CanonicalCount++
			}
			for _, key := range []string{"href", "src"} {
				if attrs[key] != "" {
					page.Refs = append(page.Refs, attrs[key])
				}
			}
			if n.Data == "script" && attrs["type"] == "application/ld+json" {
				var text strings.Builder
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					if child.Type == html.TextNode {
						text.WriteString(child.Data)
					}
				}
				page.Schemas = append(page.Schemas, json.RawMessage(text.String()))
				if n.Parent != nil && n.Parent.Data == "head" {
					page.HeadSchemas++
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return page, nil
}

func target(public string, u *url.URL) (string, error) {
	relative := strings.TrimPrefix(u.Path, "/")
	if relative != "" && !filepath.IsLocal(filepath.FromSlash(relative)) {
		return "", fmt.Errorf("URL escapes publication: %s", u)
	}
	file := filepath.Join(public, filepath.FromSlash(relative))
	info, err := os.Stat(file)
	if err == nil && info.IsDir() {
		file = filepath.Join(file, "index.html")
	} else if os.IsNotExist(err) && filepath.Ext(file) == "" {
		file += ".html"
	}
	return file, nil
}

func (e *engine) checkSchema(page htmlPage, post *document) error {
	if post == nil {
		if len(page.Schemas) > 0 {
			return fmt.Errorf("unexpected JSON-LD on a non-post page")
		}
		return nil
	}
	if len(page.Schemas) != 1 || page.HeadSchemas != 1 || page.CanonicalCount != 1 {
		return fmt.Errorf("expected one BlogPosting, got %d JSON-LD blocks", len(page.Schemas))
	}
	var got map[string]any
	if err := json.Unmarshal(page.Schemas[0], &got); err != nil {
		return fmt.Errorf("invalid JSON-LD: %w", err)
	}
	expectedJSON, err := json.Marshal(e.structuredData(*post))
	if err != nil {
		return err
	}
	var expected map[string]any
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		return err
	}
	if !reflect.DeepEqual(got, expected) {
		return fmt.Errorf("BlogPosting differs from source metadata")
	}
	if page.Canonical != expected["url"] {
		return fmt.Errorf("BlogPosting URL differs from canonical")
	}
	return nil
}

func (e *engine) checkSite(root, public string, now time.Time) error {
	_, posts, err := e.loadDocuments(root, now)
	if err != nil {
		return err
	}
	expected := map[string]*document{}
	for i := range posts {
		expected[posts[i].Route] = &posts[i]
	}
	pages := map[string]htmlPage{}
	var failures []string
	err = filepath.WalkDir(public, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch filepath.Ext(file) {
		case ".html":
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			page, err := readHTML(data)
			if err != nil {
				return err
			}
			pages[file] = page
		case ".xml":
			fileHandle, err := os.Open(file)
			if err != nil {
				return err
			}
			defer fileHandle.Close()
			decoder := xml.NewDecoder(fileHandle)
			for {
				_, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for file, page := range pages {
		relative, err := filepath.Rel(public, file)
		if err != nil {
			return err
		}
		route := publicRoute(filepath.ToSlash(relative))
		if page.Language != e.language {
			failures = append(failures, route+": incorrect language")
		}
		if !strings.HasPrefix(page.Canonical, e.baseURL+"/") {
			failures = append(failures, route+": invalid canonical")
		}
		schemaErr := e.checkSchema(page, expected[route])
		if expected[route] == nil && route != "/404.html" && !isAlias(route, posts) {
			schemaErr = e.checkNonPostSchema(page, route, posts)
		}
		if err := schemaErr; err != nil {
			failures = append(failures, route+": "+err.Error())
		}
		pageURL, _ := url.Parse(e.baseURL + route)
		for _, ref := range page.Refs {
			parsed, err := url.Parse(ref)
			if err != nil {
				failures = append(failures, route+": invalid reference "+ref)
				continue
			}
			resolved := pageURL.ResolveReference(parsed)
			if resolved.Host != pageURL.Host || resolved.Scheme != pageURL.Scheme {
				continue
			}
			dest, err := target(public, resolved)
			if err != nil {
				failures = append(failures, err.Error())
				continue
			}
			info, err := os.Stat(dest)
			if err != nil || !info.Mode().IsRegular() {
				failures = append(failures, route+": missing "+ref)
				continue
			}
			if destPage, ok := pages[dest]; ok && resolved.Fragment != "" && !destPage.IDs[resolved.Fragment] {
				failures = append(failures, route+": missing fragment "+ref)
			}
		}
	}
	for route, post := range expected {
		file := filepath.Join(public, strings.TrimPrefix(route, "/"), "index.html")
		if post.Meta.Cover != "" {
			values, err := e.coverMetadata(root, *post)
			if err != nil {
				return err
			}
			for key, value := range values {
				got := pages[file].HeadMetadata[key]
				if len(got) != 1 || got[0] != value {
					failures = append(failures, route+": missing, duplicate or incorrect "+key)
				}
			}
		}
		if _, ok := pages[file]; !ok {
			failures = append(failures, "missing post: "+route)
		}
		if image := e.coverURL(*post); image != "" {
			u, err := url.Parse(image)
			if err != nil {
				return err
			}
			file, err := target(public, u)
			if err != nil {
				return err
			}
			if _, err := os.Stat(file); err != nil {
				failures = append(failures, "missing schema image: "+image)
			}
		}
	}
	var legacy []byte
	if e.config.RequiredURLs != "" {
		legacy, err = os.ReadFile(filepath.Join(root, e.config.RequiredURLs))
	}
	if err != nil {
		return err
	}
	for _, route := range strings.Fields(string(legacy)) {
		u, err := url.Parse(route)
		if err != nil {
			return err
		}
		file, err := target(public, u)
		if err != nil {
			return err
		}
		if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
			failures = append(failures, "legacy URL missing: "+route)
		}
	}
	data, err := os.ReadFile(filepath.Join(public, "index.xml"))
	if err != nil {
		return err
	}
	var feed rss
	if err := xml.Unmarshal(data, &feed); err != nil {
		return err
	}
	if len(feed.Channel.Items) != len(posts) {
		failures = append(failures, "RSS item count differs from published posts")
	}
	for i, item := range feed.Channel.Items {
		if i >= len(posts) {
			break
		}
		if item.Link != e.baseURL+posts[i].Route || item.Title != posts[i].Meta.Title {
			failures = append(failures, "RSS order or metadata differs from posts")
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("site validation failed:\n%s", strings.Join(failures, "\n"))
	}
	fmt.Printf("Checked %d HTML pages, %d BlogPosting blocks, page JSON-LD, local links/fragments, legacy URLs and RSS\n", len(pages), len(posts))
	return nil
}
