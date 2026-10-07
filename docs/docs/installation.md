---
sidebar_position: 2
title: Installation
---

kinora ships as a Docker image, `ghcr.io/ermos/kinora`, published for `amd64` and `arm64` (Raspberry Pi 4 and 5,
Apple Silicon, ARM NAS). It is a single Go binary with the web app built in, and it runs next to a PostgreSQL database.
Docker Compose starts both.

## Requirements

- [Docker](https://docs.docker.com/get-docker/) with the Compose plugin.
- A free TMDB API key: create an account on [themoviedb.org](https://www.themoviedb.org/signup), then copy the
  **API Key** (v3) or the **API Read Access Token** (v4) from [settings/api](https://www.themoviedb.org/settings/api).
  Both work.

## Install with Docker Compose

Download the compose file into a new folder and put your TMDB key in a `.env` file next to it:

```bash
mkdir kinora && cd kinora
curl -fsSLO https://kinora.stream/docker-compose.yml
echo "TMDB_API_KEY=your_tmdb_key" > .env
```

Then start kinora:

```bash
docker compose up -d
```

This pulls the image and starts the services ([see the file](pathname:///docker-compose.yml)):

| Service | Role |
|---|---|
| `kinora` | the server and the web app, on port `8080` |
| `postgres` | the database, stored in the `postgres` volume |
| `flaresolverr` | optional, lets kinora reach sites behind Cloudflare (see [Configuration](./configuration.md#flaresolverr)) |
| `backup` | optional, off by default: daily database dumps (see [Automatic backups](#automatic-backups)) |

### Image tags

| Tag | Description |
|---|---|
| `latest` | the latest release |
| `1`, `1.2`, `1.2.3` | pin a major, minor or exact version |

## First launch

Open [http://localhost:8080](http://localhost:8080) (or `http://<server-ip>:8080` from another device). The first
visit redirects to `/setup`, where you:

1. create the **admin account**,
2. choose the **language** of the instance (French or English). It sets the catalog language, the interface, which
   sources are queried and the order of audio tracks. An admin can change it later from the Account page.

Other accounts are created by the admin from the Account page. There is no public sign-up.

## Get the apps

The web app is built into the server: open kinora in any browser, on a computer, a tablet or a phone.

| Client | Where to get it |
|---|---|
| Web (computer, phone, tablet) | built in, at `http://<your-server>:8080` |
| Android TV and Google TV | `app-release.apk` in the [latest release](https://github.com/ermos/kinora/releases/latest), see [Android TV](./android-tv.md) |

## Update

```bash
docker compose pull
docker compose up -d
```

Database migrations run on startup.

## Backup and restore

All data (users, profiles, lists, progress, settings) lives in PostgreSQL.

### Automatic backups

The compose file has an optional `backup` service. Turn it on in `.env`:

```bash
echo "COMPOSE_PROFILES=backup" >> .env
docker compose up -d
```

It dumps the database when it starts, then every day at midnight, into a `backups` folder next to the compose file:

```
backups/
  last/      every dump of the last 24 hours, and kinora-latest.sql.gz
  daily/     7 days
  weekly/    4 weeks
  monthly/   6 months
```

Change the schedule and retention with `SCHEDULE` and `BACKUP_KEEP_*` in the compose file (see
[postgres-backup-local](https://github.com/prodrigestivill/docker-postgres-backup-local#environment-variables)). Copy
the `backups` folder to another disk or machine: a backup on the same disk won't survive that disk.

### Manual backup

```bash
docker compose exec postgres pg_dump -U kinora kinora | gzip > kinora.sql.gz
```

### Restore

Stop kinora, replace the database with the dump, then start it again:

```bash
docker compose stop kinora
docker compose exec postgres dropdb -U kinora kinora
docker compose exec postgres createdb -U kinora kinora
gunzip -c backups/last/kinora-latest.sql.gz | docker compose exec -T postgres psql -q -U kinora kinora
docker compose start kinora
```

## Migrate from SQLite

Instances from before PostgreSQL stored their data in `data/kinora.db`. Import it once, into an empty PostgreSQL
database:

```bash
docker compose run --rm -v ./data:/data kinora import-sqlite /data/kinora.db
```

Users, sessions, profiles, lists, progress and settings are copied.

## Build from source

The `docker-compose.yml` of the repository builds the image from the sources instead of pulling it:

```bash
git clone https://github.com/ermos/kinora
cd kinora
cp .env.example .env   # set TMDB_API_KEY
docker compose up -d --build
```

To run without Docker, you need Go (see `go.mod` for the version), Node.js 22 and any PostgreSQL.

```bash
docker compose up -d postgres   # or your own PostgreSQL, through DATABASE_URL
make build-web                  # exports the web app and builds bin/kinora with it embedded
TMDB_API_KEY=your_tmdb_key \
DATABASE_URL='postgres://kinora:kinora@localhost:5432/kinora?sslmode=disable' \
./bin/kinora
```

## Put it behind a reverse proxy

kinora listens on plain HTTP. To reach it from outside your network, put it behind a reverse proxy that handles TLS
(Caddy, Traefik, nginx...). For example with Caddy:

```
kinora.example.com {
  reverse_proxy localhost:8080
}
```

Next: [connect your Android TV](./android-tv.md).
