# Find-by-name API fixtures

The `*-live.json` files are anonymous JSON responses captured on 2026-11-06, without cookies or API keys.
They contain metadata only; no images were fetched. Tests replay these responses through `httptest` and
substitute the endpoint variables, so they require no external network.

- `f95-search-live.json`: F95 Latest Updates, `cmd=list&cat=games&page=1&rows=8&sort=date&search=Summertime`.
- `steam-search-live.json`: Steam storesearch, `term=Being a DIK&l=english&cc=US`.
- `vndb-search-live.json`: VNDB Kana `/vn`, search `Katawa Shoujo`, eight results, fields from the spec.
- `vndb-detail-live.json`: VNDB Kana `/vn`, ID `v945`, detail fields from the spec.

See `docs/specs/find-by-name.md` at the repository root for the exact endpoints and findings.
Malformed JSON, HTTP failures, missing/null fields, BBCode and image downloads use small synthetic
responses in the tests in addition to these recorded responses.

## Pornolab collection fixtures (2026-11-09)

`pornolab-collection-3006469.html` is reduced from the owner's saved first post of topic 3006469.
All Pornolab fixtures are Windows-1251 (including their charset declaration), with CRLF line endings.
The 24px heading, aligned `var.postImg`, bold field labels, `post-hr`, and raw
`sp-wrap > sp-body[title] > sp-title` structure follow the saved source. Titles, field values and
prose are neutral fictional data; image URLs use `media.example` and local `src` placeholders.
Scripts, page chrome, account details and original artwork are not included. Tests rewrite media
URLs to httptest endpoints and never request live artwork or Pornolab.

Derived cases: `three` adds a third game and non-game notes; `spoilers` wraps each game in its own
spoiler; `single` keeps one game and a normal release title; `ambiguous` has an untitled game and
two identical headings; `no-cover` removes the first poster. Screenshot counts preserve the need
for the 15-image limit. Synthetic tests cover rendered spoiler heads and hosting URL normalization.

The old pornolab1–6 local samples are gone. The single-game fixture is the required regression;
`TestPornolabOptionalDownloads` additionally checks new single-game `pornolab*.html` files
in the current user's Downloads directory, or reports a skip when none are present.
