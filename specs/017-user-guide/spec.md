# Feature Specification: Admin App User Guide

**Feature Branch**: `compmodder26/user_guide`

**Created**: 2026-11-14

**Status**: Draft

**Input**: User description: "I'd like to create a static HTML user guide for the beer-menu GUI that is hosted with github pages. It should have separately linked pages for each "page" of the admin app (taps, on deck, etc.). It should be geared toward end users. I'd like it to be a similar style to the dark mode of the beer menu admin gui itself. Also the guide should inclue screenshots for each page documented."

## Clarifications

### Session 2026-11-14

- Q: Which topics should the guide's Docker deployment page cover? → A: First start only: get the files, choose a data folder, start, then open the admin app and display.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Learn how to run the taps from the guide (Priority: P1)

A person who pours beer at home (not a developer) opens the guide's web address. The home page says what Beer Menu does and lists every page of the admin app, grouped the same way as the app's side panel (Menu, Administration, Integrations). They click "Taps" and land on a page that shows a screenshot of the Taps page and explains, in plain words, what they see and how to do each task there: put a keg on a tap, update how much is left, take a keg off. Every page of the guide has the same navigation, so they can move to "On Deck" or "Beer Library" without going back home.

**Why this priority**: The everyday work in the app is the Menu group (taps, on deck, keg history, beer library, display settings). A guide that covers these pages alone already helps most users.

**Independent Test**: Open the published guide in a browser, follow the link to each Menu-group page, and check that each one has a screenshot of the matching admin page and step-by-step instructions for its tasks.

**Acceptance Scenarios**:

1. **Given** the guide is published, **When** a user opens its home address, **Then** they see an introduction and a link to a separate page for each admin app page.
2. **Given** a user is on any guide page, **When** they look at the navigation, **Then** it lists every guide page, grouped as in the admin app's side panel, and marks the page they are on.
3. **Given** a user opens the Taps guide page, **When** they read it, **Then** it shows at least one screenshot of the admin app's Taps page and explains each task that page supports, in the order a user would do them.
4. **Given** a user reads any guide page, **When** they meet a button, field or label name, **Then** it is spelled exactly as it appears in the admin app.

---

### User Story 2 - Cover the rest of the admin app (Priority: P2)

A user who manages the setup (adds other admins, creates API tokens, changes their password, turns on two-factor sign-in, connects Brewfather) finds a guide page for each of those admin pages too, each with a screenshot and task steps. A brand-new user also finds an "Install with Docker" page that briefly shows how to start Beer Menu for the first time, and a "Getting started" page that covers first-run setup (creating the first admin), signing in, and how to open the TV display.

**Why this priority**: These pages are used less often, but users still need help with them, and some (two-factor, API tokens) are easy to get wrong.

**Independent Test**: Follow the link to each Administration and Integrations page and to Getting started; check each has a screenshot and task steps. Follow the Install with Docker page on a machine with Docker and check the app starts.

**Acceptance Scenarios**:

1. **Given** the guide is published, **When** a user opens the navigation, **Then** it has a page for each of: Taps, On Deck, Keg History, Beer Library, Display Settings, Admin Users, API Tokens, Change Password, Two-Factor Sign-In and Brewfather, plus Install with Docker and Getting started.
2. **Given** a user reads the Two-Factor Sign-In page, **When** they follow its steps, **Then** they can set up a second factor and they are told to save their recovery codes.
3. **Given** a user reads the API Tokens page, **When** they create a token, **Then** the guide warns them the token is shown only once.
4. **Given** a user has Docker installed, **When** they follow the Install with Docker page, **Then** they get the files, choose a data folder, start Beer Menu, and open the admin app and the TV display.

---

### User Story 3 - The guide looks like the app (Priority: P3)

A user switching between the admin app (in dark mode) and the guide feels they are in the same product: the same dark background, panel and text colors, accent color and general shape of panels and side navigation. The guide is easy to read on a phone as well as a desktop.

**Why this priority**: Matching style makes screenshots and text feel continuous, but the guide is useful without it.

**Independent Test**: Open the guide next to the admin app in dark mode and compare colors and layout; open the guide at phone width and check there is no sideways scrolling and the navigation is usable.

**Acceptance Scenarios**:

1. **Given** the guide and the admin app in dark mode are side by side, **When** a user compares them, **Then** the background, panel, side panel, text, muted text, border, accent and danger colors match.
2. **Given** a user opens the guide on a phone-width screen, **When** they read any page, **Then** text and screenshots fit the screen width without sideways scrolling, and the navigation is still reachable.

---

### User Story 4 - Keep the guide current (Priority: P3)

A maintainer changes an admin page. They update the matching guide page and its screenshot in the same pull request. When the change is merged to the main branch, the published guide updates without any manual publishing step.

**Why this priority**: A guide that drifts from the app misleads users; easy updates keep it honest.

**Independent Test**: Merge a small text change to a guide page and check the published guide shows it, with no manual step.

**Acceptance Scenarios**:

1. **Given** a change to the guide is merged to the main branch, **When** publishing finishes, **Then** the public guide shows the change.
2. **Given** a maintainer needs to retake screenshots, **When** they follow the documented steps, **Then** they can produce all screenshots from sample data, at a consistent size, in dark mode.

---

### Edge Cases

- A guide page is opened directly (bookmarked or shared link), not from the home page: it still shows the full navigation and works on its own.
- A screenshot fails to load: the page still makes sense, because every screenshot has a text description.
- A user opens the guide without JavaScript, or on a slow connection: all text, links and screenshots still work.
- A user's operating system is set to light mode: the guide stays in its dark style, matching the app's dark mode (the guide does not need a light theme).
- An admin page has states that look very different (for example an empty tap versus a tap with a keg, or Brewfather not connected versus connected): the guide shows the state the user most often needs, and describes the others in text.
- A screenshot would show personal or private data: screenshots use only invented sample data, so no real names, hostnames, tokens, codes or addresses appear.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The guide MUST be a set of static web pages, published to the repository's public project site, that need no server-side code to view.
- **FR-002**: The guide MUST have a home page that introduces Beer Menu for end users and links to every other guide page.
- **FR-003**: The guide MUST have one separate page for each admin app page: Taps, On Deck, Keg History, Beer Library, Display Settings, Admin Users, API Tokens, Change Password, Two-Factor Sign-In and Brewfather.
- **FR-004**: The guide MUST have a Getting started page that covers first-run setup (creating the first admin), signing in (including the second-factor step) and opening the TV display.
- **FR-004a**: The guide MUST have an Install with Docker page that covers first start only: getting the files, choosing a data folder, starting the app, and opening the admin app and the TV display. It MUST NOT cover upgrades, settings beyond the data folder, or restoring backups.
- **FR-005**: Every guide page MUST show the same navigation, grouped as in the admin app's side panel (Menu, Administration, Integrations), after a Start group holding the home page, Install with Docker and Getting started, with the current page marked.
- **FR-006**: Every admin-page guide page MUST include at least one screenshot of that admin page, taken in the admin app's dark mode.
- **FR-007**: Every screenshot MUST have a text description that says what it shows.
- **FR-008**: Screenshots MUST show only invented sample data. Real names, hostnames, IP addresses, and working tokens, recovery codes or two-factor QR codes MUST NOT appear.
- **FR-009**: Each admin-page guide page MUST explain what the page is for and give numbered steps for each task the page supports.
- **FR-010**: Guide text MUST be written for end users: plain words, no code, no API or developer terms. Exceptions: the API Tokens page may name what a token is used for, and the Install with Docker page may show the few commands needed to start the app.
- **FR-011**: Button, field and label names in the guide MUST match the admin app's wording exactly.
- **FR-012**: The guide MUST warn users where an action cannot be undone or a secret is shown only once (for example deleting items, revoking tokens, API token values and recovery codes).
- **FR-013**: The guide's colors MUST match the admin app's dark mode (background, panel, side panel, text, muted text, border, accent and danger colors), and its layout MUST use a side navigation and rounded panels like the app.
- **FR-014**: The guide MUST be readable at phone width (from 320 px) without sideways scrolling, with the navigation still reachable.
- **FR-015**: The guide MUST work fully with JavaScript turned off.
- **FR-016**: The guide MUST be published automatically when changes to it are merged to the main branch.
- **FR-017**: The guide MUST include maintainer instructions (outside the end-user pages) for updating pages and retaking screenshots with sample data.
- **FR-018**: The repository README MUST link to the published guide.
- **FR-019**: The guide MUST meet basic accessibility: headings in order, link text that names the target (never "click here"), text contrast of at least 4.5:1 (met by using the admin app's dark tokens), and screenshot descriptions per FR-007.

### Key Entities

- **Guide page**: one topic, with a title, a navigation group, an introduction, task sections with numbered steps, and one or more screenshots.
- **Screenshot**: an image of one admin page in dark mode, with sample data and a text description.
- **Navigation**: the shared list of guide pages, grouped like the admin app's side panel.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the 10 admin app pages have their own guide page, each with at least one screenshot.
- **SC-002**: A first-time user can find the instructions for any admin task within 2 clicks from any guide page.
- **SC-003**: A first-time user can put a keg on a tap using only the guide, on their first attempt.
- **SC-004**: Every guide page fits a 320 px wide screen with no sideways scrolling.
- **SC-005**: Every link in the guide leads to an existing page or image (0 broken links).
- **SC-006**: A merged guide change appears on the public site within 15 minutes, with no manual step.
- **SC-007**: 0 screenshots contain real personal data, secrets, or private network details.

## Assumptions

- The audience is the people who run the admin app at home: comfortable with a web browser, not developers. The Install with Docker page assumes Docker is already installed and does not teach Docker itself.
- The guide covers a brief first start with Docker, the admin app, and how to open the TV display. Upgrades, deployment settings, backup restore and API use are out of scope; they stay in the README and developer docs.
- "Each page of the admin app" means the 10 pages in the admin side panel. First-run setup and sign-in, which come before the side panel, are covered on one Getting started page.
- The guide uses dark style only. It does not offer a light theme or theme switch.
- The repository is public, so the guide can be published on its free public project site.
- Screenshots are taken from a local copy of the app filled with sample data, at one consistent desktop width. Phone-width screenshots are not required.
- The guide describes the current version of the app. Versioned guides for older releases are out of scope.
- English only.
