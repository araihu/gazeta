# Gazeta

Git-managed blogs on [Margo](https://github.com/araihu/margo). Gazeta owns publication rules. Margo renders Markdown, layouts, search, themes, browser metadata and JSON-LD.

Extracted from [Guigoes](https://github.com/guilycst/guigoes-v2), its first production consumer. Content, author identity and visual assets stay in each blog repository.

## Run a blog

Requires Go 1.27.3+. Copy `examples/minimal` into your blog repository and use the pinned CLI:

```sh
go run github.com/araihu/gazeta/cmd/gazeta@v0.0.1 build
go run github.com/araihu/gazeta/cmd/gazeta@v0.0.1 check
go run github.com/araihu/gazeta/cmd/gazeta@v0.0.1 serve
```

`serve` exposes the existing output at `http://127.0.0.1:1313`. Run `build` after editing. Each command accepts an optional publication root argument.

A Go consumer can pin `github.com/araihu/gazeta` in `go.mod` and call `gazeta.Run(os.Args[1:])`. For an injected build clock, use `gazeta.Build(ctx, root, now)` or `gazeta.Check(root, now)`. There is no global mutable site identity.

## Configuration

`site.yaml` is native Margo configuration. It is the single source for `site.name`, `site.description`, `site.base_url`, logos, icons, social fallback image, webmanifest, navigation, theme and custom CSS. Set the default locale and readable date format there too. See the runnable example for the complete required identity.

Gazeta requires `source: .build/content`, `output: public`, `site.home: index.md` and an origin without a path. One locale per publication is supported. Posts without `language` inherit `locales.default`; an explicit different locale fails validation.

`blog.yaml` holds editorial options:

```yaml
version: 1
home_limit: 4
authors:
  Alex Example:
    url: /about/
page_types:
  /about/: AboutPage
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

Authors match frontmatter names exactly. Profiles may use root-relative or HTTPS URLs. Unknown guests keep their names without inheriting another author's profile. An `AboutPage` matching one configured profile receives that author's `Person` data. `page_types` accepts `WebPage`, `AboutPage`, `ContactPage` and `CollectionPage`.

Labels default to English. Available keys are `articles`, `blog`, `tags`, `categories`, `categories_body`, `not_found`, `not_found_body`, `all_articles`, `series`, `part`, `series_index`, `continue_series`, `start_series`, `start_first`, `previous`, `next`, `redirecting` and `open_article`. The two `*_body` fields accept Markdown. `files` copies regular files or directories, rejects symlinks and refuses to overwrite a different generated file. It cannot copy content or build directories around visibility filtering. The optional `static/` directory supplies hosting files.

Markup uses neutral `.blog-home`, `.blog-content`, `.blog-posts`, `.blog-post`, `.blog-post-image`, `.blog-post-text`, `.blog-series`, `.blog-series-entry` and `.blog-series-pagination` classes. Style them through Margo's `custom_css`. Gazeta does not bundle a consumer's palette, logos or article artwork. `examples/minimal/assets/editorial.css` supplies a small starting layout.

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

Builds stage content in `.build/`, render through Margo, assemble public assets and use `site.MarshalManifest` for final byte identities. Validation checks canonical URLs, locale, JSON-LD, local links and fragments, social covers, RSS and configured legacy URLs before replacing `public/`. A failed build preserves the previous publication. A build lock prevents two writers on one root. After a crashed build, remove `.build/build.lock` only once its process has stopped.

## Trust and release scope

Configuration and Git content are trusted build inputs. Margo layout commands and modules inherit the build process's access. Gazeta is not an execution sandbox for submitted repositories.

Authored raw HTML is denied by default through Margo's compiler policy. `unsafe_html: true` explicitly permits reviewed HTML/JavaScript demos. Gazeta's own escaped listing and series markup renders in either mode. Do not enable this option for unreviewed content.

`v0.0.1` is an initial extraction, with domain-root hosting and one locale per blog. No content scheduler or provider-specific deploy credentials are included. A Git push can run the build and deploy `public/` through your existing pipeline.

## Verify and release

```sh
./scripts/check.sh
```

This runs formatting, module download, Go tests, vet, build and a standalone example publication. Integration tests exercise two independent domains, author profiles, consumer CSS, private content exclusion, explicit HTML opt-in and preservation after failed validation.

CI runs the same script. A `v*` tag must pass it and point to a commit on `main` before the workflow publishes a GitHub release. Consumers should pin a tested module release without a local `replace`.

GPL-3.0, matching the source publication. Margo keeps its own MIT license.
