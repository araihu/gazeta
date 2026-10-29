# v0.0.1

Gazeta extracts the editorial publication layer from Guigoes into a reusable Go module on Margo v0.0.29.

- Native Margo site configuration controls domain, branding, theme, icons and navigation.
- Blog configuration declares author profiles, localized labels, page types and legacy files.
- Draft/future filtering, series navigation, tag archives, RSS, aliases and final artifact manifests share one build.
- Validation runs before replacing the previous publication. Builds reject concurrent writers and authored raw HTML unless explicitly enabled.
- A standalone example and two-domain integration tests verify independent consumer identities.

This first release targets Git-managed, reviewed content and one locale per publication. Routes use `/blogs/<slug>/`; publication at a domain root is required. A future date becomes visible on the next build after that date.
