# Gazeta

Git-managed blogs on [Margo](https://github.com/araihu/margo). Gazeta owns publication rules. Margo renders Markdown, layouts, search, themes, browser metadata and JSON-LD.

Extracted from [Guigoes](https://github.com/guilycst/guigoes-v2), its first production consumer. Content, author identity and visual assets stay in each blog repository.

## Run a blog

Requires Go 1.27.3+. Copy `examples/minimal` into your blog repository and use the pinned CLI:

```sh
go run github.com/araihu/gazeta/cmd/gazeta@v0.0.4 build
go run github.com/araihu/gazeta/cmd/gazeta@v0.0.4 check
go run github.com/araihu/gazeta/cmd/gazeta@v0.0.4 serve
```

`serve` exposes the existing output at `http://127.0.0.1:1313`. Run `build` after editing. Each command accepts an optional publication root argument.

A Go consumer can pin `github.com/araihu/gazeta` in `go.mod` and call `gazeta.Run(os.Args[1:])`. For an injected build clock, use `gazeta.Build(ctx, root, now)` or `gazeta.Check(root, now)`. There is no global mutable site identity.

## Configuration

`site.yaml` is native Margo configuration. It is the single source for `site.name`, `site.description`, `site.base_url`, logos, icons, social fallback image, webmanifest, navigation, theme and custom CSS. Set the default locale and readable date format there too. See the runnable example for the complete required identity.

Gazeta requires `source: .build/content`, `output: public`, `site.home: index.md` and an origin without a path. Declare languages in `locales.supported`. Default-language sources stay at the content root; other sources use lowercase locale directories such as `content/pt-br/` and `content/es/`. Posts inherit the locale of their path; an explicit conflicting `language` fails validation. Each translation keeps its own canonical URL and JSON-LD language.

`blog.yaml` holds editorial options:

```yaml
version: 1
home_limit: 4
authors:
  Alex Example:
    url: /about/
page_types:
  /about/: ProfilePage
  /contact/: ContactPage
  /projects/: CollectionPage
# Optional compatibility checks and additional public files:
required_urls: tests/legacy-urls.txt
files:
  - {source: assets/brand/favicon.ico, target: favicon.ico}
labels:
  articles: Articles
  all_articles: All articles
```

Authors match frontmatter names exactly. An optional `type: Organization` declares an institutional author; omitted types remain `Person`. Organization biographies use `AboutPage`, while `ProfilePage` remains restricted to one person. Profiles may use root-relative or HTTPS URLs. Root-relative profiles follow the current locale prefix; provide the corresponding biography in each locale. External HTTPS profiles retain their configured URL. Unknown guests keep their names without inheriting another author's profile. A `ProfilePage` requires exactly one configured author whose profile URL matches the page. It describes that author as `mainEntity`. Generic `AboutPage` behavior remains available. `page_types` accepts `WebPage`, `AboutPage`, `ProfilePage`, `ContactPage` and `CollectionPage`, keyed by directory routes or standalone `.html` routes. See [structured data](docs/structured-data.md) for the schema contract and live validation procedure.

Labels default to English. Available keys are `articles`, `blog`, `tags`, `categories`, `categories_body`, `not_found`, `not_found_body`, `all_articles`, `series`, `part`, `series_index`, `continue_series`, `start_series`, `start_first`, `previous`, `next`, `redirecting` and `open_article`. The two `*_body` fields accept Markdown. `files` copies regular files or directories, rejects symlinks and refuses to overwrite a different generated file. It cannot copy content or build directories around visibility filtering. The optional `static/` directory supplies hosting files.

Markup uses neutral `.blog-home`, `.blog-content`, `.blog-posts`, `.blog-post`, `.blog-post-image`, `.blog-post-text`, `.blog-series`, `.blog-series-entry` and `.blog-series-pagination` classes. Style them through Margo's `custom_css`. Gazeta does not bundle a consumer's palette, logos or article artwork. `examples/minimal/assets/editorial.css` supplies a small starting layout.

## Multilingual publications

One publication has independent home pages, archives, tags, RSS feeds and series
for each configured locale. Translate the same relative source path, for example
`content/blogs/welcome/index.md`, `content/pt-br/blogs/welcome/index.md` and
`content/es/blogs/welcome/index.md`. Matching paths identify equivalents; no
translation is inferred from article titles, dates or tags. Missing translations
remain absent. Drafts and future posts contribute neither equivalent links nor
assets. Series names and part numbers are unique within their locale, and series
navigation never crosses languages.

`blog.yaml` accepts locale-specific label overrides:

```yaml
locales:
  pt-BR:
    articles: Artigos
    blog: Artigos
    all_articles: Ver todos os artigos
    categories_body: Encontre publicações por [assunto](/pt-br/tags/).
```

These fields have the same keys as `labels`; unspecified values inherit the
shared labels. Translate all visible editorial copy and use locale-specific URLs
in Markdown labels. Margo localizes native English, pt-BR and Spanish controls.
Header/footer destinations in `site.yaml` accept `locale`, for example
`{label: Sobre, href: /pt-br/about/, locale: pt-BR}`. Shared links omit it.

The native Margo head includes self and reciprocal `hreflang` links for visible,
indexable equivalents. `x-default` points to the equivalent default-language page
when available; it is omitted when that page does not exist. A single sitemap
covers every indexable canonical page, while `/index.xml`, `/pt-br/index.xml` and
`/es/index.xml` each contain only their own locale. The root 404 document uses the
default language and stays out of discovery. Gazeta validates final equivalents,
canonical identities, schema and locale feeds before replacing the publication.
No browser-language or IP routing is added.

## Content and publication

Posts live in `content/blogs/<slug>/index.md`. Other Markdown pages live under `content/`. TOML frontmatter uses `+++` delimiters:

```toml
+++
title = "Our first post"
description = "What we learned."
authors = ["Alex Example"]
publishedAt = "2026-10-29T09:00:00Z"
tags = ["Engineering"]
cover = "./assets/cover.png"
cover_alt = "A description of the article illustration."
# Optional: modifiedAt, aliases, draft, series, seriesPart
+++
```

Timestamps require RFC 3339 with a timezone. `modifiedAt` cannot precede `publishedAt`. A draft or future post and its assets stay out of pages, lists, sitemap and feeds. Future posts need another build after their timestamp; Gazeta does not schedule one.

Series use `series` plus a positive, unique `seriesPart`. Navigation only includes visible parts. The newest series gets a first-part entry when that part is available. Post ordering uses publication dates, then series and part, without changing timestamps.

Gazeta generates `/blogs/`, `/tags/`, `/categories/`, `404.html`, RSS, safe redirect stubs and Cloudflare-compatible `_redirects`. These generated page names are reserved. Root home and page content stay consumer-owned. Covers must be real local `assets/` files under the post. Native Margo social metadata and `BlogPosting.image` use the same cover; posts without a cover omit the JSON-LD image.

Builds stage content in `.build/`, render through Margo, assemble public assets and use `site.MarshalManifest` for final byte identities. Validation checks canonical URLs, locale, JSON-LD, local links and fragments, social covers, RSS and configured legacy URLs before replacing `public/`. `check` also verifies the existing artifact bytes against their native Margo manifest. A failed build preserves the previous publication. A build lock prevents two writers on one root. After a crashed build, remove `.build/build.lock` only once its process has stopped.

## Verify a deployed publication

After publishing a validated `public/` artifact, run:

```sh
go run github.com/araihu/gazeta/cmd/gazeta@v0.0.4 verify -timeout 3m
```

`verify` reads the existing artifact and `site.base_url`; it never rebuilds or deploys. It reuses Margo's manifest serializer to check local artifact integrity, then requires the public manifest to match before and after each complete pass. It checks every public asset, HTML content, canonical URL, locale, JSON-LD, social metadata, legacy HTTP redirects and missing-page status. Hosting control files such as `_headers` and `_redirects` are not requested as public resources.

Non-HTML bytes must match exactly. HTML comparison allows JSON object formatting changes and the known hidden Cloudflare protection script, while retaining all publication content and metadata. This normalization changes comparison trees only. It never rewrites generated or served files. Redirects to unrelated destinations, stale content, duplicated JSON properties and soft 404 pages fail verification.

The default total deadline is two minutes, with five seconds between failed passes. Use `-timeout` and `-interval` to change these limits. A successful command emits a JSON receipt with origin, manifest digest, page/asset/redirect counts and attempts. Keep the same validated artifact in the verification job; a fresh build could describe a different publication. Google Rich Results Test remains a separate external check.

Go callers can use `gazeta.Verify(ctx, root, gazeta.VerifyOptions{})`. Options may supply a custom HTTP client, for example one trusting a private CA; verification still disables automatic redirects.

## Indexing and sitemaps

Gazeta marks the generated 404 document with Margo's native `noindex: true` frontmatter. Margo emits robots metadata and leaves the error document out of `sitemap.xml` and `llms.txt`; missing URLs still need the hosting platform's HTTP 404 behavior.

`gazeta check` requires the sitemap to contain each indexable canonical HTML page exactly once. It rejects missing pages, duplicate or unknown URLs, and pages marked `noindex`. Legacy aliases continue to use HTTP redirects and canonical targets instead of separate sitemap entries. The generated 404 document must retain its native `noindex, follow` metadata.

## Trust and release scope

Configuration and Git content are trusted build inputs. Margo layout commands and modules inherit the build process's access. Gazeta is not an execution sandbox for submitted repositories.

Authored raw HTML is denied by default through Margo's compiler policy. `unsafe_html: true` explicitly permits reviewed HTML/JavaScript demos. Gazeta's own escaped listing and series markup renders in either mode. Do not enable this option for unreviewed content.

`v0.0.x` is an initial extraction, with domain-root hosting and locale-scoped content. No content scheduler or provider-specific deploy credentials are included. A Git push can run the build and deploy `public/` through your existing pipeline.

## Repository language

Write repository documentation, GoDoc, source-code identifiers, code comments and developer-facing tooling messages in English, including upstream contributions. Published content and UI labels follow the configured locale. Preserve intentional localization/Unicode fixtures and compatibility-sensitive values.

## Verify and release

```sh
./scripts/check.sh
```

This runs formatting, module download, Go tests, vet, build and a standalone example publication. Integration tests exercise two independent domains, author profiles, consumer CSS, private content exclusion, explicit HTML opt-in and preservation after failed validation.

CI runs the same script. A `v*` tag must pass it and point to a commit on `main` before the workflow publishes a GitHub release. Consumers should pin a tested module release without a local `replace`.

GPL-3.0, matching the source publication. Margo keeps its own MIT license.
