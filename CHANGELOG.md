## v0.0.5

- Support native locale-scoped pages, archives, tags, RSS and series in one publication.
- Validate reciprocal/self hreflang, default-language fallback and per-locale identity.
- Support explicit Organization authors and institutional AboutPage metadata.
- Test three locales, missing translations, private content and successive identity changes.

## v0.0.4

- Use native Margo noindex metadata for the generated 404 page and omit it from sitemap and llms.txt.
- Validate exact sitemap coverage of indexable canonical pages before publication.

## v0.0.3

- Validate local artifact integrity through Margo in `gazeta check`.
- Add `gazeta verify` and a Go API for validating a deployed artifact against its local Margo manifest.
- Check public HTML/schema, exact asset bytes, legacy redirects and missing-page HTTP status with bounded propagation retries.
- Emit a JSON verification receipt without rebuilding or publishing.

# v0.0.2

- Add opt-in author ProfilePage data with one matching configured author. Generic AboutPage pages remain supported.
- Correct JSON-LD and validation URLs for standalone Markdown pages, including filenames ending in myindex.md.
- Document current Google recommendations and distinguish Schema.org data from supported rich results.

# v0.0.1

Gazeta extracts the editorial publication layer from Guigoes into a reusable Go module on Margo v0.0.29.

- Native Margo site configuration controls domain, branding, theme, icons and navigation.
- Blog configuration declares author profiles, localized labels, page types and legacy files.
- Draft/future filtering, series navigation, tag archives, RSS, aliases and final artifact manifests share one build.
- Validation runs before replacing the previous publication. Builds reject concurrent writers and authored raw HTML unless explicitly enabled.
- A standalone example and two-domain integration tests verify independent consumer identities.

This first release targets Git-managed, reviewed content and one locale per publication. Routes use `/blogs/<slug>/`; publication at a domain root is required. A future date becomes visible on the next build after that date.
