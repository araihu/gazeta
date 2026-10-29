package gazeta

import "github.com/araihu/margo/site"

var testBlog = &engine{
	config: config{HomeLimit: 4, Authors: map[string]author{"Alex Example": {URL: "/about/"}}, PageTypes: map[string]string{"/about/": "AboutPage", "/contact/": "ContactPage", "/projects/": "CollectionPage"}, Labels: defaultConfig().Labels},
	site:   site.Config{Site: site.SiteConfig{Name: "Example Journal"}}, baseURL: "https://journal.example", language: "pt-BR", dateFormat: "02/01/2006",
}
