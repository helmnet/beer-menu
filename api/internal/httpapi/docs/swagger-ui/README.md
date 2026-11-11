# Vendored Swagger UI

These files are copied unchanged from the npm package `swagger-ui-dist` **5.34.0**
(Apache-2.0, released 2026-10-19). Do not edit them by hand; only `fetch.sh` writes them.

- Tarball: `https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-5.34.0.tgz`
- npm integrity:
  `sha512-H6tAIjr3XqE31PbkZl0OFya1ziRLDfTwh3J6OHdumPtPCLfjTnyLeagl/55kFNZcTOz9a9QI9WP0FXldwiGJag==`
- Files: `swagger-ui-bundle.js`, `swagger-ui.css`, `swagger-ui-bundle.js.LICENSE.txt`,
  `LICENSE`, `NOTICE`

## Why vendored

The API serves the docs page at `/api/docs` from files embedded with `go:embed`. The files must
be in the repo at compile time: the image works offline, and CI runs `go test` without Docker or
a download step. Only these five files are kept, not the whole package.

Dependabot cannot see this bundle. Upgrades are manual (below).

## Upgrade

1. In `fetch.sh`, set `version` to the new release and `integrity` to its npm sha512
   (`npm view swagger-ui-dist@<version> dist.integrity`).
2. Check the release is at least two weeks old (project release-age rule).
3. Run `./fetch.sh`. It refuses a tarball whose sha512 does not match.
4. Update the version and sha512 in this README.
5. Re-run the browser check in `specs/009-api-docs/quickstart.md` §3 (read-only UI, no CSP
   violations, no requests to another origin).
