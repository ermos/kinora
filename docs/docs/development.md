---
sidebar_position: 6
title: Development
---

## Architecture

```
cmd/kinora            entry point, wiring, sites.json sync
internal/scraper      engine: hosters (hosters.go), JS unpacker, domain sync
  fr/                 French sources (vStream port)
  en/                 English sources (Scrubs V2 port) and their sites.json
internal/stream       HLS/file proxy, HMAC signing
internal/tmdb         TMDB client with an in-memory cache
internal/api          REST API /api/v1 (swaggo annotations to OpenAPI 3.1)
internal/store, db    PostgreSQL (pgx), embedded SQL migrations, SQLite import
ui/                   Expo (React Native): web and Android TV
  src/app/            expo-router routes
  src/components/     Focusable, Sidebar, Player.web.tsx (hls.js), Player.tsx (expo-video)
  src/api/            typed client generated from the spec (openapi-typescript)
docs/                 this website (Docusaurus)
```

## Run in development

```bash
docker compose up -d postgres
make dev       # Expo dev server with hot reload, on :8081
make run-dev   # in another terminal: the server, proxying the UI to Expo
```

Then open [http://localhost:8080](http://localhost:8080).

| Command | What it does |
|---|---|
| `make build-web` | exports the web app and builds `bin/kinora` with it embedded |
| `make test` | Go tests. Store tests need a PostgreSQL: `make test TEST_DATABASE_URL=postgres://...`, skipped without it |
| `make test-live` | checks every ported source and hoster against the real sites |
| `make lint` | golangci-lint and the TypeScript typecheck |
| `make openapi` | regenerates the spec from the Go annotations, then the TypeScript client |
| `make openapi-check` | fails if the committed spec is stale (run in CI) |

## Release

Push a `vX.Y.Z` tag: the `Release` workflow builds the `ghcr.io/ermos/kinora` image for `amd64` and `arm64` and tags it
`X.Y.Z`, `X.Y`, `X` and `latest`. Then build the Android TV APK with `APP_VERSION=X.Y.Z` (see
[Android TV](./android-tv.md#release-builds)) and attach `app-release.apk` to the GitHub release: installed apps offer
the update from there.

## The UI is built for TV

- **Focus first**: every interactive element goes through `Focusable`, which handles focus (remote, keyboard) and
  hover the same way. Enter and Space activate everything, and the sidebar opens on focus like on the Netflix TV app.
- **No web-only elements**: no `<select>` or `confirm()`, replaced by chips and two-press confirmations.
- **TV-ready modules only**: expo-router, expo-image, expo-video, react-native-svg, expo-linear-gradient,
  expo-secure-store.
- **One player per platform**: `Player.web.tsx` (hls.js, loaded on demand) and `Player.tsx` (expo-video, native HLS
  through ExoPlayer/AVPlayer), driven by the same `PlayerControls`.
- **No cookie on the proxy**: the stream URL is signed and expires (12 hours), it is enough on its own. Native
  players, AirPlay and Chromecast don't send the app's cookies.

## Add a language

1. `internal/scraper/<code>/`: a package that registers its `Language` (code, name, TMDB locale, track order) and its
   sources, imported in `cmd/kinora/main.go`.
2. `internal/api/i18n.go`: the titles of the home rows (a test checks none is missing).
3. `internal/api/auth.go`: the code in the `enums` tags, then `make openapi`.
4. `ui/src/i18n/<code>.ts`: the interface dictionary, declared in `ui/src/i18n/index.ts`. The TypeScript build fails
   while it is missing.
5. At least one source that declares this language: without it, the language is not offered at setup.

API errors return a stable `code` (`invalid_credentials`...) along with an English message. The interface translates
the code: each code must be in the `errors` section of every dictionary, or the TypeScript build fails (and a Go test
checks each code is declared).

## Port a source or a hoster

**Source**: a file in the package of its language, `internal/scraper/fr/<id>.go` or `internal/scraper/en/<id>.go`.
The id is the key of the site in the language's `sites.json`. There are no menus to port: only the `Find` function
matters. It receives the title (localized and original), the year, the TMDB ID and optionally the season and episode,
and returns links. vStream's Python code gives the URLs and the regexes. `FindAll` and `Find` apply the same cleanup
as vStream's `cParser`, so the regexes can be copied almost as is (Go uses RE2: no backreferences, no lookahead).

**Hoster**: an entry in `internal/scraper/hosters.go`, with the host names taken from `checkHoster` in vStream's
`resources/lib/gui/hoster.py`, and a `Resolve` function that returns the stream URL and the headers the CDN expects.
`Unpack` handles `eval(function(p,a,c,k,e,d)...)` scripts.

## This website

The site lives in `docs/` and uses [Docusaurus](https://docusaurus.io).

```bash
cd docs
npm ci
npm start   # http://localhost:3000
```

Pages are Markdown files in `docs/docs/`. The landing page is `docs/src/pages/index.tsx`.
