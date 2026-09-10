# Terp Lean docs (source of truth)

Product MDX for Terp Lean lives **here**, next to `lean-cw-ffi` (the C ABI that drives `terpz`).

`websites/terp-docs` does not own these pages. Vocs on groot-wan **imports this directory at compile time** via Vite (`@terpz-lean-docs` → `docs/pages`). Edit here; rebuild terp-docs.

Intended git home: https://github.com/terpnetwork/terpz (private). Until that remote is the worktree's `origin`, this tree is still the `feat/lean-v6` worktree under terp-core.

Layout:

```
docs/pages/index.mdx              → /research
docs/pages/status.mdx             → /research/status
docs/pages/timeline.mdx           → /research/timeline
docs/pages/glossary.mdx           → /research/glossary
docs/pages/verify.mdx             → /research/verify
docs/pages/resources.mdx          → /research/resources
docs/pages/for-*.mdx              → /research/for-*
docs/pages/protocol/*.mdx         → /research/protocol/*
```
