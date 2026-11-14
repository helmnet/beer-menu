# Security policy

Beer Menu is a self-hosted tap list: a Go REST API with a SQLite database and a React admin
app and TV display served by nginx. This policy covers the code in this repo and the images
published from it. Test only against an instance you run yourself.

## Supported versions

Only the latest release is supported: the newest `vX.Y.Z` tag and its images on GHCR. Fixes
ship in a new release, so upgrade to it (see [Upgrade](README.md#upgrade)).

## Reporting a vulnerability

Report it privately through GitHub: open the repo's Security tab and choose "Report a
vulnerability", or go straight to
<https://github.com/helmnet/beer-menu/security/advisories/new>. Do not open a public issue,
pull request or discussion about it, and keep the details private until a fixed release is
out.

Please include:

- the affected version or commit;
- steps to reproduce;
- the impact: what an attacker can do, and what access they need.

Beer Menu is maintained by one person in their spare time. Reports are handled on a best
effort basis, with no fixed response or fix deadline.
