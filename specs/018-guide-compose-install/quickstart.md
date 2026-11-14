# Quickstart: validate the feature

## Automated

```sh
node --test guide/
sh .github/ci/changed-sides.test.sh
```

Expected: all pass. Rules covered: [contracts/guide-site.md](contracts/guide-site.md).

## By hand

Prerequisites: Docker with Compose; a folder that is not a checkout of the project.

0. Note the start time; SC-001 expects step 3 done in under 5 minutes from opening the page.
1. `node guide/build.mjs`, then open `guide/_site/install.html` in a browser.
2. Press **Copy** on the compose block. Expected: label shows "Copied".
3. In an empty folder, paste into `docker-compose.yaml`. Follow the page's remaining steps
   exactly (create `data`, write `.env`, `docker compose up -d`).
   Expected: http://localhost:8080/ shows first-run setup; http://localhost:8080/display loads.
4. Open a new terminal, run `docker compose up -d` again in that folder. Expected: no
   permission errors in `docker compose logs api` (PUID/PGID kept by `.env`).
5. Set `BEER_MENU_WEB_PORT=8081` in `.env` as the page says, run `docker compose up -d`.
   Expected: the app answers on 8081.
6. Turn off JavaScript and reload. Expected: compose text shown in full, selectable, no button.
7. Narrow the window to 320 px on `install.html`, then on `index.html` and one admin page such
   as `taps.html`. Expected: only code blocks scroll sideways, never the page (SC-005).
8. Open `guide/_site/index.html`. Expected: the GitHub repository link is visible without
   scrolling and opens https://github.com/helmnet/beer-menu.
9. `docker compose down` and delete the test folder.
