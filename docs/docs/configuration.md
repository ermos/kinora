---
sidebar_position: 3
title: Configuration
---

## Environment variables

kinora reads its settings from the environment. With Docker Compose, put them in `.env`.

| Variable | Default | Description |
|---|---|---|
| `TMDB_API_KEY` | required | TMDB v3 API key or v4 read access token. |
| `DATABASE_URL` | required | PostgreSQL connection string, `postgres://user:password@host:5432/kinora?sslmode=disable`. Docker Compose sets it to its own `postgres` service. |
| `ADDR` | `:8080` | Address the server listens on. |
| `APP_ENV` | `production` | Where the Android TV app looks for updates: `production` uses the latest GitHub release, `development` the APK built locally. See [Android TV](./android-tv.md#updates). |
| `UI_DEV_URL` | | Development only: proxies the UI to an Expo dev server. See [Development](./development.md). |

## Instance settings

Everything else is set by an admin from the **Account** page, and applies without a restart:

- **Language** of the instance (French or English).
- **FlareSolverr** server address.
- **Check for updates** of the Android TV app.

Each profile can turn off the skip intro and skip credits buttons.

## FlareSolverr

Some sites protect themselves with a Cloudflare challenge. kinora only queries them if an admin sets a
[FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) server. A challenged request goes through FlareSolverr
once, then kinora reuses the `cf_clearance` cookie it got directly for that domain, until the next challenge.

The `docker-compose.yml` already runs a `flaresolverr` service. To use it, go to **Account > FlareSolverr** and enter:

```
http://flaresolverr:8191
```

If you don't need these sites, remove the `flaresolverr` service from `docker-compose.yml`.

## Site domains

Streaming sites change domains often. kinora syncs them every 6 hours from a `sites.json` per language: vStream's for
French (updated by their team several times a week), and `internal/scraper/en/sites.json` from the kinora repository
for English. There is nothing to configure.
