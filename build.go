package gazeta

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	margo "github.com/araihu/margo"
	"github.com/araihu/margo/site"
)

func writeFile(file string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("cannot copy symlink %s", file)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("not a regular file: %s", file)
		}
		relative, err := filepath.Rel(source, file)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(target, relative), data)
	})
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	Date        string `xml:"pubDate"`
}
type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language"`
	Items       []rssItem `xml:"item"`
}
type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

func (e *engine) writeFeed(file, title string, posts []document) error {
	feed := rss{Version: "2.0", Channel: rssChannel{Title: title, Link: e.baseURL + "/", Description: e.site.Site.Description, Language: e.language}}
	for _, post := range posts {
		feed.Channel.Items = append(feed.Channel.Items, rssItem{Title: post.Meta.Title, Link: e.baseURL + post.Route, GUID: e.baseURL + post.Route, Description: post.Meta.Description, Date: post.Published.Format("Mon, 02 Jan 2006 15:04:05 -0700")})
	}
	data, err := xml.Marshal(feed)
	if err != nil {
		return err
	}
	return writeFile(file, append([]byte(xml.Header), data...))
}

func (e *engine) generatedPage(stage, source, title, body string, items ...schemaListItem) error {
	if _, err := os.Stat(filepath.Join(stage, source)); err == nil {
		return fmt.Errorf("generated page collision: %s", source)
	} else if !os.IsNotExist(err) {
		return err
	}
	return e.writePage(stage, document{SchemaItems: items, Source: source, Body: body, Meta: metadata{Title: title, Language: e.language}, Front: map[string]any{"title": title, "language": e.language}})
}

func (e *engine) buildSite(root string, now time.Time) error {
	docs, posts, err := e.loadDocuments(root, now)
	if err != nil {
		return err
	}
	stage, render := filepath.Join(root, ".build/content"), filepath.Join(root, ".build/render")
	for _, dir := range []string{stage, render} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	for _, d := range docs {
		if !e.config.UnsafeHTML {
			front, err := json.Marshal(d.Front)
			if err != nil {
				return err
			}
			_, err = margo.New().Compile(e.ctx, margo.Source{Name: d.Source, Content: []byte("---\n" + string(front) + "\n---\n" + d.Body)})
			if err != nil {
				return fmt.Errorf("%s: %w", d.Source, err)
			}
		}
		sourceAssets := filepath.Join(root, "content", filepath.Dir(d.Source), "assets")
		if _, err := os.Stat(sourceAssets); err == nil {
			if err := copyTree(sourceAssets, filepath.Join(stage, filepath.Dir(d.Source), "assets")); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if d.Route != "" {
			index, pagination := e.seriesNavigation(d, posts)
			d.Body = index + d.Body + "\n\n" + pagination
		}
		if d.Route != "" && d.Meta.Cover != "" {
			alt := d.Meta.CoverAlt
			if alt == "" {
				alt = d.Meta.Title
			}
			d.Body = "![" + alt + "](" + d.Meta.Cover + ")\n\n" + d.Body
		}
		if d.Source == "index.md" {
			d.SchemaItems = e.postSchemaItems(posts[:min(e.config.HomeLimit, len(posts))])
			list, err := e.listing(posts[:min(e.config.HomeLimit, len(posts))], d.Source)
			if err != nil {
				return err
			}
			d.Body += "\n\n" + e.seriesEntry(posts) + "\n\n" + list + "\n\n[" + e.config.Labels.AllArticles + "](/blogs/)\n"
		}
		if err := e.writePage(stage, d); err != nil {
			return err
		}
	}
	list, err := e.listing(posts, "blogs/index.md")
	if err != nil {
		return err
	}
	if err := e.generatedPage(stage, "blogs/index.md", e.config.Labels.Blog, e.seriesEntry(posts)+"\n\n"+list, e.postSchemaItems(posts)...); err != nil {
		return err
	}
	type topic struct {
		label string
		posts []document
	}
	tags := map[string]*topic{}
	for _, post := range posts {
		for _, tag := range post.Meta.Tags {
			slug := tagSlug(tag)
			if tags[slug] == nil {
				tags[slug] = &topic{label: tag}
			}
			tags[slug].posts = append(tags[slug].posts, post)
		}
	}
	slugs := make([]string, 0, len(tags))
	for slug := range tags {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	var index strings.Builder
	for _, slug := range slugs {
		tag := tags[slug]
		source := "tags/" + slug + "/index.md"
		fmt.Fprintf(&index, "- [%s](/tags/%s/)\n", tag.label, slug)
		list, err := e.listing(tag.posts, source)
		if err != nil {
			return err
		}
		if err := e.generatedPage(stage, source, tag.label, list, e.postSchemaItems(tag.posts)...); err != nil {
			return err
		}
	}
	for _, page := range []struct{ source, title, body string }{
		{"tags/index.md", e.config.Labels.Tags, index.String()},
		{"categories/index.md", e.config.Labels.Categories, e.config.Labels.CategoriesBody},
		{"404.md", e.config.Labels.NotFound, e.config.Labels.NotFoundBody},
	} {
		if err := e.generatedPage(stage, page.source, page.title, page.body); err != nil {
			return err
		}
	}
	// Generated markup is trusted. Authored raw HTML requires explicit unsafe_html.
	// Site configuration and Git content are trusted build inputs, not a sandbox.
	result, err := site.BuildConfig(e.ctx, site.ConfigRequest{
		ConfigPath: filepath.Join(root, "site.yaml"), Compiler: margo.New(margo.WithUnsafeHTML()),
	})
	if err != nil {
		return err
	}
	for _, artifact := range result.Artifacts {
		if err := writeFile(filepath.Join(render, filepath.FromSlash(artifact.Path)), artifact.Content); err != nil {
			return err
		}
	}
	// Retain legacy image URLs even when Margo does not render a reference to them.
	for _, d := range docs {
		assets := filepath.Join(stage, filepath.Dir(d.Source), "assets")
		if _, err := os.Stat(assets); err == nil {
			if err := copyTree(assets, filepath.Join(render, filepath.Dir(d.Source), "assets")); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := copyOptionalTree(filepath.Join(root, "static"), render); err != nil {
		return err
	}
	for _, feed := range []struct {
		file, title string
		posts       []document
	}{
		{"index.xml", e.site.Site.Name, posts}, {"blogs/index.xml", e.site.Site.Name + " · " + e.config.Labels.Blog, posts},
		{"tags/index.xml", e.site.Site.Name + " · " + e.config.Labels.Tags, nil}, {"categories/index.xml", e.site.Site.Name + " · " + e.config.Labels.Categories, nil},
	} {
		if err := e.writeFeed(filepath.Join(render, feed.file), feed.title, feed.posts); err != nil {
			return err
		}
	}
	for _, slug := range slugs {
		if err := e.writeFeed(filepath.Join(render, "tags", slug, "index.xml"), e.site.Site.Name+" · "+tags[slug].label, tags[slug].posts); err != nil {
			return err
		}
	}
	if err := e.writeAliases(render, posts); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(render, "robots.txt"), []byte("User-agent: *\nAllow: /\nSitemap: "+e.baseURL+"/sitemap.xml\n")); err != nil {
		return err
	}
	if err := e.copyFiles(root, render); err != nil {
		return err
	}
	if err := writePublicationManifest(render, result); err != nil {
		return err
	}
	if err := e.checkSite(root, render, now); err != nil {
		return err
	}
	if err := publishDirectory(render, filepath.Join(root, "public")); err != nil {
		return err
	}
	fmt.Printf("Built %d posts with Margo into public/\n", len(posts))
	return nil
}

var aliasPattern = regexp.MustCompile(`^/[a-z0-9_/-]+/$`)

func (e *engine) writeAliases(render string, posts []document) error {
	var redirects []string
	for _, post := range posts {
		for _, alias := range post.Meta.Aliases {
			if !aliasPattern.MatchString(alias) || strings.Contains(alias, "..") {
				return fmt.Errorf("unsafe alias: %s", alias)
			}
			target := filepath.Join(render, strings.Trim(alias, "/"), "index.html")
			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("alias collision: %s", alias)
			} else if !os.IsNotExist(err) {
				return err
			}
			canonical := html.EscapeString(e.baseURL + post.Route)
			if err := writeFile(target, []byte(`<!doctype html><html lang="`+html.EscapeString(e.language)+`"><head><meta charset="utf-8"><title>`+html.EscapeString(e.config.Labels.Redirecting)+`</title><link rel="canonical" href="`+canonical+`"><meta http-equiv="refresh" content="0; url=`+canonical+`"></head><body><a href="`+canonical+`"`+html.EscapeString(e.config.Labels.OpenArticle)+`</a></body></html>`)); err != nil {
				return err
			}
			redirects = append(redirects, alias+" "+post.Route+" 301", strings.TrimSuffix(alias, "/")+" "+post.Route+" 301")
		}
	}
	return writeFile(filepath.Join(render, "_redirects"), []byte(strings.Join(redirects, "\n")+"\n"))
}

func publishDirectory(render, public string) error {
	backup := public + ".previous"
	if _, err := os.Stat(backup); err == nil {
		return fmt.Errorf("previous publication still exists: %s", backup)
	} else if !os.IsNotExist(err) {
		return err
	}
	hadPublic := false
	if _, err := os.Stat(public); err == nil {
		if err := os.Rename(public, backup); err != nil {
			return err
		}
		hadPublic = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(render, public); err != nil {
		if hadPublic {
			if restore := os.Rename(backup, public); restore != nil {
				return fmt.Errorf("publish: %v; restore %s: %w", err, backup, restore)
			}
		}
		return err
	}
	if hadPublic {
		return os.RemoveAll(backup)
	}
	return nil
}
