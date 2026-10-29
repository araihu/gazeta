package gazeta

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/unicode/norm"
)

type metadata struct {
	Title       string   `toml:"title"`
	Description string   `toml:"description"`
	Language    string   `toml:"language"`
	Authors     []string `toml:"authors"`
	PublishedAt string   `toml:"publishedAt"`
	ModifiedAt  string   `toml:"modifiedAt"`
	Cover       string   `toml:"cover"`
	CoverAlt    string   `toml:"cover_alt"`
	Tags        []string `toml:"tags"`
	Aliases     []string `toml:"aliases"`
	Draft       bool     `toml:"draft"`
	Series      string   `toml:"series"`
	SeriesPart  int      `toml:"seriesPart"`
}

type document struct {
	SchemaItems         []schemaListItem
	Meta                metadata
	Front               map[string]any
	Body, Source, Route string
	Published           time.Time
}

func parseDocument(data []byte) (document, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "+++\n") {
		return document{}, fmt.Errorf("missing TOML frontmatter")
	}
	front, body, ok := strings.Cut(text[4:], "\n+++\n")
	if !ok {
		return document{}, fmt.Errorf("unclosed TOML frontmatter")
	}
	d := document{Body: strings.TrimLeft(body, "\n")}
	if err := toml.Unmarshal([]byte(front), &d.Meta); err != nil {
		return d, err
	}
	if err := toml.Unmarshal([]byte(front), &d.Front); err != nil {
		return d, err
	}
	if strings.TrimSpace(d.Meta.Title) == "" {
		return d, fmt.Errorf("missing title")
	}
	if (d.Meta.Series == "") != (d.Meta.SeriesPart == 0) || d.Meta.SeriesPart < 0 || (d.Meta.Series != "" && strings.TrimSpace(d.Meta.Series) == "") {
		return d, fmt.Errorf("series and a positive seriesPart must be set together")
	}
	var err error
	if d.Meta.PublishedAt != "" {
		d.Published, err = time.Parse(time.RFC3339, d.Meta.PublishedAt)
		if err != nil {
			return d, fmt.Errorf("publishedAt must be RFC 3339 with timezone: %w", err)
		}
	}
	if d.Meta.ModifiedAt != "" {
		modified, err := time.Parse(time.RFC3339, d.Meta.ModifiedAt)
		if err != nil {
			return d, fmt.Errorf("invalid modifiedAt: %w", err)
		}
		if modified.Before(d.Published) {
			return d, fmt.Errorf("modifiedAt precedes publishedAt")
		}
	}
	return d, nil
}

func (d document) visible(now time.Time) bool {
	return !d.Meta.Draft && (d.Published.IsZero() || !d.Published.After(now))
}

func loadDocuments(root string, now time.Time) ([]document, []document, error) {
	var docs, posts []document
	seriesParts := map[string]map[int]bool{}
	err := filepath.WalkDir(filepath.Join(root, "content"), func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("content symlink unsupported: %s", file)
		}
		if entry.IsDir() || filepath.Ext(file) != ".md" {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		d, err := parseDocument(data)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if !d.visible(now) {
			return nil
		}
		relative, err := filepath.Rel(filepath.Join(root, "content"), file)
		if err != nil {
			return err
		}
		d.Source = filepath.ToSlash(relative)
		if strings.HasPrefix(d.Source, "blogs/") {
			if path.Base(d.Source) != "index.md" || path.Dir(d.Source) == "blogs" {
				return fmt.Errorf("post must be blogs/<slug>/index.md: %s", file)
			}
			d.Route = "/" + path.Dir(d.Source) + "/"
			if d.Published.IsZero() || len(d.Meta.Authors) == 0 {
				return fmt.Errorf("%s: posts require publishedAt and authors", file)
			}
			for _, author := range d.Meta.Authors {
				if strings.TrimSpace(author) == "" {
					return fmt.Errorf("%s: empty author", file)
				}
			}
			if d.Meta.Cover != "" {
				cover := strings.TrimPrefix(d.Meta.Cover, "./")
				if !strings.HasPrefix(cover, "assets/") || !fs.ValidPath(cover) {
					return fmt.Errorf("%s: cover must be a local assets/ path", file)
				}
				info, err := os.Stat(filepath.Join(filepath.Dir(file), filepath.FromSlash(cover)))
				if err != nil {
					return fmt.Errorf("%s: cover: %w", file, err)
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("%s: cover is not a regular file", file)
				}
			}
			for _, tag := range d.Meta.Tags {
				if tagSlug(tag) == "" {
					return fmt.Errorf("%s: tag has no URL slug: %q", file, tag)
				}
			}
			posts = append(posts, d)
			if d.Meta.Series != "" {
				if seriesParts[d.Meta.Series] == nil {
					seriesParts[d.Meta.Series] = map[int]bool{}
				}
				if seriesParts[d.Meta.Series][d.Meta.SeriesPart] {
					return fmt.Errorf("duplicate part %d in series %s", d.Meta.SeriesPart, d.Meta.Series)
				}
				seriesParts[d.Meta.Series][d.Meta.SeriesPart] = true
			}
		}
		docs = append(docs, d)
		return nil
	})
	sort.Slice(posts, func(i, j int) bool {
		if posts[i].Published.Equal(posts[j].Published) {
			// Group equal-date series before ordering their parts to keep the comparator transitive.
			if posts[i].Meta.Series != posts[j].Meta.Series {
				return posts[i].Meta.Series > posts[j].Meta.Series
			}
			if posts[i].Meta.Series != "" {
				return posts[i].Meta.SeriesPart < posts[j].Meta.SeriesPart
			}
			return posts[i].Route > posts[j].Route
		}
		return posts[i].Published.After(posts[j].Published)
	})
	return docs, posts, err
}

var slugSeparators = regexp.MustCompile(`[^a-z0-9]+`)

func tagSlug(tag string) string {
	var ascii strings.Builder
	for _, r := range norm.NFKD.String(tag) {
		if r < 128 {
			ascii.WriteRune(r)
		}
	}
	return strings.Trim(slugSeparators.ReplaceAllString(strings.ToLower(ascii.String()), "-"), "-")
}

// publicationLinks declares destinations; Margo renders the metadata header.
func (e *engine) publicationLinks(m metadata) map[string]any {
	authors, tags := map[string]string{}, map[string]string{}
	for _, author := range m.Authors {
		if profile := e.authorURL(author); profile != "" {
			authors[author] = profile
		}
	}
	for _, tag := range m.Tags {
		tags[tag] = e.baseURL + "/tags/" + tagSlug(tag) + "/"
	}
	return map[string]any{"authors": authors, "tags": tags}
}

func (e *engine) listing(posts []document, source string) (string, error) {
	var cards strings.Builder
	fmt.Fprintf(&cards, `<ol class="blog-posts" aria-label="%s">`, html.EscapeString(e.config.Labels.Articles))
	for _, post := range posts {
		m := post.Meta
		cards.WriteString(`<li class="blog-post">`)
		if m.Cover != "" {
			asset := path.Join(strings.TrimPrefix(post.Route, "/"), m.Cover)
			src, err := filepath.Rel(filepath.FromSlash(path.Dir(source)), filepath.FromSlash(asset))
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&cards, `<a class="blog-post-image" href="%s" tabindex="-1" aria-hidden="true"><img src="%s" alt="%s" loading="lazy" width="640" height="360"></a>`, html.EscapeString(post.Route), html.EscapeString(filepath.ToSlash(src)), html.EscapeString(m.CoverAlt))
		}
		fmt.Fprintf(&cards, `<div class="blog-post-text"><time datetime="%s">%s</time><h2><a href="%s">%s</a></h2><p>%s</p></div></li>`, html.EscapeString(m.PublishedAt), post.Published.Format(e.dateFormat), html.EscapeString(post.Route), html.EscapeString(m.Title), html.EscapeString(m.Description))
	}
	cards.WriteString("</ol>\n")
	return cards.String(), nil
}

func (e *engine) writePage(stage string, d document) error {
	if d.Meta.Language == "" {
		d.Meta.Language = e.language
		d.Front["language"] = e.language
	}
	body := d.Body
	heading := "# " + d.Meta.Title + "\n\n"
	if strings.HasPrefix(body, "# ") {
		var first string
		first, body, _ = strings.Cut(body, "\n")
		heading = first + "\n\n"
	}
	class := "blog-content"
	if d.Source == "index.md" {
		class = "blog-home"
	}
	frontmatter := make(map[string]any, len(d.Front)+1)
	for key, value := range d.Front {
		frontmatter[key] = value
	}
	if d.Route != "" {
		frontmatter["publication_links"] = e.publicationLinks(d.Meta)
		if d.Meta.Cover != "" {
			alt := d.Meta.CoverAlt
			if alt == "" {
				alt = d.Meta.Title
			}
			frontmatter["social_image"] = map[string]string{"path": d.Meta.Cover, "alt": alt}
		}
		frontmatter["layout"] = map[string]any{
			"kind":   "docs",
			"values": map[string]any{"toc": true},
		}
	}
	if data := e.pageStructuredData(d); data != nil {
		frontmatter["structured_data"] = data
	}
	front, err := json.Marshal(frontmatter)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(stage, filepath.FromSlash(d.Source)), []byte("---\n"+string(front)+"\n---\n\n<div class=\""+class+"\">\n\n"+heading+body+"\n\n</div>\n"))
}

func (e *engine) loadDocuments(root string, now time.Time) ([]document, []document, error) {
	docs, posts, err := loadDocuments(root, now)
	for _, list := range [][]document{docs, posts} {
		for i := range list {
			if list[i].Meta.Language == "" {
				list[i].Meta.Language = e.language
				list[i].Front["language"] = e.language
			}
			if list[i].Meta.Language != e.language {
				return nil, nil, fmt.Errorf("%s: language must match locales.default", list[i].Source)
			}
		}
	}
	return docs, posts, err
}
