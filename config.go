// Package gazeta builds Git-managed publications on Margo. Content and configuration
// belong to each consumer; rendering and document metadata belong to Margo.
package gazeta

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/araihu/margo/site"
	"go.yaml.in/yaml/v3"
)

type author struct {
	URL string `yaml:"url"`
}
type fileCopy struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}
type labels struct {
	Articles       string `yaml:"articles"`
	Blog           string `yaml:"blog"`
	Tags           string `yaml:"tags"`
	Categories     string `yaml:"categories"`
	CategoriesBody string `yaml:"categories_body"`
	NotFound       string `yaml:"not_found"`
	NotFoundBody   string `yaml:"not_found_body"`
	AllArticles    string `yaml:"all_articles"`
	Series         string `yaml:"series"`
	Part           string `yaml:"part"`
	SeriesIndex    string `yaml:"series_index"`
	ContinueSeries string `yaml:"continue_series"`
	StartSeries    string `yaml:"start_series"`
	StartFirst     string `yaml:"start_first"`
	Previous       string `yaml:"previous"`
	Next           string `yaml:"next"`
	Redirecting    string `yaml:"redirecting"`
	OpenArticle    string `yaml:"open_article"`
}
type config struct {
	Version      int               `yaml:"version"`
	Authors      map[string]author `yaml:"authors"`
	PageTypes    map[string]string `yaml:"page_types"`
	HomeLimit    int               `yaml:"home_limit"`
	UnsafeHTML   bool              `yaml:"unsafe_html"`
	RequiredURLs string            `yaml:"required_urls"`
	Files        []fileCopy        `yaml:"files"`
	Labels       labels            `yaml:"labels"`
}

// engine is private to a build. No mutable package-wide blog identity is used.
type engine struct {
	config                        config
	site                          site.Config
	baseURL, language, dateFormat string
	ctx                           context.Context
}

func defaultConfig() config {
	return config{Version: 1, HomeLimit: 4, Labels: labels{
		Articles: "Articles", Blog: "Blog", Tags: "Tags", Categories: "Categories",
		CategoriesBody: "Posts are organized by [tags](/tags/).",
		NotFound:       "Page not found", NotFoundBody: "This address does not exist. [Return to the blog](/blogs/).",
		AllArticles: "All articles", Series: "Series", Part: "Part", SeriesIndex: "Series index",
		ContinueSeries: "Continue the series", StartSeries: "Start the series", StartFirst: "Start with part 1",
		Previous: "Previous", Next: "Next", Redirecting: "Redirecting", OpenArticle: "Open article",
	}}
}

func loadEngine(root string) (*engine, error) {
	c := defaultConfig()
	data, err := os.ReadFile(filepath.Join(root, "blog.yaml"))
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return nil, fmt.Errorf("blog.yaml: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("blog.yaml must contain one YAML document")
	}
	if c.Version != 1 || c.HomeLimit < 1 {
		return nil, fmt.Errorf("blog.yaml requires version: 1 and positive home_limit")
	}
	s, err := site.LoadConfig(filepath.Join(root, "site.yaml"))
	if err != nil {
		return nil, err
	}
	if s.Source != ".build/content" || s.Output != "public" || s.Site.Home != "index.md" || s.BasePath != "" {
		return nil, fmt.Errorf("site.yaml requires source: .build/content, output: public, site.home: index.md and no base_path")
	}
	base := strings.TrimRight(s.Site.BaseURL, "/")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("site.base_url must be an absolute HTTP(S) origin")
	}
	if s.Site.Name == "" || s.Locales.Default == "" {
		return nil, fmt.Errorf("site.name and locales.default are required")
	}
	e := &engine{config: c, site: s, baseURL: base, language: s.Locales.Default, dateFormat: s.Publication.DateFormat, ctx: context.Background()}
	if e.dateFormat == "" {
		e.dateFormat = "2006-01-02"
	}
	for name, a := range c.Authors {
		profile, err := url.Parse(a.URL)
		if strings.TrimSpace(name) == "" || err != nil || a.URL == "" || profile.User != nil || strings.ContainsAny(a.URL, "\\\x00\r\n") {
			return nil, fmt.Errorf("invalid author profile: %q", name)
		}
		if strings.HasPrefix(a.URL, "/") && !strings.HasPrefix(a.URL, "//") && profile.Host == "" {
			continue
		}
		if profile.Scheme != "https" || profile.Host == "" {
			return nil, fmt.Errorf("author %q requires a root-relative or HTTPS URL", name)
		}
	}
	profiles := map[string]string{}
	for name := range c.Authors {
		profile := e.authorURL(name)
		route := strings.TrimPrefix(profile, e.baseURL)
		if kind := c.PageTypes[route]; kind == "AboutPage" || kind == "ProfilePage" {
			if previous, exists := profiles[profile]; exists {
				return nil, fmt.Errorf("%s %s must identify one author, got %s and %s", kind, route, previous, name)
			}
			profiles[profile] = name
		}
	}
	for route, kind := range c.PageTypes {
		if !strings.HasPrefix(route, "/") || (!strings.HasSuffix(route, "/") && !strings.HasSuffix(route, ".html")) || strings.Contains(route, "..") || strings.HasPrefix(route, "//") || strings.ContainsAny(route, "\\?#\r\n\t") {
			return nil, fmt.Errorf("invalid page route: %s", route)
		}
		switch kind {
		case "WebPage", "AboutPage", "ProfilePage", "ContactPage", "CollectionPage":
		default:
			return nil, fmt.Errorf("unsupported page type: %s", kind)
		}
		if kind == "ProfilePage" && profiles[e.baseURL+route] == "" {
			return nil, fmt.Errorf("ProfilePage %s requires one matching configured author URL", route)
		}
	}
	if c.RequiredURLs != "" && !fs.ValidPath(c.RequiredURLs) {
		return nil, fmt.Errorf("required_urls must be a local file")
	}
	for _, f := range c.Files {
		if !fs.ValidPath(f.Source) || !fs.ValidPath(f.Target) || f.Source == "." || f.Target == "." {
			return nil, fmt.Errorf("file copies require local source and target paths")
		}
		if first := strings.Split(f.Source, "/")[0]; strings.HasPrefix(first, ".") || first == "content" || first == "public" {
			return nil, fmt.Errorf("file copies cannot bypass publication visibility: %s", f.Source)
		}
	}
	for _, name := range []string{".build", "public", "content", "assets", "static"} {
		info, err := os.Lstat(filepath.Join(root, name))
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink unsupported: %s", name)
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return e, nil
}

// Build renders and validates a publication before replacing root/public.
// now controls draft/future visibility. Only reviewed Git content should be built.
func Build(ctx context.Context, root string, now time.Time) error {
	e, err := loadEngine(root)
	if err != nil {
		return err
	}
	e.ctx = ctx
	if err := os.MkdirAll(filepath.Join(root, ".build"), 0755); err != nil {
		return err
	}
	lock := filepath.Join(root, ".build/build.lock")
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("publication build locked, remove %s only after its process has stopped: %w", lock, err)
	}
	f.Close()
	defer os.Remove(lock)
	if err := ctx.Err(); err != nil {
		return err
	}
	return e.buildSite(root, now)
}

// Check validates an existing root/public against its content and configuration.
func Check(root string, now time.Time) error {
	e, err := loadEngine(root)
	if err != nil {
		return err
	}
	return e.checkSite(root, filepath.Join(root, "public"), now)
}
