package gazeta

import (
	"net/url"
	"path"
	"strings"
)

func (e *engine) authorURL(name string) string {
	profile := e.config.Authors[name].URL
	if strings.HasPrefix(profile, "/") {
		return e.baseURL + e.route(profile)
	}
	return profile
}

func (e *engine) pageType(route string) string {
	if kind := e.config.PageTypes[route]; kind != "" {
		return kind
	}
	localRoute := "/" + strings.TrimPrefix(strings.TrimPrefix(route, "/"), e.prefix)
	if localRoute == "/blogs/" || strings.HasPrefix(localRoute, "/tags/") {
		return "CollectionPage"
	}
	if kind := e.config.PageTypes[localRoute]; kind != "" {
		return kind
	}
	return "WebPage"
}

// pageStructuredData declares editorial meaning; Margo owns JSON-LD emission.
func (e *engine) pageStructuredData(d document) any {
	if d.Route != "" {
		options := map[string]any{"type": "BlogPosting"}
		if image := e.coverURL(d); image != "" {
			options["image"] = image
		}
		profiles := map[string]string{}
		types := map[string]string{}
		for _, author := range d.Meta.Authors {
			if e.authorType(author) != "Person" {
				types[author] = e.authorType(author)
			}
			if profile := e.authorURL(author); profile != "" {
				profiles[author] = profile
			}
		}
		if len(profiles) > 0 {
			options["author_urls"] = profiles
		}
		if len(types) > 0 {
			options["author_types"] = types
		}
		return options
	}
	if d.Source == "404.md" {
		return nil
	}
	route := publicRoute(strings.TrimSuffix(d.Source, path.Ext(d.Source)) + ".html")
	canonical := e.baseURL + (&url.URL{Path: route}).String()
	kind := e.pageType(route)
	page := map[string]any{
		"@context": "https://schema.org", "@type": kind, "@id": canonical + "#webpage", "url": canonical,
		"name": d.Meta.Title, "inLanguage": e.language, "isPartOf": map[string]string{"@id": e.baseURL + e.route("/") + "#website"},
	}
	if d.Meta.Description != "" {
		page["description"] = d.Meta.Description
	}
	for name := range e.config.Authors {
		if (kind == "AboutPage" || kind == "ProfilePage") && e.authorURL(name) == canonical {
			page["mainEntity"] = map[string]string{"@type": e.authorType(name), "@id": canonical + "#" + strings.ToLower(e.authorType(name)), "name": name, "url": canonical}
		}
	}
	if d.SchemaItems != nil {
		page["mainEntity"] = map[string]any{"@type": "ItemList", "numberOfItems": len(d.SchemaItems), "itemListElement": d.SchemaItems}
	}
	if d.Source == e.source("index.md") {
		delete(page, "@context")
		return map[string]any{"@context": "https://schema.org", "@graph": []any{
			map[string]any{"@type": "WebSite", "@id": e.baseURL + e.route("/") + "#website", "url": e.baseURL + e.route("/"), "name": e.site.Site.Name, "inLanguage": e.language}, page,
		}}
	}
	return page
}

type schemaListItem struct {
	Type     string `json:"@type"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	URL      string `json:"url"`
}

func (e *engine) postSchemaItems(posts []document) []schemaListItem {
	items := make([]schemaListItem, 0, len(posts))
	for _, post := range posts {
		items = append(items, schemaListItem{Type: "ListItem", Position: len(items) + 1, Name: post.Meta.Title, URL: e.baseURL + post.Route})
	}
	return items
}

func (e *engine) coverURL(d document) string {
	if d.Meta.Cover == "" {
		return ""
	}
	return e.baseURL + (&url.URL{Path: path.Join(d.Route, d.Meta.Cover)}).String()
}

// publicRoute identifies directory indexes by basename, not by a filename suffix.
// It mirrors the publication's Margo directory-index and standalone HTML URLs.
func publicRoute(artifact string) string {
	if path.Base(artifact) == "index.html" {
		directory := path.Dir(artifact)
		if directory == "." {
			return "/"
		}
		return "/" + directory + "/"
	}
	return "/" + artifact
}
