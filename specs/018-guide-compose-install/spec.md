# Feature Specification: Install From a Compose File, Without Cloning

**Feature Branch**: `compmodder26/user_guide_modifications`

**Created**: 2026-11-14

**Status**: Draft

**Input**: User description: "I want to make some modifications to the user guide.  People shouldn't have to clone the repo to run it.  I think instead of giving them steps to clone the repo and changing to the beer-menu directory, we should put a barebones docker-compose.yaml file embedded in the page (with ability to one click copy).  After the basic instructions, we should then have a section for documenting environment variables.  Also, under the Home section, I'd like a link to the github repo."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start Beer Menu from a copied compose file (Priority: P1)

A home-bar owner who has Docker with Compose but has never used git opens the guide's "Install with Docker" page. Instead of downloading or cloning the project, they see a short compose file shown on the page. They press a "Copy" button next to it, make a new folder, save the copied text there as `docker-compose.yaml`, create the data folder, and start Beer Menu with the commands on the page. They then open the admin app and the TV display.

**Why this priority**: This is the change the owner asked for first. Today the page sends people to clone or download the whole project, which they do not need to run it.

**Independent Test**: On a machine with Docker and Compose and no copy of the project, follow only the Install with Docker page: copy the compose file with the button, save it, run the listed commands, and open the admin app and the TV display.

**Acceptance Scenarios**:

1. **Given** a user opens the Install with Docker page, **When** they read the steps, **Then** no step asks them to clone the project, download its files, or change into the project's folder.
2. **Given** the page shows the compose file, **When** the user presses its Copy button, **Then** the full compose file text is on their clipboard, exactly as shown, and the button confirms the copy (for example, its label changes to "Copied" for a moment).
3. **Given** a user saved the copied text as `docker-compose.yaml` in an empty folder, **When** they run the start commands on the page, **Then** Beer Menu starts from the published images and the admin app and TV display open at the addresses the page gives.
4. **Given** the compose file on the page, **When** a user reads it, **Then** it contains only what a first start needs (the two services, the published images, the data folder, the user to run as, the web port and the restart rule) and no build steps or commented-out optional settings.

---

### User Story 2 - Look up an environment variable (Priority: P2)

After Beer Menu is running, a user wants to change something: the web port, the data folder, the first admin account, plain-HTTP access from other devices, Brewfather and authenticator-app encryption, or passkeys. Below the basic install steps on the same page, a section lists every setting they can change through environment variables. For each one it says what it does, its default, whether it is needed, and how to set it.

**Why this priority**: The basic install works without any of these, but users need them to go beyond a first try (other devices, Brewfather, passkeys).

**Independent Test**: Compare the section with the settings the app reads at startup and the compose file reads; every user-settable one is listed with name, purpose, default and how to set it.

**Acceptance Scenarios**:

1. **Given** a user scrolls past the basic install steps, **When** they reach the environment variables section, **Then** it lists each variable in the table under FR-006 with its purpose, default value and whether it is optional.
2. **Given** a user wants to set a variable, **When** they read the section, **Then** it shows where to set it (in the compose file's settings for the service, or in the shell or an env file before starting) and that Beer Menu must be restarted to pick it up.
3. **Given** a variable holds a secret (the first admin password or the encryption key), **When** the user reads its entry, **Then** the guide tells them not to write the value into the compose file and shows the safer way to supply it.
4. **Given** a user reads the entry for the encryption key, **When** they follow it, **Then** they learn how to create the key and that losing or changing it disconnects Brewfather and stops authenticator-app codes.

---

### User Story 3 - Find the project from the Home page (Priority: P3)

A user on the guide's Home page wants the source code, to report a problem, or to see releases. The Home page has a clear link to the project's GitHub repository.

**Why this priority**: Small, useful, and independent of the install changes.

**Independent Test**: Open the guide's Home page and follow the repository link.

**Acceptance Scenarios**:

1. **Given** a user opens the guide's Home page, **When** they read it, **Then** they find a link, labeled as the project's GitHub repository, that opens https://github.com/helmnet/beer-menu.

---

### Edge Cases

- The browser blocks clipboard access (for example, the page is opened over plain HTTP or permission is denied): the button says the copy failed, and the compose text stays selectable so the user can copy it by hand.
- Scripts are turned off: the compose file is still shown in full and can be selected and copied by hand; only the button is missing or inert.
- The user already has something on port 8080: the environment variables section shows how to choose another web port, and the page's addresses note that the port changes with it.
- The data folder does not exist or is owned by another user: the steps create it as the user's own before starting, as today; the page keeps the note that Beer Menu refuses to run as root.
- The compose file on the page drifts from the project's own example compose file: a reviewer can compare the two; the guide's file stays the trimmed, image-only version.
- A narrow phone screen: the compose file block scrolls inside itself sideways if a line is too long, without making the whole page scroll sideways.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Install with Docker page MUST NOT ask the user to clone the project, download its files, or change into a copy of the project's files.
- **FR-002**: The Install with Docker page MUST show a complete compose file inline that starts Beer Menu from the published images, as described in User Story 1, scenario 4.
- **FR-003**: The shown compose file MUST use the same defaults as the project's example compose file (its contents are listed in User Story 1, scenario 4).
- **FR-004**: The compose file block MUST have a one-click Copy button that copies the exact text shown, confirms success, and reports failure; the text MUST remain selectable without the button.
- **FR-005**: The basic steps MUST tell the user to make a folder, save the copied text there as `docker-compose.yaml`, create the data folder, set the user to run as, start Beer Menu, then open the admin app and the TV display.
- **FR-006**: After the basic steps, the same page MUST have an environment variables section covering each of these, with purpose, default, and whether optional:

  | Variable | Purpose | Default |
  |----------|---------|---------|
  | `PUID`, `PGID` | User and group Beer Menu runs as; must own the data folder; never 0 | 65532 |
  | `BEER_MENU_DATA` | Host folder that holds the database and backups | `./data` |
  | `BEER_MENU_WEB_PORT` | Port the admin app and TV display open on | 8080 |
  | `BEER_MENU_VERSION` | Which release to run | `latest` |
  | `BEER_MENU_ADMIN_USERNAME`, `BEER_MENU_ADMIN_PASSWORD` | First admin, created at startup only while no admin exists; set both or neither; password 12 to 256 characters | unset |
  | `BEER_MENU_COOKIE_SECURE` | Set `false` only for plain-HTTP access by LAN address | `true` |
  | `BEER_MENU_SECRET_KEY_FILE` or `BEER_MENU_SECRET_KEY` | Encryption key for the Brewfather API key and authenticator-app secrets; without it those features are off; the file form wins and is safer | unset |
  | `BEER_MENU_PUBLIC_URL` | Address admins open the app at; needed for passkeys | unset |

- **FR-007**: The section MUST show how to set a variable (in the compose file's service settings, or in the shell or an env file next to the compose file) and say that Beer Menu must be restarted to apply changes.
- **FR-008**: Entries for secret values MUST warn against writing the value into the compose file and MUST show the safer way to supply it (shell or env file for the password; a key file for the encryption key, with how to create it and keep a copy outside the data folder).
- **FR-009**: The guide's Home page MUST include a link to https://github.com/helmnet/beer-menu, labeled as the project's GitHub repository.
- **FR-010**: Pages MUST keep the guide's existing look, navigation and phone-width layout; the compose block MUST NOT cause the page itself to scroll sideways.
- **FR-011**: The Getting started page and any other guide page that refers to the old clone or download steps or to "the Beer Menu folder" MUST be updated to match the new steps.

### Key Entities

- **Guide compose file**: The trimmed compose file shown on the Install page; a subset of the project's example compose file, image-only.
- **Environment variable entry**: Name, purpose, default, optional or needed, how to set it, and any secret-handling warning.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with Docker and no copy of the project can go from opening the Install page to seeing the admin app in under 5 minutes, with no git and no download of project files.
- **SC-002**: Copying the compose file takes one click, and the copied text starts Beer Menu with no edits.
- **SC-003**: 100% of the environment variables the app and its compose file read for user configuration appear in the section, each with purpose and default.
- **SC-004**: The Home page has exactly one way to reach the GitHub repository in one click, visible without scrolling on a desktop screen.
- **SC-005**: At phone width, no guide page scrolls sideways.

## Assumptions

- "Home section" means the guide's Home page (the first page under "Start" in the navigation), not a new navigation group.
- The environment variables section lives on the Install with Docker page, after the basic steps, not on a separate page.
- `BEER_MENU_DATA_DIR` and `BEER_MENU_PORT` are internal to the containers; changing them would break the data mount or the web proxy, so they are not documented for users.
- The guide's compose file keeps the defaults of the project's example compose file (`latest` image tag, port 8080, `./data`, user 65532 unless `PUID`/`PGID` are set), and the steps keep exporting `PUID`/`PGID` as the user's own IDs.
- The repository's own example compose file is not changed by this feature.
- The guide is still built and published the same way as today; this feature changes page content and adds a small copy behavior.
- The Copy button is the only scripted behavior added; the guide works without it.
