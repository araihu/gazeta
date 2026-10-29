# Structured data

Gazeta declares editorial meaning through Margo's native `structured_data` API. Margo serializes one safely escaped JSON-LD block in the document head. Gazeta does not rewrite generated HTML to add schema.

## Page contracts

- Published posts use native `BlogPosting`. Headline, description, authors, language and timestamps come from the visible source metadata. Author URLs come from `blog.yaml`. A real cover supplies `image`; no cover means no article image is invented.
- Home contains `WebSite` and `WebPage` with the visible post list. Site name and URL come from native `site.yaml` identity.
- Archives use `CollectionPage` and an `ItemList` matching the visible posts and order.
- Explicit `ProfilePage` pages identify one configured author as `mainEntity`, with `Person`, name, URL and a stable page-local identifier. The profile URL must match the page. Missing or ambiguous author binding fails configuration.
- `AboutPage`, `ContactPage` and plain `WebPage` describe authored pages. An `AboutPage` with one matching author still includes that person for compatibility.
- Alias stubs and 404 omit JSON-LD. Draft and future posts remain absent from the publication.

Directory `index.md` pages use directory URLs. Other `.md` pages use `.html` URLs, matching Margo's actual canonical output. A filename such as `myindex.md` remains `myindex.html`.

```yaml
authors:
  Alex Example:
    url: /about/
page_types:
  /about/: ProfilePage
  /authors/guest.html: WebPage
```

Profile pages must actually focus on the configured author. Do not label a general archive, team page or home listing as an individual profile. Gazeta does not invent a profile portrait, social accounts, interaction counts, publication dates or publisher identity. Supply only information represented by the content. In particular, a logo is not an author's portrait or an article illustration.

## Google validation

[Google's Article documentation](https://developers.google.com/search/docs/appearance/structured-data/article) currently lists recommended properties, with no required properties. Gazeta provides applicable source fields; it does not fabricate `dateModified` or a publisher to remove hypothetical warnings. Timestamps retain their timezone. Each series part keeps its own canonical URL.

Google recommends [ProfilePage markup](https://developers.google.com/search/docs/appearance/structured-data/profile-page) for an internal author biography. An About Me page on a blog is an explicit valid use case. The required data is a `mainEntity` describing the person or organization and its name. Gazeta currently models individual authors as `Person`.

[Site name documentation](https://developers.google.com/search/docs/appearance/site-names) calls for home `WebSite.name` and `url`, both supplied by Gazeta. The Rich Results Test only reports supported rich-result features. A home or generic archive can therefore show `No items detected` while retaining valid WebSite or CollectionPage data. Do not add article, FAQ, review or breadcrumb markup to those pages merely to change that message.

For each deployment:

1. Run `./scripts/check.sh` and the consumer's publication checks. These validate the final HTML against content and configured identities, including profile drift and standalone page routes.
2. Submit the deployed canonical URLs to [Rich Results Test](https://search.google.com/test/rich-results) in URL mode. Check crawl success, detected type, errors and non-critical issues. Record the actual tested URL and shareable result URL.
3. Inspect an article, its author profile, home and archives. For a site-wide audit, test each published article. Confirm real images and visible content match the data.

A local test or pasted-code test does not establish that Google can fetch a live page. A successful Rich Results Test does not guarantee indexing or enhanced appearance in search, as explained in [Google's general guidelines](https://developers.google.com/search/docs/appearance/structured-data/sd-policies).
