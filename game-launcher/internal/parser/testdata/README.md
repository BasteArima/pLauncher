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
